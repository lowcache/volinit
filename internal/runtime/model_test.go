package runtime

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lowcache/volinit/internal/hero"
	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/theme"
)

func fixture() Model {
	return New([]registry.Repo{
		{Name: "cfg", Branch: registry.BranchSystem, Path: "/tmp/cfg", Actions: []registry.Action{
			{Name: "switch"}, {Name: "build"},
		}},
	}, theme.Default(), nil, hero.T1)
}

// press feeds one key and returns the model, dropping the command. No test
// here invokes a returned tea.Cmd: doing so would start a real process.
func press(t *testing.T, m Model, k tea.KeyPressMsg) Model {
	t.Helper()
	next, _ := m.Update(k)
	return next.(Model)
}

func typed(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func boolp(b bool) *bool { return &b }

var enter = tea.KeyPressMsg{Code: tea.KeyEnter}

// keyPress builds a tea.KeyPressMsg by name, reusing the same construction
// as typed/enter above rather than inventing a second form: named keys map
// to their Code, everything else is a single printable rune.
func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return enter
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	default:
		return typed([]rune(s)[0])
	}
}

// dismiss advances a fresh model past the greeting and the morph so a test
// can exercise the working state without exercising either.
func dismiss(m Model) Model {
	next, _ := m.Update(typed(' '))
	next, _ = next.(Model).Update(typed(' '))
	return next.(Model)
}

// tasks dismisses the greeting, then opens the first door and its first
// subsystem: the task level, the only place enter runs anything.
func tasks(m Model) Model {
	m = dismiss(m)
	for i := 0; i < 2; i++ {
		next, _ := m.Update(enter)
		m = next.(Model)
	}
	return m
}

// menuFixture spans all three doors, with a sectioned system repo.
func menuFixture() Model {
	return New([]registry.Repo{
		{Name: "cfg", Path: "/tmp/cfg", Branch: registry.BranchSystem, Actions: []registry.Action{
			{Name: "switch", Section: "System Operations"},
			{Name: "sops-edit", Section: "Secret Management"},
		}},
		{Name: "wiki", Path: "/tmp/wiki", Branch: registry.BranchWriting, Actions: []registry.Action{{Name: "serve"}}},
		{Name: "app", Path: "/tmp/app", Branch: registry.BranchCode, Actions: []registry.Action{{Name: "test"}}},
	}, theme.Default(), nil, hero.T1)
}

func TestStartsFullBleed(t *testing.T) {
	m := fixture()
	if m.stage != stageFullBleed {
		t.Fatal("a fresh cockpit must open at full bleed")
	}
}

func TestAnyKeyStartsTheMorph(t *testing.T) {
	next, cmd := fixture().Update(keyPress("x"))
	if next.(Model).stage != stageMorph {
		t.Fatal("any key must start the morph")
	}
	if cmd == nil {
		t.Fatal("the morph was started with no tick to drive it")
	}
}

// TestTheDoorIsOneWay pins the one-way door: once dismissed, nothing the
// model handles — keys, morph ticks, a resize — may reopen the greeting.
func TestTheDoorIsOneWay(t *testing.T) {
	var next tea.Model = fixture()
	next, _ = next.Update(keyPress("x"))
	msgs := []tea.Msg{morphMsg{}, morphMsg{}}
	for _, k := range []string{
		"x", "enter", "j", "k", "esc", "y", "n", " ",
		"down", "up", "ctrl+c", "backspace", "Y", "q",
	} {
		msgs = append(msgs, keyPress(k), morphMsg{})
	}
	msgs = append(msgs, tea.WindowSizeMsg{Width: 120, Height: 40})
	for _, msg := range msgs {
		next, _ = next.(Model).Update(msg)
		if next.(Model).stage == stageFullBleed {
			t.Fatalf("%#v reopened the greeting", msg)
		}
	}
}

