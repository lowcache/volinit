// Package runtime is the Bubble Tea program. Rendering here is intentionally
// plain: the visual language is designed and approved separately.
package runtime

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
	"github.com/lowcache/volinit/internal/hero"
	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/run"
	"github.com/lowcache/volinit/internal/theme"
)

type row struct {
	repo   string
	path   string
	action registry.Action
}

// mode is the one-question-at-a-time gate in front of execution. Nothing runs
// while a gate is open, and every gate is escapable.
type mode int

const (
	modeList mode = iota
	modeParam
	modeConfirm
)

// stage is the greeting/working axis: separate from mode, which tracks
// list/confirm/param once the cockpit is already in the working state.
type stage int

const (
	stageFullBleed stage = iota // the greeting: art edge to edge
	stageSidebar                // the working state: art compressed, list live
)

// defaultHeight stands in until the first WindowSizeMsg arrives, which is
// immediately in a real terminal and never in a test.
const defaultHeight = 24

// Model is the cockpit state. Flat list for now; the tree lands with the
// visual design.
type Model struct {
	rows    []row
	cursor  int
	offset  int // index of the first visible row
	height  int // terminal rows, from tea.WindowSizeMsg
	palette theme.Palette
	notices []string // degraded-state lines; shown, never fatal
	mode    mode
	stage   stage
	tier    hero.Tier
	param   string
	status  string
	quit    bool
}

func New(repos []registry.Repo, p theme.Palette, notices []string, tier hero.Tier) Model {
	var rows []row
	for _, r := range repos {
		for _, a := range r.Actions {
			rows = append(rows, row{repo: r.Name, path: r.Path, action: a})
		}
	}
	m := Model{rows: rows, palette: p, notices: notices, height: defaultHeight, stage: stageFullBleed, tier: tier}
	if tier == hero.T0 {
		m.stage = stageSidebar // T0 prints State B directly, with no transition
	}
	return m
}

// Init satisfies tea.Model. v2's Model.Init returns only a Cmd (unlike the
// (Model, Cmd) shape used elsewhere in this file's Update) — verified via
// `go doc charm.land/bubbletea/v2 Model` against the vendored v2.0.9.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.clamp()
	case run.DoneMsg:
		if msg.Err != nil {
			m.status = fmt.Sprintf("%s failed: %v", msg.Action, msg.Err)
		} else {
			m.status = msg.Action + " finished"
		}
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m Model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.stage == stageFullBleed {
		// Quit keys leave straight from the greeting. Any other key dismisses
		// and is consumed: several fleet targets send mail or deploy live sites.
		switch k.String() {
		case "q", "esc", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		}
		m.stage = stageSidebar
		return m, nil
	}
	switch m.mode {
	case modeParam:
		switch k.String() {
		case "esc", "ctrl+c":
			return m.cancel(), nil
		case "enter":
			return m.gate()
		case "backspace":
			if r := []rune(m.param); len(r) > 0 {
				m.param = string(r[:len(r)-1])
			}
		default:
			// Text is populated only for printable keys, so this cannot
			// swallow a control key into the buffer.
			m.param += k.Text
		}
		return m, nil

	case modeConfirm:
		// Anything but an explicit yes declines: the gate exists for targets
		// that send mail and deploy live sites.
		if s := k.String(); s == "y" || s == "Y" {
			m.mode = modeList
			return m.start()
		}
		return m.cancel(), nil
	}

	switch k.String() {
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	case "j", "down":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			m.clamp()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.clamp()
		}
	case "enter":
		if len(m.rows) == 0 {
			return m, nil
		}
		m.status = ""
		if m.rows[m.cursor].action.ParamName != "" {
			m.mode = modeParam
			m.param = ""
			return m, nil
		}
		return m.gate()
	}
	return m, nil
}

// gate opens the confirmation prompt when the action is gated, and
// otherwise runs. It is the last thing between a keypress and execution.
func (m Model) gate() (tea.Model, tea.Cmd) {
	if m.rows[m.cursor].action.Gated() {
		m.mode = modeConfirm
		return m, nil
	}
	m.mode = modeList
	return m.start()
}

