package main

import (
	"encoding/json"
)

// HubMessage структура нотификации
type HubMessage struct {
	AuthorID int64           `json:"-"`
	GameSlug string          `json:"-"`
	Type     string          `json:"type"`
	Body     json.RawMessage `json:"body"`
}

// NotifyMatchMessage сообщение для сервиса нотификации о матче
type NotifyMatchMessage struct {
	BotID    int64  `json:"bot_id"`
	GameSlug string `json:"game_slug"`
	MatchID  int64  `json:"match_id"`
	Diff     int64  `json:"diff"`
}

// NotifyVerifyMessage сообщение для сервиса нотификации о прохождении проверки
type NotifyVerifyMessage struct {
	BotID    int64  `json:"bot_id"`
	GameSlug string `json:"game_slug"`
	MatchID  int64  `json:"match_id"`
	Veryfied bool   `json:"veryfied"`
}