// Neither the dismissing key nor a key that interrupts the morph may act:
// several fleet targets send irreversible mail or deploy live sites.
func TestDismissDoesNotAlsoAct(t *testing.T) {
	m := New([]registry.Repo{{Name: "blog", Path: "/tmp/blog", Actions: []registry.Action{
		{Name: "deploy", Confirm: "Deploy live. Continue?"},
	}}}, theme.Default(), nil, hero.T1)

	next, cmd := m.Update(enter)
	if got := next.(Model); got.stage != stageMorph || got.mode != modeList {
		t.Fatal("enter at the greeting must start the morph and nothing else")
	}
	// Safe to invoke: it is the morph tick, which starts no process.
	if _, ok := cmd().(morphMsg); !ok {
		t.Fatal("dismissing returned something other than the morph tick")
	}

	next, cmd = next.(Model).Update(enter)
	got := next.(Model)
	if got.stage != stageSidebar {
		t.Fatal("a key during the morph must land in the sidebar")
	}
	if cmd != nil || got.mode != modeList {
		t.Fatal("the key that interrupted the morph also acted")
	}
}

func TestFullBleedShowsNoActions(t *testing.T) {
	v := fixture().View()
	if strings.Contains(v.Content, "switch") {
		t.Fatal("the greeting must show art only, no action list")
	}
}

func TestMorphRunsToTheSidebar(t *testing.T) {
	next, cmd := fixture().Update(keyPress("x"))
	for i := 0; i < morphFrames; i++ {
		if cmd == nil {
			t.Fatalf("the morph stopped ticking after %d frames", i)
		}
		next, cmd = next.(Model).Update(morphMsg{})
	}
	if next.(Model).stage != stageSidebar {
		t.Fatal("the morph never settled")
	}
	if cmd != nil {
		t.Error("a settled morph kept ticking")
	}
}

func TestStaleTickIsIgnored(t *testing.T) {
	next, cmd := dismiss(fixture()).Update(morphMsg{})
	if cmd != nil || next.(Model).stage != stageSidebar {
		t.Fatal("a tick after the morph landed must do nothing")
	}
}

func TestMorphFrameFillsTheTerminal(t *testing.T) {
	m, _ := mustUpdate(t, fixture(), tea.WindowSizeMsg{Width: 120, Height: 30})
	m, _ = mustUpdate(t, m, keyPress("x"))
	m, _ = mustUpdate(t, m, morphMsg{})
	v := m.View().Content
	if n := strings.Count(v, "\n") + 1; n != 30 {
		t.Errorf("morph frame is %d lines, want 30", n)
	}
	if !hasBraille(v) {
		t.Error("the morph frame drew nothing")
	}
	if strings.Contains(v, "UEFI") {
		t.Error("lettering leaves with the dismissal")
	}
}

func TestDownMovesTheCursor(t *testing.T) {
	m := tasks(fixture())
	next, _ := m.Update(typed('j'))
	if next.(Model).cursor != 1 {
		t.Errorf("cursor = %d, want 1", next.(Model).cursor)
	}
}

func TestCursorStopsAtTheEnd(t *testing.T) {
	m := tasks(fixture())
	m.cursor = 1
	next, _ := m.Update(typed('j'))
	if next.(Model).cursor != 1 {
		t.Errorf("cursor ran past the end: %d", next.(Model).cursor)
	}
}

func TestUpMovesTheCursorAndStopsAtTheTop(t *testing.T) {
	m := tasks(fixture())
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
	_, cmd := dismiss(fixture()).Update(typed('q'))
	if cmd == nil {
		t.Fatal("q returned no command; nothing would quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q did not return tea.Quit, got %T", cmd())
	}
}

func TestViewListsEveryAction(t *testing.T) {
	v := tasks(fixture()).View()
	for _, want := range []string{"switch", "build"} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("view missing %q:\n%s", want, v.Content)
		}
	}
}

// An empty fleet must not render as a blank alt screen with no way out.
func TestZeroRowsSaysSoAndSaysHowToLeave(t *testing.T) {
	m := dismiss(New(nil, theme.Default(), []string{"volinit: nothing found under /nowhere"}, hero.T1))
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
	m := tasks(New([]registry.Repo{{Name: "blog", Path: "/tmp/blog", Actions: []registry.Action{
		{Name: "deploy", Confirm: "Deploy live. Continue?"},
	}}}, theme.Default(), nil, hero.T1))

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
	m := tasks(New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "anon-run", ParamName: "CMD", ParamPrompt: "Command to jail"},
	}}}, theme.Default(), nil, hero.T1))

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
	m := tasks(New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "switch", Detach: true, Gate: boolp(false)},
	}}}, theme.Default(), nil, hero.T1))

	next, cmd := m.Update(enter)
	if cmd == nil {
		t.Fatal("detached action produced no command")
	}
	if got := next.(Model).status; !strings.Contains(got, "detached") {
		t.Errorf("status = %q, want it to report the detached start", got)
	}
}

