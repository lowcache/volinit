# volinit Foundation Plan 1b — Fleet, Sidecar, Execution, Runtime

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the parsers from Plan 1 into a working cockpit — fleet discovery, the sidecar, safe execution, and a minimal Bubble Tea runtime — stopping at the operator's graphics checkpoint.

**Architecture:** `registry` gains discovery and the sidecar; a new `internal/run` owns execution; `internal/runtime` is the Bubble Tea program. Rendering stays deliberately plain: no animation, no graphics, no hero.

**Tech Stack:** Go 1.26.7, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `github.com/BurntSushi/toml`, vendored.

**Spec:** `docs/superpowers/specs/2026-09-18-volinit-cockpit-design.md`
**Predecessor:** `docs/superpowers/plans/2026-09-18-volinit-foundation.md` (Tasks 1–5)

## Global Constraints

- Branch `rewrite/go-cockpit`. `main` keeps building Nim — it is a live flake input at `~/.nix-config/home/pkgs.nix:173`.
- Deps vendored; `vendorHash = null`; flake-check sandbox has no network.
- **Discovery never executes a Makefile.**
- Bubble Tea v2 API facts, verified via `go doc` against downloaded sources:
  - `Init() (tea.Model, tea.Cmd)` — returns a Model, unlike v1
  - `Update(tea.Msg) (tea.Model, tea.Cmd)`
  - `View() tea.View`, built with `tea.NewView(string)`
  - **Alt screen is `view.AltScreen = true`, NOT a ProgramOption**
  - `tea.ExecProcess(c *exec.Cmd, fn ExecCallback) Cmd`; `ExecCallback func(error) Msg`
  - Terminal restore on panic/SIGINT is automatic — do not add your own
- Lipgloss v2 is `charm.land/lipgloss/v2`; colors via `lipgloss.Color("#rrggbb")`.
- **No visual design decisions in this plan.** Plain borders and palette roles only.

---

### Task 6: Fleet discovery and the three-branch tree

**Files:**
- Create: `internal/registry/discover.go`, `internal/registry/discover_test.go`

**Interfaces:**
- Consumes: `Action`, `ParseHashHash`, `ParseHelpTarget` (Tasks 4–5)
- Produces: `type Repo struct { Name, Path, Branch string; Actions []Action }`; `func Discover(roots []string) []Repo`; branch constants `BranchSystem`, `BranchWriting`, `BranchCode`

- [ ] **Step 1: Write the failing test**

```go
package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMakefile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverFindsBothDialects(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "sys"),
		"## System Operations\n## :switch: ....: Rebuild\nswitch:\n\techo hi\n")
	writeMakefile(t, filepath.Join(root, "sites", "blogs", "wiki"),
		"help:\n\t@echo \"make serve   Live preview\"\n")

	repos := Discover([]string{root})
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(repos))
	}
	byName := map[string]Repo{}
	for _, r := range repos {
		byName[r.Name] = r
	}
	if got := byName["sys"].Actions; len(got) != 1 || got[0].Name != "switch" {
		t.Errorf("sys actions = %+v", got)
	}
	if got := byName["wiki"].Actions; len(got) != 1 || got[0].Name != "serve" {
		t.Errorf("wiki actions = %+v", got)
	}
}

func TestDiscoverClassifiesBranches(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "sites", "blogs", "wiki"),
		"help:\n\t@echo \"make serve   x\"\n")
	writeMakefile(t, filepath.Join(root, "tether"),
		"help:\n\t@echo \"make build   x\"\n")

	byName := map[string]Repo{}
	for _, r := range Discover([]string{root}) {
		byName[r.Name] = r
	}
	if byName["wiki"].Branch != BranchWriting {
		t.Errorf("wiki branch = %q, want %q", byName["wiki"].Branch, BranchWriting)
	}
	if byName["tether"].Branch != BranchCode {
		t.Errorf("tether branch = %q, want %q", byName["tether"].Branch, BranchCode)
	}
}

func TestDiscoverSkipsRepoWithNoActions(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "bare"), "all:\n\techo hi\n")
	if repos := Discover([]string{root}); len(repos) != 0 {
		t.Fatalf("got %d repos, want 0 — a Makefile with no documented targets is not worth a menu entry", len(repos))
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/registry/ -run TestDiscover -v`
Expected: FAIL — `undefined: Discover`

