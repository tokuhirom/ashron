package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/session"
)

func TestAddUserMessagePersistsImmediately(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse { return nil })
	defer server.Close()

	m := newE2EModel(t, server.URL)
	m.addUserMessage("persist me")

	loaded, err := session.Load(m.sess.ID)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if got := loaded.Messages[len(loaded.Messages)-1].Content; got != "persist me" {
		t.Fatalf("last saved message = %q, want %q", got, "persist me")
	}
}

func TestCtrlCQuitPersistsPendingMessages(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse { return nil })
	defer server.Close()

	m := newE2EModel(t, server.URL)
	m.messages = append(m.messages, api.NewUserMessage("unsaved before quit"))

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatalf("expected quit command")
	}

	loaded, err := session.Load(m.sess.ID)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if got := loaded.Messages[len(loaded.Messages)-1].Content; got != "unsaved before quit" {
		t.Fatalf("last saved message = %q, want %q", got, "unsaved before quit")
	}
}

func TestCancelCurrentRequestPersists(t *testing.T) {
	server := newDummyChatServer(t, func(_ int, _ api.ChatCompletionRequest) []api.StreamResponse { return nil })
	defer server.Close()

	m := newE2EModel(t, server.URL)
	m.messages = append(m.messages, api.NewUserMessage("unsaved before cancel"))
	m.loading = true
	_, cancel := context.WithCancel(context.Background())
	m.cancelAPICall = cancel

	m.cancelCurrentRequest()

	loaded, err := session.Load(m.sess.ID)
	if err != nil {
		t.Fatalf("Load session: %v", err)
	}
	if got := loaded.Messages[len(loaded.Messages)-1].Content; got != "unsaved before cancel" {
		t.Fatalf("last saved message = %q, want %q", got, "unsaved before cancel")
	}
}
