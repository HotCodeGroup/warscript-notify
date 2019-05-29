package main

import (
	"net/http"

	"github.com/HotCodeGroup/warscript-notify/jmodels"
	"github.com/HotCodeGroup/warscript-utils/middlewares"
	"github.com/HotCodeGroup/warscript-utils/models"
	"github.com/HotCodeGroup/warscript-utils/utils"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// SessionInfo получает информацию о сессии из контекста запроса
func SessionInfo(r *http.Request) *models.SessionPayload {
	if rv := r.Context().Value(middlewares.SessionInfoKey); rv != nil {
		if rInfo, ok := rv.(*models.SessionPayload); ok {
			return rInfo
		}
	}

	return nil
}

// OpenWS создаёт ws клиент, который подключает к hub
func OpenWS(w http.ResponseWriter, r *http.Request) {
	logger := utils.GetLogger(r, logger, "OpenWS")
	errWriter := utils.NewErrorResponseWriter(w, logger)

	info := SessionInfo(r)
	if info == nil {
		errWriter.WriteWarn(http.StatusUnauthorized, errors.New("session info is not presented"))
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // мы уже прошли слой CORS
		},
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		errWriter.WriteError(http.StatusInternalServerError, errors.Wrap(err, "upgrade to websocket error"))
		return
	}

	sessionID := uuid.New().String()
	verifyClient := &HubClient{
		SessionID: sessionID,
		UserID:    info.ID,

		h:       h,
		conn:    c,
		send:    make(chan *jmodels.HubMessage),
		filters: make([]func(*jmodels.HubMessage) bool, 0, 0),
	}
	verifyClient.h.register <- verifyClient

	go verifyClient.WriteStatusUpdates()
	go verifyClient.WaitForClose()
}