- [ ] **Step 3: Implement**

```go
package registry

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Branch names the three top-level groupings the cockpit presents.
const (
	BranchSystem  = "system"
	BranchWriting = "writing"
	BranchCode    = "code"
)

// Repo is one discovered repository and the actions it exposes.
type Repo struct {
	Name    string
	Path    string
	Branch  string
	Actions []Action
}

// maxDepth bounds the walk. The blogs live at sites/blogs/<name>, three
// levels below ~/CodeRepo, so anything shallower misses them.
const maxDepth = 4

// Discover walks each root for Makefiles and parses whichever dialect each
// uses. Nothing is executed: both parsers are static readers.
func Discover(roots []string) []Repo {
	var out []Repo
	for _, root := range roots {
		base := strings.Count(filepath.Clean(root), string(os.PathSeparator))
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable dirs are skipped, never fatal
			}
			if d.IsDir() {
				if strings.Count(path, string(os.PathSeparator))-base >= maxDepth {
					return fs.SkipDir
				}
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "vendor" {
					return fs.SkipDir
				}
				return nil
			}
			if d.Name() != "Makefile" {
				return nil
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			actions := ParseHashHash(string(src))
			if len(actions) == 0 {
				actions = ParseHelpTarget(string(src))
			}
			if len(actions) == 0 {
				return nil
			}
			dir := filepath.Dir(path)
			out = append(out, Repo{
				Name:    filepath.Base(dir),
				Path:    dir,
				Branch:  classify(dir),
				Actions: actions,
			})
			return nil
		})
	}
	return out
}

// classify buckets a repo by path. nix-config is the system; anything under
// a blogs/ or sites/ path is writing; everything else is code.
func classify(dir string) string {
	switch {
	case strings.Contains(dir, ".nix-config"):
		return BranchSystem
	case strings.Contains(dir, "/blogs/"), strings.Contains(dir, "/sites/"):
		return BranchWriting
	default:
		return BranchCode
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/registry/ -v`
Expected: PASS (7 tests total across the package)

- [ ] **Step 5: Verify against the real fleet**

```bash
cat > /tmp/volinit_fleet.go <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/lowcache/volinit/internal/registry"
)

func main() {
	home, _ := os.UserHomeDir()
	for _, r := range registry.Discover([]string{home + "/.nix-config", home + "/CodeRepo"}) {
		fmt.Printf("%-8s %-20s %d actions\n", r.Branch, r.Name, len(r.Actions))
	}
}
EOF
go run /tmp/volinit_fleet.go
```

Expected: `.nix-config` with 45 actions under `system`; `wiki`, `hotelevangelism`, `volnixos-blog` under `writing`; `seeksascha` under `writing` (it lives in sites/).

