package main

import (
	"context"

	"github.com/HotCodeGroup/warscript-utils/models"
)

type NotifyManager struct{}

func (gm *NotifyManager) SendNotify(ctx context.Context, m *models.Message) (*models.Empty, error) {

	h.broadcast <- &HubMessage{
		Type:     m.Type,
		AuthorID: m.User,
		GameSlug: m.Game,
		Body:     m.Body,
	}

	return nil, nil
}
