package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/provider"
)

// Conversations serves the JSON conversation routes.
type Conversations struct {
	store conversation.Store
}

func NewConversations(store conversation.Store) *Conversations {
	return &Conversations{store: store}
}

type conversationItem struct {
	ID        string    `json:"id"`
	Channel   string    `json:"channel"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *Conversations) List(w http.ResponseWriter, r *http.Request) {
	items := []conversationItem{}
	if h.store != nil {
		list, err := h.store.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		channel := r.URL.Query().Get("channel")
		for _, c := range list {
			if channel != "" && c.Channel != channel {
				continue
			}
			items = append(items, conversationItem{
				ID:        c.ID,
				Channel:   c.Channel,
				Title:     c.Title,
				UpdatedAt: c.UpdatedAt,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": items})
}

type conversationDetail struct {
	ID        string             `json:"id"`
	Channel   string             `json:"channel"`
	Title     string             `json:"title"`
	UpdatedAt time.Time          `json:"updated_at"`
	Messages  []provider.Message `json:"messages"`
}

func (h *Conversations) Get(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "conversation not found"})
		return
	}
	c, err := h.store.Load(r.PathValue("id"))
	if errors.Is(err, conversation.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "conversation not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if c.Messages == nil {
		c.Messages = []provider.Message{}
	}
	writeJSON(w, http.StatusOK, conversationDetail{
		ID:        c.ID,
		Channel:   c.Channel,
		Title:     c.Title,
		UpdatedAt: c.UpdatedAt,
		Messages:  c.Messages,
	})
}
