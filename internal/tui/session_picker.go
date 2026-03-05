package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tokuhirom/ashron/internal/session"
)

type pickResult struct {
	SessionID string
	Cancelled bool
}

type sessionPickerModel struct {
	sessions []session.Summary
	cursor   int
	width    int
	result   pickResult
}

func newSessionPickerModel(sessions []session.Summary) *sessionPickerModel {
	return &sessionPickerModel{sessions: sessions}
}

func (m *sessionPickerModel) Init() tea.Cmd {
	return nil
}

func (m *sessionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.sessions) {
				m.cursor++
			}
			return m, nil
		case tea.KeyEnter:
			if m.cursor == 0 {
				m.result = pickResult{}
			} else {
				m.result = pickResult{SessionID: m.sessions[m.cursor-1].ID}
			}
			return m, tea.Quit
		case tea.KeyEsc:
			m.result = pickResult{Cancelled: true}
			return m, tea.Quit
		default:
			switch strings.ToLower(msg.Text) {
			case "j":
				if m.cursor < len(m.sessions) {
					m.cursor++
				}
				return m, nil
			case "k":
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			case "q":
				m.result = pickResult{Cancelled: true}
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *sessionPickerModel) View() tea.View {
	title := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("Ashron Session Picker")
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Enter: select  Up/Down or j/k: move  Esc/q: cancel")

	var lines []string
	lines = append(lines, title, hint, "")

	lines = append(lines, m.renderLine(0, "Start new session", ""))
	for i, s := range m.sessions {
		meta := fmt.Sprintf("%s  %s  %s/%s",
			s.CreatedAt.Local().Format(time.DateTime),
			shortPath(s.WorkingDir),
			s.Provider,
			s.Model,
		)
		lines = append(lines, m.renderLine(i+1, s.ID, meta))
	}

	content := strings.Join(lines, "\n")
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *sessionPickerModel) renderLine(index int, title, meta string) string {
	prefix := "  "
	if m.cursor == index {
		prefix = "> "
	}
	line := prefix + title
	if meta != "" {
		line += "\n   " + lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render(meta)
	}
	return line
}

func shortPath(path string) string {
	if path == "" {
		return "-"
	}
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		return path
	}
	return base
}

// SelectSessionInteractive shows a small TUI picker and returns selected session ID.
// Empty string means start a new session.
func SelectSessionInteractive(sessions []session.Summary) (pickResult, error) {
	if len(sessions) == 0 {
		return pickResult{}, nil
	}
	model := newSessionPickerModel(sessions)
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return pickResult{}, err
	}
	picked, ok := finalModel.(*sessionPickerModel)
	if !ok {
		return pickResult{}, fmt.Errorf("unexpected model type: %T", finalModel)
	}
	return picked.result, nil
}
