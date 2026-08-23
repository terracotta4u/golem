package store

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/terracotta4u/golem/provider"
)

var ErrNotFound = errors.New("conversation not found")

type Conversation struct {
	ID        string             `json:"id"`
	Channel   string             `json:"channel"`
	Title     string             `json:"title,omitempty"`
	UpdatedAt time.Time          `json:"updated_at"`
	Messages  []provider.Message `json:"messages"`
}

type Store interface {
	Load(id string) (Conversation, error)
	Save(Conversation) error
	List() ([]Conversation, error)
}

func New(channel string) Conversation {
	return Conversation{
		ID:      newID(),
		Channel: channel,
	}
}

// Open loads a conversation by id, or returns an unsaved one with that id and
// channel. Callers that already have a stable identity (a Slack thread, a
// Telegram chat) should use this instead of New.
func Open(st Store, id, channel string) (Conversation, error) {
	if id == "" {
		return Conversation{}, fmt.Errorf("conversation id is required")
	}
	c, err := st.Load(id)
	if errors.Is(err, ErrNotFound) {
		return Conversation{ID: id, Channel: channel}, nil
	}
	return c, err
}

func (c *Conversation) SetTitleFrom(text string) {
	if c.Title != "" {
		return
	}
	c.Title = truncate(oneLine(text), 80)
}

func Last(list []Conversation, channel string) (Conversation, error) {
	var best Conversation
	found := false
	for _, c := range list {
		if channel != "" && c.Channel != channel {
			continue
		}
		if !found || c.UpdatedAt.After(best.UpdatedAt) {
			best = c
			found = true
		}
	}
	if !found {
		return Conversation{}, ErrNotFound
	}
	return best, nil
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