func (m Model) cancel() Model {
	m.mode = modeList
	m.param = ""
	m.status = "cancelled"
	return m
}

// start hands the command to the execution path the action asks for.
// Foreground work goes through tea.ExecProcess, which releases the terminal so
// sudo can prompt on a real TTY; detached work must survive the cockpit
// exiting, so it is never suspended into it.
func (m Model) start() (tea.Model, tea.Cmd) {
	r := m.rows[m.cursor]
	cmd := run.For(r.action, r.path, m.param)
	name := r.action.Name
	m.param = ""

	if r.action.Detach {
		m.status = "started " + name + " detached"
		return m, func() tea.Msg {
			// systemd-run returns as soon as the unit is registered; this
			// reports the registration, not the build.
			return run.DoneMsg{Action: name, Err: cmd.Run()}
		}
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return run.DoneMsg{Action: name, Err: err}
	})
}

// visible is how many list rows fit: the terminal minus the notice lines and
// the footer.
func (m Model) visible() int {
	h := m.height
	if h <= 0 {
		h = defaultHeight
	}
	if n := h - len(m.notices) - 1; n > 0 {
		return n
	}
	return 1
}

// clamp scrolls the window so the cursor stays inside it.
func (m *Model) clamp() {
	v := m.visible()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+v {
		m.offset = m.cursor - v + 1
	}
	if max := len(m.rows) - v; m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) View() tea.View {
	if m.stage == stageFullBleed {
		return m.viewFullBleed()
	}
	return m.viewSidebar()
}

// viewFullBleed is the greeting. Art is a placeholder here — the cell-art
// pipeline and the morph land in a later task — but it must never show the
// action list, since nothing has been dismissed yet.
func (m Model) viewFullBleed() tea.View {
	fg := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.OnSurface))
	v := tea.NewView(fg.Render("volinit") + "\n\npress any key")
	v.AltScreen = true
	return v
}

func (m Model) viewSidebar() tea.View {
	fg := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.OnSurface))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Primary))

	var b strings.Builder
	for _, n := range m.notices {
		b.WriteString(fg.Render(n))
		b.WriteString("\n")
	}

	if len(m.rows) == 0 {
		// Never leave the operator staring at a blank alt screen with no way
		// out: an empty fleet says so, and says how to leave.
		b.WriteString(fg.Render("no runnable targets found"))
		b.WriteString("\n")
	} else {
		end := m.offset + m.visible()
		if end > len(m.rows) {
			end = len(m.rows)
		}
		for i := m.offset; i < end; i++ {
			r := m.rows[i]
			line := fmt.Sprintf("  %-14s %-18s %s", r.repo, r.action.Name, r.action.Description)
			if i == m.cursor {
				b.WriteString(sel.Render("▸" + line))
			} else {
				b.WriteString(fg.Render(" " + line))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(fg.Render(m.footer()))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m Model) footer() string {
	switch m.mode {
	case modeParam:
		a := m.rows[m.cursor].action
		prompt := a.ParamPrompt
		if prompt == "" {
			prompt = a.ParamName
		}
		return fmt.Sprintf("%s: %s_   enter runs · esc cancels", prompt, m.param)
	case modeConfirm:
		a := m.rows[m.cursor].action
		prompt := a.Confirm
		if prompt == "" {
			prompt = fmt.Sprintf("run %s in %s?", a.Name, m.rows[m.cursor].repo)
		}
		return prompt + " [y/N]"
	}
	if len(m.rows) == 0 {
		return "q quits"
	}
	keys := "j/k move · enter runs · q quits"
	if m.status != "" {
		return fmt.Sprintf("%s   %s", m.status, keys)
	}
	return fmt.Sprintf("%d/%d   %s", m.cursor+1, len(m.rows), keys)
}

// Run starts the cockpit. Bubble Tea restores the terminal on panic and
// SIGINT itself, so no guard is wrapped around this.
func Run(repos []registry.Repo, p theme.Palette, notices []string) error {
	tier := hero.Detect(os.Getenv, term.IsTerminal(os.Stdout.Fd()))
	_, err := tea.NewProgram(New(repos, p, notices, tier)).Run()
	return err
}
