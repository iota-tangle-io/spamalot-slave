package api

import (
	"github.com/gorilla/websocket"
	"net/url"
	"gopkg.in/inconshreveable/log15.v2"
	"github.com/iota-tangle-io/spamalot-slave/backend/utilities"
	"github.com/iota-tangle-io/spamalot-coo/api"
	"github.com/iota-tangle-io/iota-spamalot.go"
	"encoding/json"
	"fmt"
	"github.com/CWarner818/giota"
	"github.com/pkg/errors"
	"crypto/md5"
	"encoding/hex"
	"os"
	"os/signal"
	"syscall"
)

func NewSlave(cooAddress string, apiToken string) *Slave {
	slave := &Slave{CooAddress: cooAddress, APIToken: apiToken}
	return slave
}

type Slave struct {
	CooAddress    string
	APIToken      string
	logger        log15.Logger
	ws            *websocket.Conn
	spammerConfig *api.SpammerConfig
	spammer       *spamalot.Spammer
	metrics       chan spamalot.Metric

	// use channels to enfore max one reader and writer throughout the slave
	wsWrite chan *api.SlaveMsg
	wsRead  chan *api.CooMsg
}

func (slave *Slave) Connect() {
	logger, err := utilities.GetLogger("slave")
	if err != nil {
		// TODO: replace in the future
		panic(err)
	}

	slave.logger = logger.New("address", slave.CooAddress)
	slave.metrics = make(chan spamalot.Metric)
	slave.wsWrite = make(chan *api.SlaveMsg)
	slave.wsRead = make(chan *api.CooMsg)

	u := url.URL{Scheme: "ws", Host: slave.CooAddress, Path: "/api"}
	slave.logger.Info("connecting to coordinator")

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		slave.logger.Warn("unable to connect to coordinator", "err", err.Error())
		return
	}
	defer ws.Close()
	slave.ws = ws

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		// racy
		slave.wsWrite <- &api.SlaveMsg{Type: api.SLAVE_BYE}
		slave.ws.Close()
	}()

	slave.logger.Info("greeting coordinator...")
	helloMsgPayload, err := json.Marshal(api.SlaveHelloMsg{APIToken: slave.APIToken})
	if err != nil {
		slave.logger.Warn("unable to marshal hello msg payload", "err", err.Error())
		slave.sendInternalErrorCode()
		return
	}

	helloMsg := api.SlaveMsg{Type: api.SLAVE_HELLO, Payload: helloMsgPayload}
	if err := slave.ws.WriteJSON(helloMsg); err != nil {
		slave.logger.Warn("unable to send hello msg", "err", err.Error())
		return
	}

	// waiting for coordinator approval
	cooMsg := &api.CooMsg{}
	if err := slave.ws.ReadJSON(cooMsg); err != nil {
		slave.logger.Warn("unable to parse coordinator's msg", "err", err.Error())
		return
	}

	switch cooMsg.Type {
	case api.SLAVE_API_TOKEN_INVALID:
		slave.logger.Warn("coordinator doesn't accept API token")
	case api.SLAVE_WELCOME:

		// grab spammer configuration
		spammerConfig := &api.SpammerConfig{}
		if err := json.Unmarshal(cooMsg.Payload, spammerConfig); err != nil {
			slave.logger.Info("unable to parse spammer configuration from coo, canceling conn,", "err", err.Error())
			slave.sendInternalErrorCode()
			return
		}
		slave.spammerConfig = spammerConfig

		slave.logger.Info("coordinator connection successful")
		slave.communicate()
	default:
		slave.logger.Warn("received undefined msg from coordinator")
	}

	// conn closed
	slave.logger.Info("closing connection to coordinator")
}

func (slave *Slave) sendInternalErrorCode() {
	slave.wsWrite <- &api.SlaveMsg{Type: api.SLAVE_INTERNAL_ERROR}
}

func (slave *Slave) printSpammerConfig() {
	if slave.spammerConfig == nil {
		return
	}

	prettyConfig, err := json.MarshalIndent(slave.spammerConfig, "", "   ")
	if err != nil {
		slave.logger.Warn("unable to print spammer config", "err", err.Error())
		return
	}
	fmt.Print(string(prettyConfig) + "\n")
}

func (slave *Slave) receiveMetrics() {
	for metric := range slave.metrics {
		// only send summaries for now
		if metric.Kind != spamalot.SUMMARY && metric.Kind != spamalot.INC_SUCCESSFUL_TX{
			continue
		}
		metricJSON, err := json.Marshal(&metric)
		if err != nil {
			slave.logger.Warn("unable to marshal metric")
			slave.sendInternalErrorCode()
			return
		}
		slaveMsg := &api.SlaveMsg{Type: api.SLAVE_METRIC, Payload: metricJSON}
		slave.wsWrite <- slaveMsg
	}
}

func (slave *Slave) openReceiveChannel() {
exit:
	for {
		cooMsg := &api.CooMsg{}
		if err := slave.ws.ReadJSON(cooMsg); err != nil {
			slave.logger.Warn("unable to read coo msg", "err", err.Error())
			break exit
		}
		slave.wsRead <- cooMsg
	}
	close(slave.wsRead)
}

func (slave *Slave) openSendChannel() {
exit:
	for {
		select {
		case msg := <-slave.wsWrite:
			if err := slave.ws.WriteJSON(msg); err != nil {
				slave.logger.Warn("unable to send slave msg", "err", err.Error())
				break exit
			}
			// TODO: add stop signal
		}
	}
}