- [ ] **Step 6: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): walk the fleet and classify repos into three branches"
```

---

### Task 7: Sidecar and doctor

**Files:**
- Create: `internal/registry/sidecar.go`, `internal/registry/sidecar_test.go`

**Interfaces:**
- Consumes: `Action`, `Repo` (Task 6)
- Produces: fields on `Action` — `Confirm string`, `Sudo bool`, `Detach bool`, `Stream bool`, `ParamName string`, `ParamPrompt string`, `When string`; `func ApplySidecar(r *Repo) error`; `func Doctor(repos []Repo) []string`

- [ ] **Step 1: Extend the Action struct**

In `internal/registry/makefile.go`, replace the `Action` definition with:

```go
// Action is one runnable entry. The fields below Section come from the
// optional sidecar; zero values mean "run it plainly".
type Action struct {
	Name        string
	Description string
	Section     string

	Confirm     string // non-empty => prompt with this text before running
	Sudo        bool   // needs a real TTY for a password prompt
	Detach      bool   // run via systemd-run --user, follow the journal
	Stream      bool   // long-lived output, render in a pager view
	ParamName   string // env var name to pass, e.g. CMD
	ParamPrompt string // prompt shown when collecting the param
	When        string // visibility expression, e.g. host == nix-on-droid
}
```

- [ ] **Step 2: Write the failing test**

```go
package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplySidecarAnnotatesMatchingActions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".volinit"), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `
[switch]
detach = true

[deploy]
confirm = "Rescue-path deploy. Continue?"

[anon-run]
param = { name = "CMD", prompt = "Command to jail" }
`
	if err := os.WriteFile(filepath.Join(dir, ".volinit", "actions.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	r := Repo{Path: dir, Actions: []Action{
		{Name: "switch"}, {Name: "deploy"}, {Name: "anon-run"}, {Name: "build"},
	}}
	if err := ApplySidecar(&r); err != nil {
		t.Fatalf("ApplySidecar: %v", err)
	}
	if !r.Actions[0].Detach {
		t.Error("switch should be detached")
	}
	if r.Actions[1].Confirm == "" {
		t.Error("deploy should carry a confirm")
	}
	if r.Actions[2].ParamName != "CMD" {
		t.Errorf("ParamName = %q, want CMD", r.Actions[2].ParamName)
	}
	if r.Actions[3].Detach || r.Actions[3].Confirm != "" {
		t.Error("build has no entry and must stay plain")
	}
}

func TestApplySidecarMissingFileIsNotAnError(t *testing.T) {
	r := Repo{Path: t.TempDir(), Actions: []Action{{Name: "build"}}}
	if err := ApplySidecar(&r); err != nil {
		t.Fatalf("missing sidecar must not error, got %v", err)
	}
}

func TestDoctorFlagsDangerousActionsWithoutConfirm(t *testing.T) {
	repos := []Repo{{Name: "cfg", Actions: []Action{
		{Name: "switch"},
		{Name: "deploy", Confirm: "ok?"},
		{Name: "build"},
	}}}
	warns := Doctor(repos)
	if len(warns) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warns), warns)
	}
	if !strings.Contains(warns[0], "switch") {
		t.Errorf("warning should name switch, got %q", warns[0])
	}
}
```

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/registry/ -run "Sidecar|Doctor" -v`
Expected: FAIL — `undefined: ApplySidecar`

- [ ] **Step 4: Implement**

```go
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// sidecarEntry mirrors one table in .volinit/actions.toml.
type sidecarEntry struct {
	Confirm string `toml:"confirm"`
	Sudo    bool   `toml:"sudo"`
	Detach  bool   `toml:"detach"`
	Stream  bool   `toml:"stream"`
	When    string `toml:"when"`
	Param   struct {
		Name   string `toml:"name"`
		Prompt string `toml:"prompt"`
	} `toml:"param"`
}

// ApplySidecar overlays <repo>/.volinit/actions.toml onto already-discovered
// actions. Absent or unreadable, everything still runs plainly — the sidecar
// is never required.
func ApplySidecar(r *Repo) error {
	path := filepath.Join(r.Path, ".volinit", "actions.toml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var entries map[string]sidecarEntry
	if _, err := toml.Decode(string(raw), &entries); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for i := range r.Actions {
		e, ok := entries[r.Actions[i].Name]
		if !ok {
			continue
		}
		r.Actions[i].Confirm = e.Confirm
		r.Actions[i].Sudo = e.Sudo
		r.Actions[i].Detach = e.Detach
		r.Actions[i].Stream = e.Stream
		r.Actions[i].When = e.When
		r.Actions[i].ParamName = e.Param.Name
		r.Actions[i].ParamPrompt = e.Param.Prompt
	}
	return nil
}

// dangerous names targets that change system or published state. Make
// targets are not a stable API and the sidecar rots; this is the cheap
// check that catches a destructive target sitting one keypress deep.
var dangerous = []string{"switch", "deploy", "rekey", "force", "arm", "disarm", "clean", "rm"}

// Doctor reports actions that look destructive but carry no confirmation.
func Doctor(repos []Repo) []string {
	var warns []string
	for _, r := range repos {
		for _, a := range r.Actions {
			if a.Confirm != "" {
				continue
			}
			for _, d := range dangerous {
				if strings.Contains(a.Name, d) {
					warns = append(warns, fmt.Sprintf(
						"%s: %q looks destructive but has no confirm in .volinit/actions.toml", r.Name, a.Name))
					break
				}
			}
		}
	}
	return warns
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/registry/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): sidecar annotations and a doctor for ungated destructive targets"
```

