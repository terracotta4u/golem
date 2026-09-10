package server

import (
	"context"
	"errors"
	"html"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/store"
)

const webChannel = "web"

func (s *Server) mountWebChat(mux *http.ServeMux, runCtx context.Context) {
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /conversations/{id}", s.handleConversation)
	mux.HandleFunc("POST /conversations/{id}/turns", s.handleWebPostTurn(runCtx))
	mux.HandleFunc("GET /turns/{id}", s.handleWebTurn)
}

func (s *Server) webConversations() ([]store.Conversation, error) {
	if s.opts.Store == nil {
		return nil, nil
	}
	all, err := s.opts.Store.List()
	if err != nil {
		return nil, err
	}
	var list []store.Conversation
	for _, c := range all {
		if c.Channel == webChannel {
			list = append(list, c)
		}
	}
	return list, nil
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.showConversation(w, r, uuid.NewString())
}

func (s *Server) handleWebPostTurn(runCtx context.Context) http.HandlerFunc {
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
		if s.opts.Store != nil {
			c, err := s.opts.Store.Load(convID)
			switch {
			case errors.Is(err, store.ErrNotFound):
			case err != nil:
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			case c.Channel != webChannel:
				http.NotFound(w, r)
				return
			}
		}
		t := s.startTurn(runCtx, convID, postTurnRequest{Channel: webChannel, Text: text})
		s.render(w, "turn", map[string]any{"User": text, "ID": t.ID})
	}
}

func (s *Server) handleWebTurn(w http.ResponseWriter, r *http.Request) {
	s.serveTurnEvents(w, r, r.PathValue("id"), func() {
		http.NotFound(w, r)
	}, func(ev turnEvent) bool {
		switch ev.name {
		case "log":
			card, err := s.execute("tool-call", ev.log)
			if err != nil {
				return false
			}
			return writeSSE(w, "", `<hx-partial hx-target="find .tool-log" hx-swap="beforeend">`+card+`</hx-partial>`)
		case "done":
			if !writeSSE(w, "", `<hx-partial hx-target="find .reply">`+string(markdownHTML(ev.text))+`</hx-partial>`) {
				return false
			}
			return writeSSE(w, "close", "")
		case "error":
			if !writeSSE(w, "", `<hx-partial hx-target="find .reply"><p class="error">`+sseEscape(ev.err)+`</p></hx-partial>`) {
				return false
			}
			return writeSSE(w, "close", "")
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

func (s *Server) handleConversation(w http.ResponseWriter, r *http.Request) {
	s.showConversation(w, r, r.PathValue("id"))
}

func (s *Server) showConversation(w http.ResponseWriter, r *http.Request, id string) {
	conv := store.Conversation{ID: id, Channel: webChannel}
	if s.opts.Store != nil {
		c, err := s.opts.Store.Load(id)
		switch {
		case errors.Is(err, store.ErrNotFound):
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
	list, err := s.webConversations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "conversation", map[string]any{
		"Title":         conv.Title,
		"ID":            conv.ID,
		"Items":         chatItems(conv.Messages),
		"Conversations": list,
		"Sidebar":       true,
	})
}

type chatItem struct {
	Role    string
	Content string
	Tools   []toolLog
}

func chatItems(msgs []provider.Message) []chatItem {
	var items []chatItem
	var pending []toolLog
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
				var tools []toolLog
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

func collectTools(calls []provider.ToolCall, msgs []provider.Message, i int) ([]toolLog, int) {
	results := make(map[string]string, len(calls))
	for i < len(msgs) && msgs[i].Role == "tool" {
		results[msgs[i].ToolCallID] = msgs[i].Content
		i++
	}
	tools := make([]toolLog, 0, len(calls))
	for _, call := range calls {
		tools = append(tools, toolLog{
			Name:   call.Function.Name,
			Args:   call.Function.Arguments,
			Result: results[call.ID],
		})
	}
	return tools, i
}