func (slave *Slave) communicate() {

	// show the console what's going on
	slave.logger.Info("spammer configuration from coo:")
	slave.printSpammerConfig()

	// instantiate the spammer
	spammer, err := slave.newSpammer()
	if err != nil {
		slave.logger.Warn("unable to instantiate spammer", "err", err.Error())
		slave.sendInternalErrorCode()
		return
	}
	slave.spammer = spammer

	go slave.openReceiveChannel()
	go slave.openSendChannel()
	go slave.receiveMetrics()

	// send spammer state
	slave.sendSpammerState()

	for {
		cooMsg, ok := <-slave.wsRead
		if !ok {
			break
		}

		// obey to the coordinator
		switch cooMsg.Type {
		case api.SP_START:
			slave.startSpammer()
			slave.logger.Info("got spammer start msg")

		case api.SP_STOP:
			slave.logger.Info("got spammer stop msg")
			slave.stopSpammer()

		case api.SP_RESTART:
			slave.logger.Info("got spammer restart msg")
			slave.restartSpammer()

		case api.SP_RESET_CONFIG:
			wasRunning := slave.spammer.IsRunning()
			if err := slave.stopSpammer(); err != nil {
				continue
			}
			slave.logger.Info("got spammer reset config msg")
			if err := slave.configureSpammer(cooMsg.Payload); err != nil {
				slave.logger.Warn("couldn't reset configuration")
				continue
			}

			if wasRunning {
				if err := slave.startSpammer(); err != nil {
					continue
				}
			}
		case api.SP_METRICS:
			slave.logger.Info("got spammer metrics msg")

		default:
			slave.logger.Warn("got an unknown msg type from coo", "code", cooMsg.Type)
		}

		// after each action executed the slave returns its state to the coordinator
		slave.sendSpammerState()
	}

}

func (slave *Slave) sendSpammerState() {
	payload := api.SlaveSpammerStateMsg{}
	configBytes, err := json.Marshal(slave.spammerConfig)
	if err != nil {
		slave.logger.Warn("unable to marshal current config")
		slave.sendInternalErrorCode()
		return
	}

	// create state message
	hasher := md5.New()
	hasher.Write(configBytes)
	payload.ConfigHash = hex.EncodeToString(hasher.Sum(nil))
	payload.Running = slave.spammer.IsRunning()

	msg, err := api.NewSlaveMsg(api.SLAVE_SPAMMER_STATE, payload)
	if err != nil {
		slave.logger.Warn("unable to construct slave state msg", "err", err.Error())
		slave.sendInternalErrorCode()
		return
	}

	slave.logger.Info("sending state msg to coo...")
	slave.wsWrite <- msg
	slave.logger.Info("state msg sent to coo")
}

var ErrSpammerNotInitialised = errors.New("spammer is not initialised")

func (slave *Slave) printSlaveNotInitMsg() {
	slave.logger.Warn("spammer is not initialised")
}

func (slave *Slave) restartSpammer() error {
	slave.logger.Info("attempting to restart spammer...")
	if err := slave.stopSpammer(); err != nil {
		return err
	}
	if err := slave.startSpammer(); err != nil {
		return err
	}
	return nil
}

func (slave *Slave) stopSpammer() error {
	if slave.spammer == nil {
		slave.printSlaveNotInitMsg()
		return ErrSpammerNotInitialised
	}

	if !slave.spammer.IsRunning() {
		return nil
	}

	slave.logger.Info("halting spammer...")
	if err := slave.spammer.Stop(); err != nil {
		slave.logger.Warn("couldn't stop spammer", "err", err.Error())
		return err
	}
	slave.logger.Info("spammer stopped")
	return nil
}

func (slave *Slave) startSpammer() error {
	if slave.spammer == nil {
		slave.printSlaveNotInitMsg()
		return ErrSpammerNotInitialised
	}

	if slave.spammer.IsRunning() {
		return nil
	}

	slave.logger.Info("starting spammer...")
	go slave.spammer.Start() // don't actually do it for now
	slave.logger.Info("spammer started")
	return nil
}

func (slave *Slave) configureSpammer(payload []byte) error {
	spammerConfig := &api.SpammerConfig{}
	if err := json.Unmarshal(payload, spammerConfig); err != nil {
		slave.logger.Warn("unable to parse new spammer config", "err", err.Error())
		return err
	}

	slave.logger.Info("spammer configuration from coo:")

	// reset config so newSpammer() will create a spammer with the new config
	slave.spammerConfig = spammerConfig
	slave.printSpammerConfig()

	// previous spammer should be stopped
	spammer, err := slave.newSpammer()
	if err != nil {
		return err
	}
	slave.spammer = spammer
	return nil
}

func (slave *Slave) newSpammer() (*spamalot.Spammer, error) {
	config := slave.spammerConfig
	spammer, err := spamalot.New(
		spamalot.WithMWM(int64(config.MWM)),
		spamalot.WithDepth(int64(config.Depth)),
		spamalot.ToAddress(config.DestAddress),
		spamalot.WithTag(config.Tag),
		spamalot.WithMessage(config.Message),
		spamalot.WithSecurityLevel(spamalot.SecurityLevel(config.SecurityLvl)),
		spamalot.FilterTrunk(config.FilterTrunk),
		spamalot.FilterBranch(config.FilterTrunk),
		spamalot.FilterMilestone(config.FilterMilestone),
		spamalot.WithMetricsRelay(slave.metrics),
	)
	if err != nil {
		return nil, err
	}

	// configure PoW
	if config.PoWMode == api.POW_LOCAL {
		_, pow := giota.GetBestPoW()
		spammer.UpdateSettings(spamalot.WithPoW(pow))
		spammer.UpdateSettings(spamalot.WithNode(config.NodeAddress, false))
	} else {
		spammer.UpdateSettings(spamalot.WithNode(config.NodeAddress, true))
	}

	return spammer, nil
}
