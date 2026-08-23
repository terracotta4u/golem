package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/store"
)

type turn struct {
	ID     string   `json:"id"`
	Status string   `json:"status"`
	Text   string   `json:"text,omitempty"`
	Error  string   `json:"error,omitempty"`
	Log    []string `json:"log,omitempty"`
}

type postTurnRequest struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

func (s *Server) mountChat(mux *http.ServeMux, runCtx context.Context) {
	mux.HandleFunc("POST /v1/conversations/{id}/turns", s.handlePostTurn(runCtx))
	mux.HandleFunc("GET /v1/turns/{id}", s.handleGetTurn)
}

func (s *Server) handlePostTurn(runCtx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		var req postTurnRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		convID := r.PathValue("id")
		if convID == "" || req.Channel == "" || req.Text == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel and text are required"})
			return
		}

		t := &turn{ID: uuid.NewString(), Status: "pending"}
		s.mu.Lock()
		s.turns[t.ID] = t
		s.mu.Unlock()

		go s.run(runCtx, t.ID, convID, req)
		writeJSON(w, http.StatusAccepted, map[string]string{"id": t.ID})
	}
}

func (s *Server) handleGetTurn(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id := r.PathValue("id")
	s.mu.Lock()
	t, ok := s.turns[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "turn not found"})
		return
	}
	out := *t
	if t.Log != nil {
		out.Log = append([]string(nil), t.Log...)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) run(ctx context.Context, turnID, convID string, req postTurnRequest) {
	l := s.lockFor(convID)
	l.Lock()
	defer l.Unlock()

	conv, err := store.Open(s.opts.Store, convID, req.Channel)
	if err != nil {
		s.finish(turnID, "", err)
		return
	}

	sess := s.opts.Agent.Session(s.opts.Store, conv)
	sess.OnTool = func(name, args string) {
		line := fmt.Sprintf("[%s] %s", name, args)
		fmt.Fprintln(os.Stderr, line)
		s.appendLog(turnID, line)
	}
	text, err := sess.Send(ctx, req.Text)
	s.finish(turnID, text, err)
}

func (s *Server) appendLog(id, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.turns[id]
	if !ok {
		return
	}
	t.Log = append(t.Log, line)
}

func (s *Server) finish(id, text string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.turns[id]
	if !ok {
		return
	}
	if err != nil {
		t.Status = "error"
		t.Error = err.Error()
		return
	}
	t.Status = "done"
	t.Text = text
}

func (s *Server) lockFor(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[id]
	if !ok {
		l = &sync.Mutex{}
		s.locks[id] = l
	}
	return l
}