---

### Task 8: Execution

**Files:**
- Create: `internal/run/run.go`, `internal/run/run_test.go`

**Interfaces:**
- Consumes: `registry.Action`
- Produces: `func Command(a registry.Action, repoPath, param string) *exec.Cmd`; `func DetachedCommand(a registry.Action, repoPath string) *exec.Cmd`; `type DoneMsg struct { Action string; Err error }`

- [ ] **Step 1: Write the failing test**

```go
package run

import (
	"strings"
	"testing"

	"github.com/lowcache/volinit/internal/registry"
)

func TestCommandRunsMakeInRepo(t *testing.T) {
	c := Command(registry.Action{Name: "build"}, "/tmp/repo", "")
	if c.Dir != "/tmp/repo" {
		t.Errorf("Dir = %q", c.Dir)
	}
	got := strings.Join(c.Args, " ")
	if !strings.Contains(got, "make") || !strings.Contains(got, "build") {
		t.Errorf("args = %q", got)
	}
}

func TestCommandPassesParamAsEnv(t *testing.T) {
	a := registry.Action{Name: "anon-run", ParamName: "CMD"}
	c := Command(a, "/tmp/repo", "curl example.com")
	var found bool
	for _, e := range c.Env {
		if e == "CMD=curl example.com" {
			found = true
		}
	}
	if !found {
		t.Errorf("CMD not in env: %v", c.Env)
	}
}

func TestDetachedCommandUsesSystemdRun(t *testing.T) {
	c := DetachedCommand(registry.Action{Name: "switch"}, "/tmp/repo")
	got := strings.Join(c.Args, " ")
	for _, want := range []string{"systemd-run", "--user", "--unit=volinit-switch", "make", "switch"} {
		if !strings.Contains(got, want) {
			t.Errorf("args %q missing %q", got, want)
		}
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/run/ -v`
Expected: FAIL — `undefined: Command`

- [ ] **Step 3: Implement**

```go
// Package run turns a registry.Action into a command. It builds commands but
// never starts them: the caller decides whether to hand one to
// tea.ExecProcess or to start it detached.
package run

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/lowcache/volinit/internal/registry"
)

// DoneMsg is delivered to the Bubble Tea model when an action finishes.
type DoneMsg struct {
	Action string
	Err    error
}

// Command builds a foreground `make <target>` for the repo. The caller runs
// it through tea.ExecProcess, which releases the terminal so sudo can prompt
// on a real TTY.
func Command(a registry.Action, repoPath, param string) *exec.Cmd {
	c := exec.Command("make", a.Name)
	c.Dir = repoPath
	c.Env = os.Environ()
	if a.ParamName != "" && param != "" {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", a.ParamName, param))
	}
	return c
}

// DetachedCommand runs the target as a transient user unit so it survives the
// cockpit exiting. `make switch` ran 4h10m on 2026-09-18; a TUI owning that
// process would reintroduce the failure `switch-detached` exists to prevent.
func DetachedCommand(a registry.Action, repoPath string) *exec.Cmd {
	c := exec.Command("systemd-run",
		"--user", "--collect",
		fmt.Sprintf("--unit=volinit-%s", a.Name),
		fmt.Sprintf("--working-directory=%s", repoPath),
		"make", a.Name,
	)
	c.Dir = repoPath
	c.Env = os.Environ()
	return c
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/run/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/run/
git commit -m "feat(run): foreground and detached command construction"
```

