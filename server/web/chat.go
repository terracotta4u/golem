package web

import (
	"context"
	"errors"
	"html"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/server/api"
)

const webChannel = "web"

// Chat serves the conversation pages.
type Chat struct {
	pages *Pages
	store conversation.Store
	turns *api.Chat
}

func NewChat(pages *Pages, store conversation.Store, turns *api.Chat) *Chat {
	return &Chat{pages: pages, store: store, turns: turns}
}

func (h *Chat) conversations() ([]conversation.Conversation, error) {
	if h.store == nil {
		return nil, nil
	}
	all, err := h.store.List()
	if err != nil {
		return nil, err
	}
	var list []conversation.Conversation
	for _, c := range all {
		if c.Channel == webChannel {
			list = append(list, c)
		}
	}
	return list, nil
}

func (h *Chat) Home(w http.ResponseWriter, r *http.Request) {
	h.show(w, r, uuid.NewString())
}

func (h *Chat) Post(runCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		convID := r.PathValue("id")
		text := strings.TrimSpace(r.FormValue("message"))
		if convID == "" || text == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}
		if h.store != nil {
			c, err := h.store.Load(convID)
			switch {
			case errors.Is(err, conversation.ErrNotFound):
			case err != nil:
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			case c.Channel != webChannel:
				http.NotFound(w, r)
				return
			}
		}
		id := h.turns.Start(runCtx, convID, webChannel, text)
		h.pages.Render(w, "turn", map[string]any{"User": text, "ID": id})
	}
}

func (h *Chat) Events(w http.ResponseWriter, r *http.Request) {
	h.turns.Serve(w, r, r.PathValue("id"), func() {
		http.NotFound(w, r)
	}, func(ev api.Event) bool {
		switch ev.Name {
		case "log":
			card, err := h.pages.Execute("tool-call", ev.Log)
			if err != nil {
				return false
			}
			return api.WriteSSE(w, "", `<hx-partial hx-target="find .tool-log" hx-swap="beforeend">`+card+`</hx-partial>`)
		case "done":
			if !api.WriteSSE(w, "", `<hx-partial hx-target="find .reply">`+string(Markdown(ev.Text))+`</hx-partial>`) {
				return false
			}
			return api.WriteSSE(w, "close", "")
		case "error":
			if !api.WriteSSE(w, "", `<hx-partial hx-target="find .reply"><p class="error">`+sseEscape(ev.Err)+`</p></hx-partial>`) {
				return false
			}
			return api.WriteSSE(w, "close", "")
		default:
			return false
		}
	})
}

func sseEscape(s string) string {
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", " ")
}

func (h *Chat) Conversation(w http.ResponseWriter, r *http.Request) {
	h.show(w, r, r.PathValue("id"))
}

func (h *Chat) show(w http.ResponseWriter, r *http.Request, id string) {
	conv := conversation.Conversation{ID: id, Channel: webChannel}
	if h.store != nil {
		c, err := h.store.Load(id)
		switch {
		case errors.Is(err, conversation.ErrNotFound):
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		case c.Channel != webChannel:
			http.NotFound(w, r)
			return
		default:
			conv = c
		}
	}
	list, err := h.conversations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.pages.Render(w, "conversation", map[string]any{
		"Title":         conv.Title,
		"ID":            conv.ID,
		"Items":         chatItems(conv.Messages),
		"Conversations": list,
		"Sidebar":       true,
		"PageCSS":       "chat.css",
	})
}

type chatItem struct {
	Role    string
	Content string
	Tools   []api.ToolLog
}

func chatItems(msgs []provider.Message) []chatItem {
	var items []chatItem
	var pending []api.ToolLog
	for i := 0; i < len(msgs); {
		m := msgs[i]
		switch m.Role {
		case "user":
			if m.Content != "" {
				items = append(items, chatItem{Role: "user", Content: m.Content})
			}
			i++
		case "assistant":
			i++
			if len(m.ToolCalls) > 0 {
				var tools []api.ToolLog
				tools, i = collectTools(m.ToolCalls, msgs, i)
				pending = append(pending, tools...)
			}
			if m.Content == "" {
				continue
			}
			items = append(items, chatItem{Role: "assistant", Content: m.Content, Tools: pending})
			pending = nil
		default:
			i++
		}
	}
	if len(pending) > 0 {
		items = append(items, chatItem{Role: "assistant", Tools: pending})
	}
	return items
}

func collectTools(calls []provider.ToolCall, msgs []provider.Message, i int) ([]api.ToolLog, int) {
	results := make(map[string]string, len(calls))
	for i < len(msgs) && msgs[i].Role == "tool" {
		results[msgs[i].ToolCallID] = msgs[i].Content
		i++
	}
	tools := make([]api.ToolLog, 0, len(calls))
	for _, call := range calls {
		tools = append(tools, api.ToolLog{
			Name:   call.Function.Name,
			Args:   call.Function.Arguments,
			Result: results[call.ID],
		})
	}
	return tools, i
}
