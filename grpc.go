package main

import (
	"context"

	"github.com/HotCodeGroup/warscript-notify/jmodels"

	"github.com/HotCodeGroup/warscript-utils/models"
)

// NotifyManager реализации интерфеса gRPC сервера.
type NotifyManager struct{}

// SendNotify отправлка уведомления пользователю, полученного из другого сервера
func (gm *NotifyManager) SendNotify(ctx context.Context, m *models.Message) (*models.Empty, error) {

	h.broadcast <- &jmodels.HubMessage{
		Type:     m.Type,
		AuthorID: m.User,
		GameSlug: m.Game,
		Body:     m.Body,
	}

	return &models.Empty{}, nil
}
