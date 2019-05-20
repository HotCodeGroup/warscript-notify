package main

import (
	"encoding/json"

	vk "github.com/GDVFox/vkbot-go"
)

// ProcessVKEvents обработка всех входящих через vk сообщений
func ProcessVKEvents(events vk.EventsChannel) {
	logger := logger.WithField("method", "ProcessVKEvents")

	for event := range events {
		if event.Type == "message_new" {
			var message vk.MessagesMessage
			err := json.Unmarshal(event.Object, &message)
			if err != nil {
				logger.Errorf("New Message unmarshal error: %v", err)
				continue
			}

			logger.Infof("[%s] FROM %d; PEER %d; MSG_ID %d: %s", event.Type, message.FromID,
				message.PeerID, message.ConversationMessageID, message.Text)
		}
	}
}
