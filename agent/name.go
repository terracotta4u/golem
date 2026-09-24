package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/provider"
)

const namePrompt = `Name this conversation in a few words based on the user's first message. Reply with the title only.`

func (s *Session) nameChat(ctx context.Context, input string) {
	if s.agent.Fast == nil {
		return
	}
	msg, err := s.agent.Fast.Chat(ctx, provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: namePrompt},
			{Role: "user", Content: input},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "name: %v\n", err)
		return
	}
	if title := conversation.TitleFrom(msg.Content); title != "" {
		s.conv.Title = title
	}
}