func TestUnflaggedActionRunsImmediately(t *testing.T) {
	m := tasks(New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "build"},
	}}}, theme.Default(), nil, hero.T1))

	next, cmd := m.Update(enter)
	if cmd == nil {
		t.Fatal("unflagged action produced no command")
	}
	if next.(Model).mode != modeList {
		t.Errorf("mode = %v, want modeList", next.(Model).mode)
	}
}

func TestHeuristicGatedActionShowsConfirmPrompt(t *testing.T) {
	m := tasks(New([]registry.Repo{{Name: "nix-config", Path: "/tmp/nix", Actions: []registry.Action{
		{Name: "deploy"},
	}}}, theme.Default(), nil, hero.T1))

	next, cmd := m.Update(enter)
	m = next.(Model)
	if cmd != nil {
		t.Fatal("heuristic-gated action ran without confirmation")
	}
	if m.mode != modeConfirm {
		t.Fatalf("mode = %v, want modeConfirm", m.mode)
	}
	// The list row already names repo and action, so assert on the prompt itself.
	if got, want := m.footer(), "run deploy in nix-config? [y/N]"; got != want {
		t.Errorf("confirm prompt = %q, want %q", got, want)
	}
}

func TestGateFalseBypassesHeuristic(t *testing.T) {
	m := tasks(New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "deploy", Gate: boolp(false)},
	}}}, theme.Default(), nil, hero.T1))

	next, cmd := m.Update(enter)
	if cmd == nil {
		t.Fatal("Gate=false should bypass and run immediately")
	}
	if next.(Model).mode != modeList {
		t.Errorf("mode = %v, want modeList", next.(Model).mode)
	}
}

func TestGateTrueForcesConfirm(t *testing.T) {
	m := tasks(New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "build", Gate: boolp(true)},
	}}}, theme.Default(), nil, hero.T1))

	next, cmd := m.Update(enter)
	m = next.(Model)
	if cmd != nil {
		t.Fatal("Gate=true should not run without confirmation")
	}
	if m.mode != modeConfirm {
		t.Fatalf("mode = %v, want modeConfirm", m.mode)
	}
}

func TestQuitKeysLeaveStraightFromTheGreeting(t *testing.T) {
	for _, k := range []string{"q", "esc", "ctrl+c"} {
		next, cmd := fixture().Update(keyPress(k))
		if cmd == nil {
			t.Fatalf("%q at the greeting returned no command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%q at the greeting did not quit", k)
		}
		if !next.(Model).quit {
			t.Errorf("%q at the greeting did not mark the model quit", k)
		}
	}
}

func TestT0OpensAtTheSidebar(t *testing.T) {
	m := New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "build"},
	}}}, theme.Default(), nil, hero.T0)
	if m.stage != stageSidebar {
		t.Fatal("T0 prints State B directly: there is no greeting to dismiss")
	}
}

// wide returns a model already past the greeting: these tests are about
// scrolling the working list, not the dismiss.
func wide(n int) Model {
	var actions []registry.Action
	for i := 0; i < n; i++ {
		actions = append(actions, registry.Action{Name: fmt.Sprintf("a%02d", i)})
	}
	m := New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: actions}}, theme.Default(), nil, hero.T1)
	return tasks(m)
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

func hasBraille(s string) bool {
	for _, r := range s {
		if r > 0x2800 && r <= 0x28FF {
			return true
		}
	}
	return false
}

func TestGreetingDrawsTheAssembly(t *testing.T) {
	m, _ := mustUpdate(t, fixture(), tea.WindowSizeMsg{Width: 200, Height: 50})
	v := m.View().Content
	if !hasBraille(v) {
		t.Fatal("the greeting drew no art")
	}
	if !strings.Contains(v, "UEFI + Lanzaboote") {
		t.Error("a wide greeting letters the parts list")
	}
}

