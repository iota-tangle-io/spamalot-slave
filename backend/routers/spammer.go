package routers

import (
	"github.com/labstack/echo"
	"github.com/iota-tangle-io/spamalot-slave/backend/controllers"
	"github.com/gorilla/websocket"
	"github.com/iota-tangle-io/spamalot-slave/backend/utilities"
	"time"
)

var (
	upgrader = websocket.Upgrader{}
)

type SpammerRouter struct {
	WebEngine *echo.Echo               `inject:""`
	Ctrl      *controllers.SpammerCtrl `inject:""`
}

type MsgType byte

const (
	SERVER_READ_ERROR MsgType = 0

	START  MsgType = 1
	STOP   MsgType = 2
	METRIC MsgType = 3
	STATE  MsgType = 4
)

type wsmsg struct {
	MsgType MsgType     `json:"msg_type"`
	Data    interface{} `json:"data"`
	TS      time.Time   `json:"ts"`
}

func newWSMsg() *wsmsg {
	return &wsmsg{TS: time.Now()}
}

func (router *SpammerRouter) Init() {

	logger, err := utilities.GetLogger("spammer-router")
	if err != nil {
		panic(err)
	}
	group := router.WebEngine.Group("/api/spammer")

	group.GET("", func(c echo.Context) error {

		ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}
		defer ws.Close()

		writer := make(chan interface{})
		metrics := make(chan interface{})
		stop := make(chan struct{})
		defer close(stop) // auto-free writer, poller

		// subscribe to metrics
		listenerID := router.Ctrl.AddMetricListener(metrics)
		defer router.Ctrl.RemoveMetricListener(listenerID)

		// sync WS writer
		go func() {
			for {
				select {
				case msgToWrite := <-writer:
					if err := ws.WriteJSON(msgToWrite); err != nil {
						logger.Error("unable to send message", "err", err.Error())
						return
					}
				case <-stop:
					return
				}
			}
		}()

		// metrics poller
		go func() {
			for {
				select {
				case metric, ok := <-metrics:
					if !ok {
						// timeout was reached in controller for metric send
						return
					}
					writer <- wsmsg{MsgType: METRIC, Data: metric, TS: time.Now()}
				case <-stop:
					return
				}
			}
		}()

		for {
			msg := &wsmsg{}
			if err := ws.ReadJSON(msg); err != nil {
				logger.Error("unable to read ws message", "err", err.Error())
				break
			}

			switch msg.MsgType {
			case START:
				router.Ctrl.Start()
			case STOP:
				router.Ctrl.Stop()
			}

			// auto send state after each received command
			writer <- wsmsg{MsgType: STATE, Data: router.Ctrl.State(), TS: time.Now()}
		}

		return nil
	})
}