---

### Task 9: Minimal runtime and the startup budget

Deliberately plain. No animation, no graphics, no layout ambition — those
are gated on the operator checkpoint that follows this task.

**Files:**
- Create: `internal/runtime/model.go`, `internal/runtime/model_test.go`
- Modify: `main.go`
- Create: `internal/registry/bench_test.go`

**Interfaces:**
- Consumes: `registry.Repo`, `theme.Palette`, `run.Command`
- Produces: `func New(repos []registry.Repo, p theme.Palette) Model`; `func Run(repos []registry.Repo, p theme.Palette) error`

- [ ] **Step 1: Add the deps and vendor**

```bash
go get charm.land/bubbletea/v2 charm.land/lipgloss/v2
go mod vendor
```

- [ ] **Step 2: Write the failing test**

```go
package runtime

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/theme"
)

func fixture() Model {
	return New([]registry.Repo{
		{Name: "cfg", Branch: registry.BranchSystem, Actions: []registry.Action{
			{Name: "switch"}, {Name: "build"},
		}},
	}, theme.Default())
}

func TestDownMovesTheCursor(t *testing.T) {
	m := fixture()
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if next.(Model).cursor != 1 {
		t.Errorf("cursor = %d, want 1", next.(Model).cursor)
	}
}

func TestCursorStopsAtTheEnd(t *testing.T) {
	m := fixture()
	m.cursor = 1
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if next.(Model).cursor != 1 {
		t.Errorf("cursor ran past the end: %d", next.(Model).cursor)
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
```

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/runtime/ -v`
Expected: FAIL — `undefined: New`

- [ ] **Step 4: Implement**

```go
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

func (m Model) Init() (tea.Model, tea.Cmd) { return m, nil }

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
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/runtime/ -v`
Expected: PASS (3 tests)

- [ ] **Step 6: Wire main.go**

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/runtime"
	"github.com/lowcache/volinit/internal/theme"
)

func main() {
	home, _ := os.UserHomeDir()
	roots := []string{filepath.Join(home, ".nix-config"), filepath.Join(home, "CodeRepo")}

	repos := registry.Discover(roots)
	for i := range repos {
		if err := registry.ApplySidecar(&repos[i]); err != nil {
			fmt.Fprintln(os.Stderr, "volinit:", err)
		}
	}
	p, _ := theme.Load(filepath.Join(home, ".config", "volinit", "palette.toml"))

	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		for _, w := range registry.Doctor(repos) {
			fmt.Println(w)
		}
		return
	}
	if err := runtime.Run(repos, p); err != nil {
		fmt.Fprintln(os.Stderr, "volinit:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: The startup budget benchmark**

Create `internal/registry/bench_test.go`:

```go
package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The discovery walk runs on every interactive shell. This fails the build
// if it drifts past the budget, which is what stops latency creeping in one
// feature at a time.
func TestDiscoveryStaysWithinBudget(t *testing.T) {
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
	const budget = 20 * time.Millisecond
	start := time.Now()
	Discover(roots)
	if elapsed := time.Since(start); elapsed > budget {
		t.Fatalf("discovery took %v, budget is %v", elapsed, budget)
	}
}
```

- [ ] **Step 8: Run everything**

Run: `go test ./... && nix build .#default && ./result/bin/volinit doctor`
Expected: all tests pass; `doctor` prints warnings for ungated destructive targets across the real fleet.

