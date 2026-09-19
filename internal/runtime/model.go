// Package runtime is the Bubble Tea program. Rendering here is intentionally
// plain: the visual language is designed and approved separately.
package runtime

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
	"github.com/lowcache/volinit/internal/hero"
	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/run"
	"github.com/lowcache/volinit/internal/theme"
)

// row is one entry at the current menu level. repo, path and action are
// set only at the task level.
type row struct {
	label  string
	detail string
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
	stageFullBleed stage = iota // the greeting: the assembly exploded, edge to edge
	stageMorph                  // the one-way transition; any key lands it
	stageSidebar                // the working state: assembly in a strip, list live
)

const (
	morphFrames   = 18
	morphInterval = 16 * time.Millisecond
)

// morphMsg advances the morph one frame.
type morphMsg struct{}

func morphTick() tea.Cmd {
	return tea.Tick(morphInterval, func(time.Time) tea.Msg { return morphMsg{} })
}

// defaultHeight stands in until the first WindowSizeMsg arrives, which is
// immediately in a real terminal and never in a test.
const defaultHeight = 24

const (
	defaultWidth  = 80 // stands in until the first WindowSizeMsg, like defaultHeight
	sidebarCols   = 24 // the assembled strip beside the list
	minStripWidth = 72 // below this the list gets every column
)

// Model is the cockpit state: a three-level menu (doors, subsystems, tasks)
// walked by path, with rows holding the current level's entries.
type Model struct {
	menu    []registry.Door
	path    []int // entry chosen at each level above the current one
	rows    []row
	cursor  int
	offset  int // index of the first visible row
	width   int // terminal columns, from tea.WindowSizeMsg
	height  int // terminal rows, from tea.WindowSizeMsg
	palette theme.Palette
	notices []string // degraded-state lines; shown, never fatal
	mode    mode
	stage   stage
	frame   int // morph progress, 0..morphFrames
	tier    hero.Tier
	param   string
	status  string
	quit    bool
}

func New(repos []registry.Repo, p theme.Palette, notices []string, tier hero.Tier) Model {
	m := Model{menu: registry.Menu(repos), palette: p, notices: notices, width: defaultWidth, height: defaultHeight, stage: stageFullBleed, tier: tier}
	m.load()
	if tier == hero.T0 {
		m.stage = stageSidebar // T0 prints State B directly, with no transition
	}
	return m
}

// atTasks reports the task level: the only level where enter runs anything.
func (m Model) atTasks() bool { return len(m.path) == 2 }

// load fills rows with the entries of the level path points at.
func (m *Model) load() {
	var rows []row
	switch len(m.path) {
	case 0:
		for _, d := range m.menu {
			rows = append(rows, row{label: d.Name, detail: count(len(d.Groups), d.Noun)})
		}
	case 1:
		for _, g := range m.menu[m.path[0]].Groups {
			rows = append(rows, row{label: g.Name, detail: count(len(g.Actions), "tasks")})
		}
	default:
		g := m.menu[m.path[0]].Groups[m.path[1]]
		for _, a := range g.Actions {
			rows = append(rows, row{label: a.Name, detail: a.Description, repo: g.Repo, path: g.Path, action: a})
		}
	}
	m.rows = rows
}

// open descends into the entry under the cursor.
func (m Model) open() Model {
	// Copy before appending: earlier Models may share this backing array.
	m.path = append(append([]int(nil), m.path...), m.cursor)
	m.cursor, m.offset = 0, 0
	m.load()
	return m
}

// back climbs one level, landing on the entry it came from.
func (m Model) back() Model {
	last := m.path[len(m.path)-1]
	m.path = m.path[:len(m.path)-1]
	m.load()
	m.cursor, m.offset = last, 0
	m.clamp()
	return m
}

// crumb names the path, e.g. "System › Secret Management"; empty at the doors.
func (m Model) crumb() string {
	switch len(m.path) {
	case 0:
		return ""
	case 1:
		return m.menu[m.path[0]].Name
	}
	return m.menu[m.path[0]].Name + " › " + m.menu[m.path[0]].Groups[m.path[1]].Name
}

// count reads "1 site", "4 sites".
func count(n int, noun string) string {
	if n == 1 {
		noun = strings.TrimSuffix(noun, "s")
	}
	return fmt.Sprintf("%d %s", n, noun)
}

