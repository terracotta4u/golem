package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/memory"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/tool"
)

type Agent struct {
	// MaxToolRounds caps Chat/tool loops per Send. Zero means no cap.
	MaxToolRounds int
	// Fast is an optional cheaper provider for lightweight work. Nil means use the default.
	Fast provider.Provider
	// Memories is retrieval and extraction for each Send. The zero value skips both.
	Memories Memories
	// Catalog, when set, adds tools that are live for the current round.
	// A builtin with the same name is kept.
	Catalog Catalog

	provider  provider.Provider
	list      []tool.Tool
	workspace string
	wg        sync.WaitGroup
}

// Memories is how an agent reads and records memories.
// Search nil skips retrieval. Store nil skips extraction.
type Memories struct {
	Search        memory.Searcher
	Store         *memory.Store
	Index         memory.Indexer
	MinSimilarity float32
	BudgetTokens  int
}

// Catalog is the set of extension tools available right now.
type Catalog interface {
	Tools() []tool.Tool
}

func New(p provider.Provider, dir string, tools ...tool.Tool) *Agent {
	return &Agent{provider: p, list: tools, workspace: dir}
}

type Session struct {
	agent    *Agent
	store    conversation.Store
	conv     conversation.Conversation
	memories []memory.Memory
	OnTool   func(name, args, result string)
}

func (a *Agent) Session(st conversation.Store, conv conversation.Conversation) *Session {
	return &Session{agent: a, store: st, conv: conv}
}

func (s *Session) ID() string { return s.conv.ID }

func (s *Session) Send(ctx context.Context, input string) (string, error) {
	untitled := s.conv.Title == ""
	s.conv.SetTitleFrom(input)
	s.conv.Messages = append(s.conv.Messages, provider.Message{Role: "user", Content: input})
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.retrieve(ctx, input)

	for round := 0; ; round++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if max := s.agent.MaxToolRounds; max > 0 && round >= max {
			return "", fmt.Errorf("exceeded %d tool rounds", max)
		}

		byName, defs := s.agent.roundTools()
		msg, err := s.agent.provider.Chat(ctx, provider.ChatRequest{
			Messages: withContext(systemPrompt(s.memories, s.agent.list...), s.conv.Messages),
			Tools:    defs,
		})
		if err != nil {
			if n := len(s.conv.Messages); n > 0 && s.conv.Messages[n-1].Role == "user" {
				s.conv.Messages = s.conv.Messages[:n-1]
			}
			return "", err
		}

		s.conv.Messages = append(s.conv.Messages, msg)
		if len(msg.ToolCalls) == 0 {
			if untitled {
				s.nameChat(ctx, input)
			}
			if err := s.persist(); err != nil {
				return msg.Content, err
			}
			s.agent.goRemember(ctx, rememberJob{
				provider: s.agent.provider,
				store:    s.agent.Memories.Store,
				indexer:  s.agent.Memories.Index,
				convID:   s.conv.ID,
				input:    input,
				reply:    msg.Content,
			})
			return msg.Content, nil
		}

		for _, call := range msg.ToolCalls {
			result := s.agent.runTool(ctx, byName, call)
			if s.OnTool != nil {
				s.OnTool(call.Function.Name, call.Function.Arguments, result)
			}
			s.conv.Messages = append(s.conv.Messages, provider.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}
}

func withContext(prompt string, msgs []provider.Message) []provider.Message {
	out := make([]provider.Message, 0, 1+len(msgs))
	out = append(out, provider.Message{Role: "system", Content: prompt})
	return append(out, msgs...)
}

const memorySearchLimit = 20

func (s *Session) retrieve(ctx context.Context, query string) {
	if s.agent.Memories.Search == nil {
		return
	}
	hits, err := s.agent.Memories.Search.Search(ctx, query, memorySearchLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: search: %v\n", err)
		return
	}
	s.memories = memory.Retrieve(hits, s.agent.Memories.MinSimilarity, s.agent.Memories.BudgetTokens, 0, memory.ApproxTokenEstimator{})
}

func (a *Agent) goRemember(ctx context.Context, job rememberJob) {
	if job.store == nil {
		return
	}
	ctx = context.WithoutCancel(ctx)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		remember(ctx, job)
	}()
}

func (a *Agent) Wait() {
	a.wg.Wait()
}

type rememberJob struct {
	provider provider.Provider
	store    *memory.Store
	indexer  memory.Indexer
	convID   string
	input    string
	reply    string
}

func remember(ctx context.Context, job rememberJob) {
	if job.store == nil {
		return
	}
	contents, err := memory.Extract(ctx, job.provider, []provider.Message{
		{Role: "user", Content: job.input},
		{Role: "assistant", Content: job.reply},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: extract: %v\n", err)
		return
	}
	saved, err := job.store.SaveExtracted(contents, job.convID, uuid.NewString())
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: save: %v\n", err)
	}
	if job.indexer == nil {
		return
	}
	for _, m := range saved {
		if err := job.indexer.Index(ctx, m); err != nil {
			fmt.Fprintf(os.Stderr, "memory: index: %v\n", err)
		}
	}
}

func (s *Session) persist() error {
	s.conv.UpdatedAt = time.Now().UTC()
	return s.store.Save(s.conv)
}

func (a *Agent) roundTools() (map[string]tool.Tool, []provider.ToolDef) {
	list := a.list
	if a.Catalog != nil {
		if extra := a.Catalog.Tools(); len(extra) > 0 {
			list = make([]tool.Tool, 0, len(a.list)+len(extra))
			list = append(list, a.list...)
			list = append(list, extra...)
		}
	}
	byName := make(map[string]tool.Tool, len(list))
	defs := make([]provider.ToolDef, 0, len(list))
	for _, t := range list {
		spec := t.Spec()
		if _, ok := byName[spec.Name]; ok {
			continue
		}
		byName[spec.Name] = t
		defs = append(defs, provider.ToolDef{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return byName, defs
}

func (a *Agent) runTool(ctx context.Context, tools map[string]tool.Tool, call provider.ToolCall) string {
	t, ok := tools[call.Function.Name]
	if !ok {
		return fmt.Sprintf("unknown tool: %s", call.Function.Name)
	}

	result, err := t.Call(ctx, json.RawMessage(call.Function.Arguments))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}