- [ ] **Step 9: Commit**

```bash
git add internal/runtime/ internal/registry/bench_test.go main.go go.mod go.sum vendor/
git commit -m "feat(runtime): minimal cockpit, doctor subcommand, startup budget test"
```


---

### Task 10: The noctalia template (closes a spec gap)

Task 3 reads `~/.config/volinit/palette.toml`, but nothing writes it. This
ships the template that does. Simpler than `starship-m3`: volinit reads its
own file, so there is no `post_hook` and no `apply.sh` to splice a block
into someone else's config.

**Files:**
- Create: `contrib/noctalia/volinit-m3/template.toml`
- Create: `contrib/noctalia/volinit-m3/volinit-m3.toml`
- Modify: `README.md` (installation note)

**Interfaces:**
- Consumes: the role names `theme.Palette` unmarshals (Task 3)
- Produces: `$XDG_CONFIG_HOME/volinit/palette.toml` whenever noctalia applies a scheme

- [ ] **Step 1: Verify the neutral-ramp variable names**

The role variables are `{{colors.<role>.default.hex}}`. Confirm how the
18-step ramp is addressed before writing the template:

```bash
grep -n "neutral" ~/.config/noctalia/templates/starship-m3/starship-m3.toml
```

Use exactly the spelling that appears there. Do not guess.

- [ ] **Step 2: Write the manifest**

`contrib/noctalia/volinit-m3/template.toml`:

```toml
# volinit reads this palette directly, so unlike starship-m3 there is no
# post_hook: the generated file IS the consumed file.

[catalog.volinit-m3]
name = "volinit (M3 roles)"
category = "terminal"

[templates.volinit-m3]
input_path = "volinit-m3.toml"
output_path = "$XDG_CONFIG_HOME/volinit/palette.toml"
```

- [ ] **Step 3: Write the template**

`contrib/noctalia/volinit-m3/volinit-m3.toml` — flat keys, matching the
`toml` tags on `theme.Palette`:

```toml
# Generated by Noctalia from Material 3 roles - do not edit manually
primary            = "{{colors.primary.default.hex}}"
on_primary         = "{{colors.on_primary.default.hex}}"
surface            = "{{colors.surface.default.hex}}"
on_surface         = "{{colors.on_surface.default.hex}}"
surface_variant    = "{{colors.surface_variant.default.hex}}"
surface_container  = "{{colors.surface_container.default.hex}}"
outline            = "{{colors.outline.default.hex}}"
error              = "{{colors.error.default.hex}}"
```

Then append `neutral_0` through `neutral_17` using the spelling confirmed in
Step 1.

- [ ] **Step 4: Install and apply**

```bash
cp -r contrib/noctalia/volinit-m3 ~/.config/noctalia/templates/
```

Enable it the way noctalia enables a template, then change the wallpaper or
re-apply the scheme to trigger generation.

- [ ] **Step 5: Verify it generated and parses**

```bash
cat "${XDG_CONFIG_HOME:-$HOME/.config}/volinit/palette.toml"
go test ./internal/theme/ -v
./result/bin/volinit
```

Expected: the file exists with real hex values (no `{{` left unsubstituted),
theme tests pass, and the cockpit renders in wallpaper colours rather than
the compiled-in defaults.

- [ ] **Step 6: Commit**

```bash
git add contrib/noctalia/ README.md
git commit -m "feat(theme): ship the volinit-m3 noctalia template"
```

---

## CHECKPOINT — stop here

Everything above is deliberately plain: a working, tested cockpit that lists
the fleet and can run targets safely.

**Do not begin hero, animation, layout, or graphics work.** The operator has
asked to review and approve the visual direction before any of it is built.
Present the working plain cockpit, then get sign-off on:

1. the settled-frame layout
2. the animation concept
3. whether T3 (Limoni 3D) is attempted at all

Plan 2 covers the hero and is written after that conversation.
