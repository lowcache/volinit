// Package runtime is the Bubble Tea program. Rendering here is intentionally
// plain: the visual language is designed and approved separately.
package runtime

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/theme"
)

type row struct {
	repo   string
	action registry.Action
}

// Model is the cockpit state. Flat list for now; the tree lands with the
// visual design.
type Model struct {
	rows    []row
	cursor  int
	palette theme.Palette
	quit    bool
}

func New(repos []registry.Repo, p theme.Palette) Model {
	var rows []row
	for _, r := range repos {
		for _, a := range r.Actions {
			rows = append(rows, row{repo: r.Name, action: a})
		}
	}
	return Model{rows: rows, palette: p}
}

// Init satisfies tea.Model. v2's Model.Init returns only a Cmd (unlike the
// (Model, Cmd) shape used elsewhere in this file's Update) — verified via
// `go doc charm.land/bubbletea/v2 Model` against the vendored v2.0.9.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "q", "esc", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	fg := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.OnSurface))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Primary))

	var b strings.Builder
	for i, r := range m.rows {
		line := fmt.Sprintf("  %-14s %-18s %s", r.repo, r.action.Name, r.action.Description)
		if i == m.cursor {
			b.WriteString(sel.Render("▸" + line))
		} else {
			b.WriteString(fg.Render(" " + line))
		}
		b.WriteString("\n")
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// Run starts the cockpit. Bubble Tea restores the terminal on panic and
// SIGINT itself, so no guard is wrapped around this.
func Run(repos []registry.Repo, p theme.Palette) error {
	_, err := tea.NewProgram(New(repos, p)).Run()
	return err
}
