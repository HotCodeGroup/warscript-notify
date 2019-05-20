package main

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// HubClient представляет ws клиента, подключенного к хабу
type HubClient struct {
	SessionID string
	UserID    int64

	h       *hub
	conn    *websocket.Conn
	send    chan *HubMessage
	filters []func(*HubMessage) bool
}

// WaitForClose отключает клиента от хаба, при закрытии ws
func (bv *HubClient) WaitForClose() {
	logger := log.WithFields(log.Fields{
		"ws_session": bv.SessionID,
		"method":     "WaitForClose",
	})

	defer func() {
		bv.h.unregister <- bv
		bv.conn.Close()
	}()
	bv.conn.SetPongHandler(func(string) error { return bv.conn.SetReadDeadline(time.Now().Add(pongWait)) })
	for {
		_, _, err := bv.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error(errors.Wrap(err, "unexpected close websocket error"))
			}
			break
		}
	}
}

// WriteStatusUpdates отправляет клиенту нотификации по ws
func (bv *HubClient) WriteStatusUpdates() {
	logger := log.WithFields(log.Fields{
		"ws_session": bv.SessionID,
		"method":     "WriteStatusUpdates",
	})

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		bv.conn.Close()
	}()

	for {
		select {
		case message, ok := <-bv.send:
			if !ok {
				// The hub closed the channel.
				err := bv.conn.WriteMessage(websocket.CloseMessage, nil)
				if err != nil {
					logger.Error(errors.Wrap(err, "websocket write close message error"))
				}
				return
			}
			send := true
			for _, f := range bv.filters {
				if !f(message) {
					send = false
					break
				}
			}
			if send {
				err := bv.conn.WriteJSON(message)
				if err != nil {
					logger.Error(errors.Wrap(err, "websocket write status update message error"))
				}
			}
		case <-ticker.C:
			if err := bv.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error(errors.Wrap(err, "websocket write ping message error"))
				return
			}
		}
	}
}