func TestSidebarPutsTheAssemblyBesideTheList(t *testing.T) {
	m, _ := mustUpdate(t, tasks(fixture()), tea.WindowSizeMsg{Width: 120, Height: 30})
	v := m.View().Content
	if !hasBraille(v) {
		t.Fatal("the sidebar lost the assembly")
	}
	for _, line := range strings.Split(v, "\n") {
		if i := strings.Index(line, "switch"); i >= 0 {
			if w := lipgloss.Width(line[:i]); w < sidebarCols+2 {
				t.Errorf("list starts at column %d, inside the %d-column strip", w, sidebarCols)
			}
			return
		}
	}
	t.Fatal("list row for switch not found")
}

func TestNarrowSidebarDropsTheStrip(t *testing.T) {
	m, _ := mustUpdate(t, dismiss(fixture()), tea.WindowSizeMsg{Width: minStripWidth - 1, Height: 30})
	if hasBraille(m.View().Content) {
		t.Error("a narrow terminal gives every column to the list")
	}
}

func TestT0SidebarHasNoArt(t *testing.T) {
	m := New([]registry.Repo{{Name: "cfg", Path: "/tmp/cfg", Actions: []registry.Action{
		{Name: "build"},
	}}}, theme.Default(), nil, hero.T0)
	if hasBraille(m.View().Content) {
		t.Error("T0 prints plain text: no art")
	}
}

func TestTopLevelIsTheThreeDoors(t *testing.T) {
	v := dismiss(menuFixture()).View().Content
	for _, want := range []string{"System", "2 subsystems", "Writing", "1 site", "Code", "1 project"} {
		if !strings.Contains(v, want) {
			t.Errorf("door level missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "switch") {
		t.Error("the door level must not list tasks")
	}
}

func TestEnterOpensDownToTheTasks(t *testing.T) {
	m := dismiss(menuFixture())
	m = press(t, m, enter)      // System
	m = press(t, m, typed('j')) // Secret Management
	m = press(t, m, enter)
	v := m.View().Content
	if !strings.Contains(v, "System › Secret Management") {
		t.Errorf("breadcrumb missing:\n%s", v)
	}
	if !strings.Contains(v, "sops-edit") || strings.Contains(v, "switch") {
		t.Errorf("wrong tasks listed:\n%s", v)
	}
}

func TestBackLandsWhereYouCameFrom(t *testing.T) {
	for _, k := range []string{"esc", "h", "left", "backspace"} {
		m := dismiss(menuFixture())
		m = press(t, m, typed('j')) // Writing
		m = press(t, m, enter)
		m = press(t, m, keyPress(k))
		if m.quit {
			t.Errorf("%q inside the menu quit instead of going back", k)
		}
		if len(m.path) != 0 || m.cursor != 1 {
			t.Errorf("%q: path %v cursor %d, want the door level on Writing", k, m.path, m.cursor)
		}
	}
}

func TestEscAtTheTopQuits(t *testing.T) {
	next, cmd := dismiss(menuFixture()).Update(keyPress("esc"))
	if cmd == nil || !next.(Model).quit {
		t.Fatal("esc at the door level must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("esc at the door level did not return tea.Quit")
	}
}

// l and right open doors and subsystems; only enter runs a target.
func TestLOpensButNeverRuns(t *testing.T) {
	m := dismiss(menuFixture())
	m = press(t, m, keyPress("l"))
	m = press(t, m, keyPress("right"))
	if !m.atTasks() {
		t.Fatal("l and right must open doors and subsystems")
	}
	for _, k := range []string{"l", "right"} {
		next, cmd := m.Update(keyPress(k))
		if cmd != nil || next.(Model).mode != modeList {
			t.Fatalf("%q at the task level acted on a target", k)
		}
	}
}

// Models are values and path is a slice: back() leaves spare capacity, so an
// open() that appended in place would rewrite the older model's path.
func TestOpeningDoesNotRewriteAnEarlierModel(t *testing.T) {
	deep := press(t, press(t, press(t, dismiss(menuFixture()), enter), typed('j')), enter)
	up := press(t, deep, keyPress("esc")) // back to System, on Secret Management
	press(t, press(t, up, typed('k')), enter)
	if got := deep.crumb(); got != "System › Secret Management" {
		t.Fatalf("an older model's path was rewritten: %q", got)
	}
}
