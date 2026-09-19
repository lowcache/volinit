# volinit Menu Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the 101-row flat list with a three-level menu: door → subsystem → task.

**Architecture:** `registry.Menu` shapes discovered repos into plain data (doors holding groups holding actions). The runtime walks it with a `path` of chosen indices and keeps `rows` as the entries of the current level, so the param gate, confirm gate and run paths — which index `m.rows[m.cursor]` — are untouched.

**Tech Stack:** Go 1.26.2 (nix-provided), `charm.land/bubbletea/v2` v2.0.9, `charm.land/lipgloss/v2` v2.0.6, vendored.

**Spec:** `docs/superpowers/specs/2026-09-18-volinit-cockpit-design.md` — the "Addendum — the menu".

## Global Constraints

- Branch `rewrite/go-cockpit`. `main` stays Nim — it is a live flake input.
- Deps vendored; add NO dependencies. Do not run `go get` / `go mod tidy` or touch `go.mod`, `go.sum`, `vendor/`.
- Stage by explicit path only. Never `git add -A`, `.`, or `-a`.
- **Only `enter` runs a target**, and only at the task level. `l` / `→` open doors and subsystems and do nothing at the task level.
- The param and confirm gates are unchanged: `gate()`, `start()`, `cancel()` and the `modeParam` / `modeConfirm` branches of `key()` must not be edited.
- The greeting, morph and one-way door are unchanged.
- `internal/registry/sidecar.go` and `internal/registry/makefile_test.go` already fail `gofmt -l`; do not reformat them. New and touched files must be gofmt-clean.
- Tests may invoke a returned `tea.Cmd` only when it is `tea.Quit` or the morph tick.

---

### Task 1: `registry.Menu`

**Files:**
- Create: `internal/registry/menu.go`
- Create: `internal/registry/menu_test.go`

**Interfaces:**
- Consumes: `Repo`, `Action`, `BranchSystem`, `BranchWriting`, `BranchCode`, `Discover` (existing)
- Produces:
  - `type Door struct { Name string; Noun string; Groups []Group }`
  - `type Group struct { Name string; Repo string; Path string; Actions []Action }`
  - `func Menu(repos []Repo) []Door`

- [ ] **Step 1: Write the failing tests** — `internal/registry/menu_test.go`

```go
package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func doorNames(ds []Door) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Name)
	}
	return out
}

func TestMenuOrdersDoorsAndSkipsEmptyOnes(t *testing.T) {
	got := Menu([]Repo{
		{Name: "app", Branch: BranchCode, Actions: []Action{{Name: "build"}}},
		{Name: "cfg", Branch: BranchSystem, Actions: []Action{{Name: "switch"}}},
	})
	if len(got) != 2 || got[0].Name != "System" || got[1].Name != "Code" {
		t.Fatalf("doors = %v, want [System Code]", doorNames(got))
	}
	if got[0].Noun != "subsystems" || got[1].Noun != "projects" {
		t.Errorf("nouns = %q, %q", got[0].Noun, got[1].Noun)
	}
}

func TestSystemRepoSplitsBySection(t *testing.T) {
	gs := Menu([]Repo{{Name: "cfg", Path: "/c", Branch: BranchSystem, Actions: []Action{
		{Name: "switch", Section: "System Operations"},
		{Name: "sops-edit", Section: "Secret Management"},
		{Name: "build", Section: "System Operations"},
		{Name: "loose"},
	}}})[0].Groups
	want := []struct {
		name string
		n    int
	}{{"System Operations", 2}, {"Secret Management", 1}, {"cfg", 1}}
	if len(gs) != len(want) {
		t.Fatalf("got %d groups, want %d", len(gs), len(want))
	}
	for i, w := range want {
		if gs[i].Name != w.name || len(gs[i].Actions) != w.n {
			t.Errorf("group %d = %q with %d actions, want %q with %d", i, gs[i].Name, len(gs[i].Actions), w.name, w.n)
		}
		if gs[i].Repo != "cfg" || gs[i].Path != "/c" {
			t.Errorf("group %q lost its repo: %q %q", gs[i].Name, gs[i].Repo, gs[i].Path)
		}
	}
}

func TestOtherDoorsGroupByRepoInNameOrder(t *testing.T) {
	gs := Menu([]Repo{
		{Name: "wiki", Branch: BranchWriting, Actions: []Action{{Name: "deploy"}}},
		{Name: "blog", Branch: BranchWriting, Actions: []Action{{Name: "serve", Section: "Dev"}, {Name: "deploy"}}},
	})[0].Groups
	if len(gs) != 2 || gs[0].Name != "blog" || gs[1].Name != "wiki" {
		t.Fatalf("writing groups wrong: %+v", gs)
	}
	if len(gs[0].Actions) != 2 {
		t.Error("outside the system door a repo is one subsystem, sections or not")
	}
}

func TestHelpIsDropped(t *testing.T) {
	got := Menu([]Repo{
		{Name: "site", Branch: BranchWriting, Actions: []Action{{Name: "help"}, {Name: "build"}}},
		{Name: "bare", Branch: BranchCode, Actions: []Action{{Name: "help"}}},
	})
	if len(got) != 1 {
		t.Fatalf("a door holding only help targets must vanish, got %v", doorNames(got))
	}
	if as := got[0].Groups[0].Actions; len(as) != 1 || as[0].Name != "build" {
		t.Errorf("help survived: %+v", as)
	}
}

func TestUnclassifiedRepoIsCode(t *testing.T) {
	got := Menu([]Repo{{Name: "x", Actions: []Action{{Name: "build"}}}})
	if len(got) != 1 || got[0].Name != "Code" {
		t.Fatalf("doors = %v, want [Code]", doorNames(got))
	}
}

// The menu may regroup the fleet but must never lose a runnable target.
func TestMenuKeepsEveryRealTargetButHelp(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	roots := []string{filepath.Join(home, ".nix-config"), filepath.Join(home, "CodeRepo")}
	for _, r := range roots {
		if _, err := os.Stat(r); err != nil {
			t.Skip("fleet not present on this machine")
		}
	}
	repos := Discover(roots)
	want := 0
	for _, r := range repos {
		for _, a := range r.Actions {
			if a.Name != "help" {
				want++
			}
		}
	}
	got := 0
	for _, d := range Menu(repos) {
		for _, g := range d.Groups {
			got += len(g.Actions)
		}
	}
	if got != want {
		t.Fatalf("menu holds %d targets, fleet has %d runnable", got, want)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/registry/`
