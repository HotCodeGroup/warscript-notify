package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/HotCodeGroup/warscript-utils/models"

	vk "github.com/GDVFox/vkbot-go"
)

func SendMessageToPeer(message string, peer int64) error {
	_, err := notifyVKBot.MessagesSend(&vk.MessagesSendParams{
		PeerID:   peer,
		RandomID: rand.Int63(),
		Message:  message,
	})

	return err
}

// ProcessVKEvents обработка всех входящих через vk сообщений
func ProcessVKEvents(events vk.EventsChannel) {
	logger := logger.WithField("method", "ProcessVKEvents")

	for event := range events {
		if event.Type != "message_new" {
			continue
		}

		var message vk.MessagesMessage
		err := json.Unmarshal(event.Object, &message)
		if err != nil {
			logger.Errorf("New Message unmarshal error: %v", err)
			continue
		}

		userInfo, err := authGPRC.GetUserBySecret(context.Background(), &models.VkSecret{VkSecret: message.Text})
		if err != nil {
			logger.Warnf("can not get information about user by secret")
			err = SendMessageToPeer("Либо у нас что-то не работает, либо неправильный токен. "+
				"Сорян, нам очень жаль. ¯\\_(ツ)_/¯", message.PeerID)
			if err != nil {
				logger.Warnf("can not send sorry message")
			}
			continue
		}

		err = SendMessageToPeer(fmt.Sprintf("Ну всё, я тебя запомнил, %s!", userInfo.Username), message.PeerID)
		if err != nil {
			logger.Warnf("can not send sorry message")
		}
	}
}
