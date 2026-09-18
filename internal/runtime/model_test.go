package runtime

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/theme"
)

func fixture() Model {
	return New([]registry.Repo{
		{Name: "cfg", Branch: registry.BranchSystem, Path: "/tmp/cfg", Actions: []registry.Action{
			{Name: "switch"}, {Name: "build"},
		}},
	}, theme.Default(), nil)
}

// press feeds one key and returns the model, dropping the command. No test
// here invokes a returned tea.Cmd: doing so would start a real process.
func press(t *testing.T, m Model, k tea.KeyPressMsg) Model {
	t.Helper()
	next, _ := m.Update(k)
	return next.(Model)
}

func typed(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

var enter = tea.KeyPressMsg{Code: tea.KeyEnter}

func TestDownMovesTheCursor(t *testing.T) {
	m := fixture()
	next, _ := m.Update(typed('j'))
	if next.(Model).cursor != 1 {
		t.Errorf("cursor = %d, want 1", next.(Model).cursor)
	}
}

func TestCursorStopsAtTheEnd(t *testing.T) {
	m := fixture()
	m.cursor = 1
	next, _ := m.Update(typed('j'))
	if next.(Model).cursor != 1 {
		t.Errorf("cursor ran past the end: %d", next.(Model).cursor)
	}
}

func TestUpMovesTheCursorAndStopsAtTheTop(t *testing.T) {
	m := fixture()
	m.cursor = 1
	m = press(t, m, typed('k'))
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}
	if m = press(t, m, typed('k')); m.cursor != 0 {
		t.Errorf("cursor ran past the top: %d", m.cursor)
	}
}

func TestQuitAsksTheProgramToStop(t *testing.T) {
	_, cmd := fixture().Update(typed('q'))
	if cmd == nil {
		t.Fatal("q returned no command; nothing would quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q did not return tea.Quit, got %T", cmd())
	}
}

func TestViewListsEveryAction(t *testing.T) {
	v := fixture().View()
	for _, want := range []string{"switch", "build"} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("view missing %q:\n%s", want, v.Content)
		}
	}
}

// An empty fleet must not render as a blank alt screen with no way out.
func TestZeroRowsSaysSoAndSaysHowToLeave(t *testing.T) {
	m := New(nil, theme.Default(), []string{"volinit: nothing found under /nowhere"})
	v := m.View()
	for _, want := range []string{"no runnable targets found", "q quits", "nothing found under /nowhere"} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("view missing %q:\n%s", want, v.Content)
		}
	}
	if _, cmd := m.Update(enter); cmd != nil {
		t.Error("enter with no rows produced a command")
	}
}

func TestEnterConfirmsBeforeRunning(t *testing.T) {
	m := New([]registry.Repo{{Name: "blog", Path: "/tmp/blog", Actions: []registry.Action{
		{Name: "deploy", Confirm: "Deploy live. Continue?"},
	}}}, theme.Default(), nil)

	next, cmd := m.Update(enter)
	m = next.(Model)
	if cmd != nil {
		t.Fatal("enter ran a confirm-gated action without asking")
	}
	if m.mode != modeConfirm {
		t.Fatalf("mode = %v, want modeConfirm", m.mode)
	}
	if !strings.Contains(m.View().Content, "Deploy live. Continue?") {
		t.Error("confirm prompt not rendered")
	}

	// Anything but an explicit yes declines.
	declined, cmd := m.Update(typed('n'))
	if cmd != nil {
		t.Error("declining still ran the action")
	}
	if declined.(Model).mode != modeList {
		t.Error("declining left the gate open")
	}
	if _, cmd := m.Update(typed('y')); cmd == nil {
		t.Error("confirming did not run the action")
	}
}

func TestEnterCollectsParamBeforeRunning(t *testing.T) {
	m := New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "anon-run", ParamName: "CMD", ParamPrompt: "Command to jail"},
	}}}, theme.Default(), nil)

	next, cmd := m.Update(enter)
	m = next.(Model)
	if cmd != nil {
		t.Fatal("enter ran a parameterised action before collecting the param")
	}
	if m.mode != modeParam {
		t.Fatalf("mode = %v, want modeParam", m.mode)
	}
	for _, r := range "id" {
		m = press(t, m, typed(r))
	}
	m = press(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = press(t, m, typed('s'))
	if m.param != "is" {
		t.Errorf("param = %q, want %q", m.param, "is")
	}
	if got := m.View().Content; !strings.Contains(got, "Command to jail: is") {
		t.Errorf("param prompt not rendered:\n%s", got)
	}
	if _, cmd := m.Update(enter); cmd == nil {
		t.Error("enter after the param did not run the action")
	}
}

func TestDetachedActionStartsWithoutSuspending(t *testing.T) {
	m := New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "switch", Detach: true},
	}}}, theme.Default(), nil)

	next, cmd := m.Update(enter)
	if cmd == nil {
		t.Fatal("detached action produced no command")
	}
	if got := next.(Model).status; !strings.Contains(got, "detached") {
		t.Errorf("status = %q, want it to report the detached start", got)
	}
}

func wide(n int) Model {
	var actions []registry.Action
	for i := 0; i < n; i++ {
		actions = append(actions, registry.Action{Name: fmt.Sprintf("a%02d", i)})
	}
	return New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: actions}}, theme.Default(), nil)
}

func TestViewRendersOnlyWhatFits(t *testing.T) {
	m := wide(30)
	m, _ = mustUpdate(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})

	got := m.View().Content
	if lines := strings.Count(got, "\n") + 1; lines > 10 {
		t.Errorf("rendered %d lines into a 10-row terminal:\n%s", lines, got)
	}
	if !strings.Contains(got, "a00") {
		t.Error("first row missing")
	}
	if strings.Contains(got, "a29") {
		t.Error("rendered a row that does not fit")
	}
}

func TestWindowFollowsTheCursor(t *testing.T) {
	m := wide(30)
	m, _ = mustUpdate(t, m, tea.WindowSizeMsg{Width: 80, Height: 10})
	for i := 0; i < 20; i++ {
		m = press(t, m, typed('j'))
	}
	got := m.View().Content
	if !strings.Contains(got, "a20") {
		t.Errorf("cursor row not visible after scrolling:\n%s", got)
	}
	if strings.Contains(got, "a00") {
		t.Errorf("window did not scroll:\n%s", got)
	}
	for i := 0; i < 20; i++ {
		m = press(t, m, typed('k'))
	}
	if got := m.View().Content; !strings.Contains(got, "a00") {
		t.Errorf("window did not scroll back:\n%s", got)
	}
}

func mustUpdate(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}