// Init satisfies tea.Model. v2's Model.Init returns only a Cmd (unlike the
// (Model, Cmd) shape used elsewhere in this file's Update) — verified via
// `go doc charm.land/bubbletea/v2 Model` against the vendored v2.0.9.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
	case run.DoneMsg:
		if msg.Err != nil {
			m.status = fmt.Sprintf("%s failed: %v", msg.Action, msg.Err)
		} else {
			m.status = msg.Action + " finished"
		}
	case morphMsg:
		// A tick can arrive after a key already landed the morph; drop it.
		if m.stage != stageMorph {
			return m, nil
		}
		m.frame++
		if m.frame >= morphFrames {
			m.stage = stageSidebar
			return m, nil
		}
		return m, morphTick()
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m Model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.stage {
	case stageFullBleed:
		// Quit keys leave straight from the greeting. Any other key starts the
		// morph and is consumed: several fleet targets send mail or deploy.
		switch k.String() {
		case "q", "esc", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		}
		m.stage = stageMorph
		return m, morphTick()
	case stageMorph:
		// Interruptible: land now, and consume this key too.
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
	case "q", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	case "esc", "h", "left", "backspace":
		if len(m.path) > 0 {
			return m.back(), nil
		}
		if k.String() == "esc" {
			m.quit = true
			return m, tea.Quit
		}
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
	case "l", "right":
		if !m.atTasks() && len(m.rows) > 0 {
			return m.open(), nil
		}
	case "enter":
		if len(m.rows) == 0 {
			return m, nil
		}
		if !m.atTasks() {
			return m.open(), nil
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

// visible is how many list rows fit: the terminal minus the notice lines,
// the breadcrumb when there is one, and the footer.
func (m Model) visible() int {
	h := m.height
	if h <= 0 {
		h = defaultHeight
	}
	chrome := len(m.notices) + 1
	if len(m.path) > 0 {
		chrome++
	}
	if n := h - chrome; n > 0 {
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
	var content string
	switch m.stage {
	case stageFullBleed:
		pose, labels := hero.GreetingPose(m.width, m.height)
		content = hero.Frame(m.width, m.height, pose, labels, m.palette)
	case stageMorph:
		content = m.viewMorph()
	default:
		content = m.viewSidebar()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// viewMorph closes the explosion while the assembly travels into the strip:
// the same parts, arriving where they will live.
func (m Model) viewMorph() string {
	from, _ := hero.GreetingPose(m.width, m.height)
	to := hero.StripPose(m.stripCols(), m.height)
	t := ease(float64(m.frame) / morphFrames)
	return hero.Frame(m.width, m.height, hero.Lerp(from, to, t), false, m.palette)
}

// ease is cubic in-out: the parts start gently and settle gently.
func ease(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - math.Pow(-2*t+2, 3)/2
}

// stripCols is the sidebar strip's width, or 0 when there is no art: at T0,
// or when the terminal is too narrow to spare the columns.
func (m Model) stripCols() int {
	if m.tier == hero.T0 || m.width < minStripWidth {
		return 0
	}
	return sidebarCols
}

func (m Model) viewSidebar() string {
	fg := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.OnSurface))
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Primary))

	var b strings.Builder
	for _, n := range m.notices {
		b.WriteString(fg.Render(n))
		b.WriteString("\n")
	}

	if c := m.crumb(); c != "" {
		b.WriteString(fg.Render(c))
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
			line := fmt.Sprintf("  %-26s %s", r.label, r.detail)
			if i == m.cursor {
				b.WriteString(sel.Render("▸" + line))
			} else {
				b.WriteString(fg.Render(" " + line))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(fg.Render(m.footer()))

	list := b.String()
	if sc := m.stripCols(); sc > 0 {
		strip := hero.Frame(sc, m.height, hero.StripPose(sc, m.height), false, m.palette)
		return lipgloss.JoinHorizontal(lipgloss.Top, strip, "  ", list)
	}
	return list
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
	keys := "j/k move · enter opens · q quits"
	switch {
	case m.atTasks():
		keys = "j/k move · enter runs · esc back · q quits"
	case len(m.path) > 0:
		keys = "j/k move · enter opens · esc back · q quits"
	}
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