Expected: FAIL — `undefined: Menu`.

- [ ] **Step 3: Implement** — `internal/registry/menu.go`

```go
package registry

import "sort"

// Door is one top-level branch of the cockpit's menu.
type Door struct {
	Name   string // System, Writing, Code
	Noun   string // what its subsystems are called, plural
	Groups []Group
}

// Group is one subsystem: a Makefile section of a system repo, or a whole
// repo behind the other doors.
type Group struct {
	Name    string
	Repo    string
	Path    string
	Actions []Action
}

var doors = []struct{ branch, name, noun string }{
	{BranchSystem, "System", "subsystems"},
	{BranchWriting, "Writing", "sites"},
	{BranchCode, "Code", "projects"},
}

// Menu arranges repos into doors → subsystems → tasks. Doors come in fixed
// order and are left out when empty.
func Menu(repos []Repo) []Door {
	var out []Door
	for _, d := range doors {
		var in []Repo
		for _, r := range repos {
			if branchOf(r) == d.branch {
				in = append(in, r)
			}
		}
		sort.SliceStable(in, func(i, j int) bool { return in[i].Name < in[j].Name })
		door := Door{Name: d.name, Noun: d.noun}
		for _, r := range in {
			if d.branch == BranchSystem {
				door.Groups = append(door.Groups, sections(r)...)
			} else if acts := runnable(r.Actions); len(acts) > 0 {
				door.Groups = append(door.Groups, Group{Name: r.Name, Repo: r.Name, Path: r.Path, Actions: acts})
			}
		}
		if len(door.Groups) > 0 {
			out = append(out, door)
		}
	}
	return out
}

// branchOf files anything not classified system or writing under code,
// which is how the spec defines the code door.
func branchOf(r Repo) string {
	if r.Branch == BranchSystem || r.Branch == BranchWriting {
		return r.Branch
	}
	return BranchCode
}

// sections splits a system repo by Makefile section in order of first
// appearance; unsectioned targets group under the repo's own name.
func sections(r Repo) []Group {
	var out []Group
	at := map[string]int{}
	for _, a := range runnable(r.Actions) {
		name := a.Section
		if name == "" {
			name = r.Name
		}
		i, ok := at[name]
		if !ok {
			i = len(out)
			at[name] = i
			out = append(out, Group{Name: name, Repo: r.Name, Path: r.Path})
		}
		out[i].Actions = append(out[i].Actions, a)
	}
	return out
}

// runnable drops `help`: it prints the Makefile's own target list, which
// the menu replaces.
func runnable(as []Action) []Action {
	var out []Action
	for _, a := range as {
		if a.Name != "help" {
			out = append(out, a)
		}
	}
	return out
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/registry/ -v -run 'Menu|Section|Doors|Help|Unclassified' && go vet ./internal/registry/ && gofmt -l internal/registry/menu.go internal/registry/menu_test.go`
Expected: PASS (6 tests; the real-fleet one skips where the fleet is absent); gofmt prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/registry/menu.go internal/registry/menu_test.go
git commit -m "feat(registry): shape the fleet into doors, subsystems and tasks"
```

---

### Task 2: The runtime walks the menu

**Files:**
- Modify: `internal/runtime/model.go`
- Modify: `internal/runtime/model_test.go`

**Interfaces:**
- Consumes: `registry.Menu`, `registry.Door`, `registry.Group` (Task 1)
- Produces: `Model.menu`, `Model.path`; methods `load`, `open`, `back`, `atTasks`, `crumb`; func `count`

- [ ] **Step 1: Update the test helpers in `model_test.go`**

Add two cases to the `switch` in `keyPress`, before `default`:

```go
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
```

Add after `dismiss`:

```go
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
```

- [ ] **Step 2: Move the task-level tests to the task level**

These tests exercise a task list and must start there. In each, replace
the call `dismiss(` with `tasks(` — nothing else in them changes:

`TestDownMovesTheCursor`, `TestCursorStopsAtTheEnd`,
`TestUpMovesTheCursorAndStopsAtTheTop`, `TestViewListsEveryAction`,
`TestEnterConfirmsBeforeRunning`, `TestEnterCollectsParamBeforeRunning`,
`TestDetachedActionStartsWithoutSuspending`,
`TestUnflaggedActionRunsImmediately`,
`TestHeuristicGatedActionShowsConfirmPrompt`,
`TestGateFalseBypassesHeuristic`, `TestGateTrueForcesConfirm`,
`TestSidebarPutsTheAssemblyBesideTheList`, and the helper `wide`
(its `return dismiss(m)` becomes `return tasks(m)`).

Leave every other test as it is.

- [ ] **Step 3: Add the new tests**

```go
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
```

- [ ] **Step 4: Run and confirm failure**

Run: `go test ./internal/runtime/`
Expected: FAIL — `m.atTasks undefined` (and `crumb`).

- [ ] **Step 5: Implement in `model.go`**

Replace the `row` type:

```go
// row is one entry at the current menu level. repo, path and action are
// set only at the task level.
type row struct {
	label  string
	detail string
	repo   string
	path   string
	action registry.Action
}
```

Replace the `Model` doc comment's first two lines
(`// Model is the cockpit state. Flat list for now; the tree lands with the`
/ `// visual design.`) with:

```go
// Model is the cockpit state: a three-level menu (doors, subsystems, tasks)
// walked by path, with rows holding the current level's entries.
```

Add two fields to `Model`, directly above `rows`:

```go
	menu    []registry.Door
	path    []int // entry chosen at each level above the current one
```

Replace `New`:

```go
func New(repos []registry.Repo, p theme.Palette, notices []string, tier hero.Tier) Model {
	m := Model{menu: registry.Menu(repos), palette: p, notices: notices, width: defaultWidth, height: defaultHeight, stage: stageFullBleed, tier: tier}
	m.load()
	if tier == hero.T0 {
		m.stage = stageSidebar // T0 prints State B directly, with no transition
	}
	return m
}
```

Add after `New`:

```go
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
```

In `key`, replace ONLY the final list-mode `switch k.String() { ... }`
block (the one after the `switch m.mode` block, starting at
`case "q", "esc", "ctrl+c":`) with:

```go
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
```

Replace `visible`:

```go
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
```

In `viewSidebar`, directly after the `for _, n := range m.notices { ... }`
loop, add:

```go
	if c := m.crumb(); c != "" {
		b.WriteString(fg.Render(c))
		b.WriteString("\n")
	}
```

and replace the row format line
`line := fmt.Sprintf("  %-14s %-18s %s", r.repo, r.action.Name, r.action.Description)`
with:

```go
			line := fmt.Sprintf("  %-26s %s", r.label, r.detail)
```

In `footer`, replace `keys := "j/k move · enter runs · q quits"` with:

```go
	keys := "j/k move · enter opens · q quits"
	switch {
	case m.atTasks():
		keys = "j/k move · enter runs · esc back · q quits"
	case len(m.path) > 0:
		keys = "j/k move · enter opens · esc back · q quits"
	}
```

- [ ] **Step 6: Run tests**

Run: `go test ./... && go vet ./... && gofmt -l internal/runtime/`
Expected: PASS; gofmt prints nothing. A failure in any test not named in
Steps 2–3 is a real regression — report it; do not edit that test to pass.

- [ ] **Step 7: Commit**

```bash
git add internal/runtime/model.go internal/runtime/model_test.go
git commit -m "feat(runtime): walk the menu — doors, subsystems, tasks"
```

---

## After Task 2

1. `nix flake check` exits 0; `nix build .#default` for the operator's look.
2. Operator check at a real terminal: the three doors, drilling in, `esc` back.
