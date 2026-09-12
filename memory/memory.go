package memory

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("memory not found")

type Memory struct {
	ID             string
	Content        string
	ConversationID string
	TurnID         string
	CreatedAt      time.Time
}
