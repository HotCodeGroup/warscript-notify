package jmodels

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
