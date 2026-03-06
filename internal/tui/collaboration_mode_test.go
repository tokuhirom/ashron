package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestToggleCollaborationMode_Cycles(t *testing.T) {
	m := newE2EModel(t, "http://localhost:0")
	m.collaborationMode = "default"

	m.toggleCollaborationMode()
	if m.collaborationMode != "auto_edit" {
		t.Fatalf("expected auto_edit, got %q", m.collaborationMode)
	}

	m.toggleCollaborationMode()
	if m.collaborationMode != "plan" {
		t.Fatalf("expected plan, got %q", m.collaborationMode)
	}

	m.toggleCollaborationMode()
	if m.collaborationMode != "default" {
		t.Fatalf("expected default, got %q", m.collaborationMode)
	}
}

func TestToggleCollaborationMode_WorksDuringLoading(t *testing.T) {
	m := newE2EModel(t, "http://localhost:0")
	m.collaborationMode = "default"
	m.loading = true

	m.toggleCollaborationMode()
	if m.collaborationMode != "auto_edit" {
		t.Fatalf("toggleCollaborationMode should work during loading: got %q", m.collaborationMode)
	}
}

func TestShiftTab_WorksDuringLoading(t *testing.T) {
	m := newE2EModel(t, "http://localhost:0")
	m.collaborationMode = "default"
	m.loading = true

	keyMsg := tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	_, _ = m.Update(keyMsg)

	if m.collaborationMode != "auto_edit" {
		t.Fatalf("Shift+Tab during loading: expected auto_edit, got %q", m.collaborationMode)
	}
}
