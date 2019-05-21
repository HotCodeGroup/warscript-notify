package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"

	"github.com/HotCodeGroup/warscript-utils/models"
	"github.com/go-redis/redis"
	"github.com/pkg/errors"

	vk "github.com/GDVFox/vkbot-go"
)

// UserNotifyInfo информация, которая хранится в Redis
type UserNotifyInfo struct {
	Peers map[int64]struct{} `json:"peers"`
}

// AddPeer присоединяет пир во множество слушателей
func (u *UserNotifyInfo) AddPeer(peer int64) {
	u.Peers[peer] = struct{}{}
}

// RemovePeer отсоединяет пир
func (u *UserNotifyInfo) RemovePeer(peer int64) {
	delete(u.Peers, peer)
}

// GetPeerByUser получаем список пиров, которые хотят инфу по юзеру
func GetPeerByUser(userID int64) (*UserNotifyInfo, error) {
	redisKey := strconv.FormatInt(userID, 10)
	data, err := rediCli.Get(redisKey).Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "can not get value by key")
	}

	updatedInfo := &UserNotifyInfo{}
	if err = json.Unmarshal(data, updatedInfo); err != nil {
		return nil, errors.Wrap(err, "can not unmarshal data by user id")
	}

	return updatedInfo, nil
}

// ConnectPeerToUser кладём в базу связь между userID и peerID
func ConnectPeerToUser(userID, peerID int64) error {
	redisKey := strconv.FormatInt(userID, 10)

	var updatedInfo *UserNotifyInfo
	data, err := rediCli.Get(redisKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			updatedInfo = &UserNotifyInfo{
				Peers: map[int64]struct{}{
					peerID: {},
				},
			}
		} else {
			return errors.Wrap(err, "can not get data by user id")
		}
	} else {
		if data != nil {
			updatedInfo = &UserNotifyInfo{}
			if err = json.Unmarshal(data, updatedInfo); err != nil {
				return errors.Wrap(err, "can not unmarshal data by user id")
			}
			updatedInfo.AddPeer(peerID)
		}
	}

	newData, err := json.Marshal(updatedInfo)
	if err != nil {
		return errors.Wrap(err, "can not marshal new info")
	}

	if err = rediCli.Set(redisKey, newData, 0).Err(); err != nil {
		return errors.Wrap(err, "can not set new data")
	}

	return nil
}

func SendMessageToPeer(message string, peer int64) error {
	_, err := notifyVKBot.MessagesSend(&vk.MessagesSendParams{
		PeerID:   peer,
		RandomID: rand.Int63(),
		Message:  message,
	})

	return err
}

// SendMessageToUser рассылает сообщение всем, кто подписывался на userID
func SendMessageToUser(message string, userID int64) {
	logger := logger.WithField("method", "ProcessVKEvents")

	peers, err := GetPeerByUser(userID)
	if err != nil {
		if errors.Cause(err) != redis.Nil {
			logger.Warnf("can not send message to %d: %v", userID, err)
		}
		return
	}

	wg := &sync.WaitGroup{}
	for peer := range peers.Peers {
		wg.Add(1)
		go func(m string, p int64) {
			defer wg.Done()
			if err := SendMessageToPeer(m, p); err != nil {
				logger.Warnf("can not send to %d: %v", p, err)
			}
		}(message, peer)
	}
	wg.Wait()
}

func ProcessMessageForVK(message *HubMessage) {
	switch message.Type {
	case "match":
		msgBody := &NotifyMatchMessage{}
		if err := json.Unmarshal(message.Body, msgBody); err != nil {
			logger.Warnf("can not unmarshal notify body to %d: %v", message.AuthorID, err)
		}

		verdict := "В другой раз обязательно получится!"
		if msgBody.Diff >= 0 {
			verdict = "Отличная работа!"
		}

		SendMessageToUser(fmt.Sprintf("%s\nВ новом сражении твой бот #%d набрал %d очков.\n"+
			"Настало время посмотреть реплей: http://89.208.198.192/pong/matches/%d",
			verdict, msgBody.BotID, msgBody.Diff, msgBody.MatchID), message.AuthorID)
	case "verify":
		msgBody := &NotifyVerifyMessage{}
		if err := json.Unmarshal(message.Body, msgBody); err != nil {
			logger.Warnf("can not unmarshal notify body to %d: %v", message.AuthorID, err)
		}

		not := " не "
		verdict := "Стоить поработать над решением!"
		if msgBody.Veryfied {
			not = " "
			verdict = "Отличное начало, дальше только интереснее!"
		}

		SendMessageToUser(fmt.Sprintf("%s\nТвой бот #%d%sпрошел тестрирование.\n"+
			"Настало время посмотреть реплей: http://89.208.198.192/pong/matches/%d",
			verdict, msgBody.BotID, not, msgBody.MatchID), message.AuthorID)
	}
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
			logger.Warnf("can not get information about user by secret: %v", err)
			err = SendMessageToPeer("Либо у нас что-то не работает, либо неправильный токен.\n"+
				"В любом случае, нам очень жаль. ¯\\_(ツ)_/¯", message.PeerID)
			if err != nil {
				logger.Warnf("can not send get user sorry message: %v", err)
			}
			continue
		}

		if err = ConnectPeerToUser(userInfo.ID, message.PeerID); err != nil {
			logger.Warnf("can not update userID peer information: %v", err)
			err = SendMessageToPeer(fmt.Sprintf("Я тебя узнал, %s, но запомнить не вышло. :(\n"+
				"Давай в другой раз.", userInfo.Username), message.PeerID)
			if err != nil {
				logger.Warnf("can not send update sorry message: %v", err)
			}
			continue
		}

		err = SendMessageToPeer(fmt.Sprintf("Ну всё, я тебя запомнил, %s!", userInfo.Username), message.PeerID)
		if err != nil {
			logger.Warnf("can not send sorry message")
		}
	}
}
