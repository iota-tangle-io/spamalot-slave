package api

import (
	"github.com/gorilla/websocket"
	"net/url"
	"gopkg.in/inconshreveable/log15.v2"
	"github.com/iota-tangle-io/spamalot-slave/backend/utilities"
	"github.com/iota-tangle-io/spamalot-coo/api"
)

func NewSlave(cooAddress string, apiToken string) *Slave {
	slave := &Slave{CooAddress: cooAddress, APIToken: apiToken}
	return slave
}

type Slave struct {
	CooAddress string
	APIToken   string
	logger log15.Logger
}


func (slave *Slave) Connect() {
	logger, err := utilities.GetLogger("slave")
	if err != nil {
		// TODO: replace in the future
		panic(err)
	}

	slave.logger = logger.New("address", slave.CooAddress)

	u := url.URL{Scheme: "ws", Host: slave.CooAddress, Path: "/api"}
	slave.logger.Info("connecting to coordinator")

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		slave.logger.Warn("unable to connect to coordinator", "err", err.Error())
		return
	}
	defer ws.Close()

	slave.logger.Info("greeting coordinator...")
	if err := ws.WriteJSON(&api.SlaveHelloMsg{APIToken: slave.APIToken}); err != nil {
		slave.logger.Warn("unable to send hello msg", "err", err.Error())
		return
	}

	// waiting for coordinator approval
	cooMsg := &api.CooMsg{}
	if err := ws.ReadJSON(cooMsg); err != nil {
		slave.logger.Warn("unable to parse coordinator's message", "err", err.Error())
		return
	}

	switch cooMsg.Type {
	case api.SLAVE_API_TOKEN_INVALID:
		slave.logger.Warn("coordinator doesn't accept API token, closing connection")
	case api.SLAVE_WELCOME:
		slave.logger.Info("coordinator connection successful")
		slave.communicate(ws)
	default:
		slave.logger.Warn("received undefined message from coordinator, closing connection")
	}

	// conn closed
}

func (slave *Slave) communicate(ws *websocket.Conn) {

}