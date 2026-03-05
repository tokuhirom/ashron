package tui

import (
	"strings"
	"testing"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/session"
)

func TestStartNewSessionResetsConversationAndSessionID(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse { return nil })
	defer server.Close()

	m := newE2EModel(t, server.URL)
	oldID := m.sess.ID
	m.addUserMessage("keep in old session")

	_ = m.StartNewSession()

	if m.sess == nil || m.sess.ID == "" {
		t.Fatalf("new session should be set")
	}
	if m.sess.ID == oldID {
		t.Fatalf("session id should change on /new")
	}
	if len(m.messages) == 0 || m.messages[0].Role != "system" {
		t.Fatalf("new session should start with system prompt")
	}
	for _, msg := range m.messages {
		if msg.Role == "user" && strings.Contains(msg.Content, "keep in old session") {
			t.Fatalf("old conversation should not remain in new session")
		}
	}

	oldSess, err := session.Load(oldID)
	if err != nil {
		t.Fatalf("old session must remain loadable: %v", err)
	}
	if got := oldSess.Messages[len(oldSess.Messages)-1].Content; got != "keep in old session" {
		t.Fatalf("old session latest message = %q", got)
	}
}

func TestClearCommandResetsDisplayHeader(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse { return nil })
	defer server.Close()

	m := newE2EModel(t, server.URL)
	m.AddDisplayContent("temp line")

	cmd, ok := m.commandRegistry.GetCommand("/clear")
	if !ok {
		t.Fatalf("/clear command not found")
	}
	_ = cmd.Body(m.commandRegistry, m, nil)

	joined := strings.Join(m.displayContent, "\n")
	if !strings.Contains(joined, "Ashron - AI Coding Assistant") {
		t.Fatalf("header missing after /clear")
	}
	if strings.Contains(joined, "temp line") {
		t.Fatalf("/clear should remove previous display content")
	}
}
