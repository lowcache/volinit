# volinit Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace volinit's Nim banner with a Go foundation — buildable flake, crash-safe terminal handling, noctalia theming, and a Makefile fleet registry — stopping before any visual work.

**Architecture:** Four packages (`registry`, `theme`, `runtime`, `hero`); this plan builds the first two plus the terminal core. `registry` and `theme` are headless and fully testable with no terminal.

**Tech Stack:** Go 1.26.7, Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2, vendored deps, `buildGoModule` via `~/.nix-config/templates/go`.

**Spec:** `docs/superpowers/specs/2026-09-18-volinit-cockpit-design.md`

## Global Constraints

- Go module path: `github.com/lowcache/volinit`
- Deps are **vendored** (`vendor/` committed). `vendorHash = null`. Flake check sandbox has no network.
- **Never execute discovered Makefiles.** Parse statically, `help:` recipes included.
- Work on branch `rewrite/go-cockpit`. `main` stays Nim — it is a live flake input at `~/.nix-config/home/pkgs.nix:173` and breaking it breaks `make switch`.
- No visual/animation work in this plan. That is gated on operator sign-off.
- Every terminal-touching path must restore the terminal on panic and SIGINT.

---

### Task 1: Branch, flake, and buildable Go skeleton

**Files:**
- Create: `flake.nix` (from template), `go.mod`, `main.go`, `.envrc`
- Delete: none yet — Nim stays until Go is proven

**Interfaces:**
- Produces: a `volinit` binary that builds under `nix build`

- [ ] **Step 1: Branch**

```bash
git checkout -b rewrite/go-cockpit
```

- [ ] **Step 2: Instantiate the Go template**

```bash
cp /home/lowcache/.nix-config/templates/go/flake.nix ./flake.nix.new
cp /home/lowcache/.nix-config/templates/go/.envrc ./.envrc
```

Edit `flake.nix.new`: set `pname = "volinit";` and leave `vendorHash = null;` (correct once `vendor/` exists). Set:

```nix
checkCommands = {
  vet = "go vet ./...";
  test = "go test ./...";
};
```

Then `mv flake.nix.new flake.nix` (replacing the Nim flake).

- [ ] **Step 3: Init the module**

```bash
go mod init github.com/lowcache/volinit
```

- [ ] **Step 4: Minimal main**

```go
package main

import "fmt"

func main() {
	fmt.Println("volinit")
}
```

- [ ] **Step 5: Verify it builds under nix**

Run: `nix build .#default && ./result/bin/volinit`
Expected: prints `volinit`

- [ ] **Step 6: Commit**

```bash
git add flake.nix go.mod main.go .envrc
git commit -m "feat: Go skeleton building under the nix-config go template"
```

---

### Task 2: Terminal restore core

The spec makes this first because every other feature sits on it, and a
terminal left in raw mode is the difference between "solid" and "janky".

**Files:**
- Create: `internal/term/restore.go`, `internal/term/restore_test.go`

**Interfaces:**
- Produces: `term.Guard` with `func NewGuard() (*Guard, error)`, `func (g *Guard) Restore()`, and `func (g *Guard) Wrap(fn func() error) error`

- [ ] **Step 1: Write the failing test**

```go
package term

import (
	"errors"
	"testing"
)

func TestWrapRestoresOnPanic(t *testing.T) {
	g := &Guard{restored: false, restore: func() {}}
	err := g.Wrap(func() error { panic("boom") })
	if err == nil || !errors.Is(err, ErrPanicked) {
		t.Fatalf("want ErrPanicked, got %v", err)
	}
	if !g.restored {
		t.Fatal("terminal was not restored after panic")
	}
}

func TestWrapRestoresOnError(t *testing.T) {
	g := &Guard{restored: false, restore: func() {}}
	want := errors.New("nope")
	got := g.Wrap(func() error { return want })
	if !errors.Is(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	if !g.restored {
		t.Fatal("terminal was not restored after error")
	}
}

func TestRestoreIsIdempotent(t *testing.T) {
	calls := 0
	g := &Guard{restore: func() { calls++ }}
	g.Restore()
	g.Restore()
	if calls != 1 {
		t.Fatalf("restore ran %d times, want 1", calls)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/term/ -v`
Expected: FAIL — `undefined: Guard`

- [ ] **Step 3: Implement**

```go
package term

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// ErrPanicked wraps a panic recovered by Wrap so callers get an error
// instead of a process that dies with the terminal in raw mode.
var ErrPanicked = errors.New("panicked")

type Guard struct {
	once     sync.Once
	restored bool
	restore  func()
}

// Restore returns the terminal to its entry state. Safe to call repeatedly;
// only the first call does anything.
func (g *Guard) Restore() {
	g.once.Do(func() {
		if g.restore != nil {
			g.restore()
		}
		g.restored = true
	})
}

// Wrap runs fn with restore guaranteed on every exit path: normal return,
// error, panic, or SIGINT/SIGTERM.
func (g *Guard) Wrap(fn func() error) (err error) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	go func() {
		<-sigs
		g.Restore()
		os.Exit(130)
	}()

	defer func() {
		g.Restore()
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrPanicked, r)
		}
	}()
	return fn()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/term/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/term/
git commit -m "feat(term): restore guard covering panic, error and signal paths"
```

---

### Task 3: Noctalia palette reader

**Files:**
- Create: `internal/theme/palette.go`, `internal/theme/palette_test.go`, `internal/theme/testdata/palette.toml`

**Interfaces:**
- Consumes: nothing
- Produces: `theme.Palette` struct with fields `Primary, OnPrimary, Surface, OnSurface, SurfaceVariant, SurfaceContainer, Outline, Error string` and `Neutral [18]string`; `func Load(path string) (Palette, error)`; `func (p Palette) Hash() string`

- [ ] **Step 1: Fixture**

Create `internal/theme/testdata/palette.toml`:

```toml
primary            = "#e5c799"
on_primary         = "#161311"
surface            = "#161311"
on_surface         = "#dcd8cd"
surface_variant    = "#231e1a"
surface_container  = "#231e1a"
outline            = "#6b6057"
error              = "#cf767c"
neutral_0 = "#000000"
neutral_1 = "#13100e"
```

- [ ] **Step 2: Write the failing test**

```go
package theme

import "testing"

func TestLoadReadsM3Roles(t *testing.T) {
	p, err := Load("testdata/palette.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Primary != "#e5c799" {
		t.Errorf("Primary = %q, want #e5c799", p.Primary)
	}
	if p.OnSurface != "#dcd8cd" {
		t.Errorf("OnSurface = %q, want #dcd8cd", p.OnSurface)
	}
	if p.Neutral[1] != "#13100e" {
		t.Errorf("Neutral[1] = %q, want #13100e", p.Neutral[1])
	}
}

func TestMissingFileReturnsDefaultPalette(t *testing.T) {
	p, err := Load("testdata/does-not-exist.toml")
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	if p.Primary == "" {
		t.Fatal("default palette must be populated")
	}
}

func TestHashChangesWithPalette(t *testing.T) {
	a, _ := Load("testdata/palette.toml")
	b := a
	b.Primary = "#ffffff"
	if a.Hash() == b.Hash() {
		t.Fatal("hash must change when a role changes")
	}
}
```

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/theme/ -v`
Expected: FAIL — `undefined: Load`

- [ ] **Step 4: Add the TOML dep and vendor**

```bash
go get github.com/BurntSushi/toml
go mod vendor
```

- [ ] **Step 5: Implement**

```go
package theme

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Palette holds the Material 3 roles noctalia renders. Missing values fall
// back to Default so a fresh machine with no template still looks right.
type Palette struct {
	Primary          string `toml:"primary"`
	OnPrimary        string `toml:"on_primary"`
	Surface          string `toml:"surface"`
	OnSurface        string `toml:"on_surface"`
	SurfaceVariant   string `toml:"surface_variant"`
	SurfaceContainer string `toml:"surface_container"`
	Outline          string `toml:"outline"`
	Error            string `toml:"error"`
	Neutral          [18]string
}

// Default is the compiled-in palette, used when noctalia has written nothing.
func Default() Palette {
	return Palette{
		Primary: "#e5c799", OnPrimary: "#161311",
		Surface: "#161311", OnSurface: "#dcd8cd",
		SurfaceVariant: "#231e1a", SurfaceContainer: "#231e1a",
		Outline: "#6b6057", Error: "#cf767c",
	}
}

// Load reads a noctalia-rendered palette. A missing file is not an error:
// volinit runs on every shell and must never fail because a theme is absent.
func Load(path string) (Palette, error) {
	p := Default()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return p, err
	}
	var flat map[string]any
	if _, err := toml.Decode(string(raw), &flat); err != nil {
		return p, err
	}
	if _, err := toml.Decode(string(raw), &p); err != nil {
		return p, err
	}
	for i := 0; i < 18; i++ {
		if v, ok := flat[fmt.Sprintf("neutral_%d", i)].(string); ok {
			p.Neutral[i] = v
		}
	}
	return p, nil
}

// Hash keys the hero frame cache: a palette change must invalidate it.
func (p Palette) Hash() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%#v", p)))
	return hex.EncodeToString(sum[:8])
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/theme/ -v`
Expected: PASS (3 tests)

- [ ] **Step 7: Commit**

```bash
git add internal/theme/ go.mod go.sum vendor/
git commit -m "feat(theme): read noctalia M3 palette, defaulting when absent"
```

---

### Task 4: Makefile parser — the `##` dialect

**Files:**
- Create: `internal/registry/makefile.go`, `internal/registry/makefile_test.go`, `internal/registry/testdata/hashhash.mk`

**Interfaces:**
- Consumes: nothing
- Produces: `registry.Action{Name, Description, Section string}`; `func ParseHashHash(src string) []Action`

- [ ] **Step 1: Fixture**

Create `internal/registry/testdata/hashhash.mk`:

```make
## System Operations
## :switch: ..........: Rebuild and switch system live
switch:
	sudo nixos-rebuild switch

## :build: ..........: Build without switching
build:
	nix build

## Anonymous Mode
## :anon-arm: ..........: Arm anonymous mode
anon-arm:
	@echo arming
```

- [ ] **Step 2: Write the failing test**

```go
package registry

import (
	"os"
	"testing"
)

func TestParseHashHashExtractsSectionsAndTargets(t *testing.T) {
	src, err := os.ReadFile("testdata/hashhash.mk")
	if err != nil {
		t.Fatal(err)
	}
	got := ParseHashHash(string(src))
	if len(got) != 3 {
		t.Fatalf("got %d actions, want 3", len(got))
	}
	if got[0].Name != "switch" {
		t.Errorf("Name = %q, want switch", got[0].Name)
	}
	if got[0].Description != "Rebuild and switch system live" {
		t.Errorf("Description = %q", got[0].Description)
	}
	if got[0].Section != "System Operations" {
		t.Errorf("Section = %q, want System Operations", got[0].Section)
	}
	if got[2].Section != "Anonymous Mode" {
		t.Errorf("section did not switch, got %q", got[2].Section)
	}
}

func TestParseHashHashIgnoresPlainComments(t *testing.T) {
	got := ParseHashHash("# just a comment\nfoo:\n\techo hi\n")
	if len(got) != 0 {
		t.Fatalf("got %d actions, want 0", len(got))
	}
}
```

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/registry/ -v`
Expected: FAIL — `undefined: ParseHashHash`

- [ ] **Step 4: Implement**

```go
package registry

import (
	"regexp"
	"strings"
)

// Action is one runnable entry discovered in a repository.
type Action struct {
	Name        string
	Description string
	Section     string
}

// targetLine matches `## :name: ....: description` — the dots are decorative
// and variable in length, so they are consumed rather than counted.
var targetLine = regexp.MustCompile(`^##\s*:([A-Za-z0-9_-]+):\s*\.*\s*:\s*(.+?)\s*$`)

// sectionLine matches `## Section Name` but not a target line, which is why
// this is applied only after targetLine fails.
var sectionLine = regexp.MustCompile(`^##\s+([A-Z][^:]*?)\s*$`)

// ParseHashHash reads the nix-config dialect: `## Section` headers grouping
// `## :target: ....: description` entries. Nothing is executed.
func ParseHashHash(src string) []Action {
	var out []Action
	section := ""
	for _, line := range strings.Split(src, "\n") {
		if m := targetLine.FindStringSubmatch(line); m != nil {
			out = append(out, Action{Name: m[1], Description: m[2], Section: section})
			continue
		}
		if m := sectionLine.FindStringSubmatch(line); m != nil {
			section = m[1]
		}
	}
	return out
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/registry/ -v`
Expected: PASS (2 tests)

- [ ] **Step 6: Verify against the real Makefile**

```bash
cat > /tmp/volinit_smoke.go <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/lowcache/volinit/internal/registry"
)

func main() {
	b, _ := os.ReadFile("/home/lowcache/.nix-config/Makefile")
	a := registry.ParseHashHash(string(b))
	fmt.Println("actions:", len(a))
}
EOF
go run /tmp/volinit_smoke.go
```

Expected: `actions: 45` — matching `grep -c '^## :' ~/.nix-config/Makefile`.

- [ ] **Step 7: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): parse the ## Makefile dialect"
```

---

### Task 5: Makefile parser — the `help:` dialect

**Files:**
- Modify: `internal/registry/makefile.go`
- Modify: `internal/registry/makefile_test.go`
- Create: `internal/registry/testdata/helptarget.mk`

**Interfaces:**
- Consumes: `Action` from Task 4
- Produces: `func ParseHelpTarget(src string) []Action`

- [ ] **Step 1: Fixture**

Create `internal/registry/testdata/helptarget.mk`:

```make
help:
	@echo "make serve       Live preview incl. drafts (http://localhost:1313)"
	@echo "make build       Production build to ./public (runs ./build.sh)"
	@echo "make deploy      Build, upload ./public to Cloudflare Workers, then verify"

serve:
	hugo server -D
```

- [ ] **Step 2: Write the failing test**

```go
func TestParseHelpTargetReadsEchoedLines(t *testing.T) {
	src, err := os.ReadFile("testdata/helptarget.mk")
	if err != nil {
		t.Fatal(err)
	}
	got := ParseHelpTarget(string(src))
	if len(got) != 3 {
		t.Fatalf("got %d actions, want 3", len(got))
	}
	if got[0].Name != "serve" {
		t.Errorf("Name = %q, want serve", got[0].Name)
	}
	if got[0].Description != "Live preview incl. drafts (http://localhost:1313)" {
		t.Errorf("Description = %q", got[0].Description)
	}
	if got[2].Name != "deploy" {
		t.Errorf("Name = %q, want deploy", got[2].Name)
	}
}

func TestParseHelpTargetStopsAtNextTarget() {}
```

Replace that last stub with:

```go
func TestParseHelpTargetStopsAtRecipeEnd(t *testing.T) {
	src := "help:\n\t@echo \"make a   first\"\n\nother:\n\t@echo \"make b   second\"\n"
	got := ParseHelpTarget(src)
	if len(got) != 1 {
		t.Fatalf("got %d actions, want 1 — must not read past the help recipe", len(got))
	}
}
```

- [ ] **Step 3: Run it and confirm it fails**

Run: `go test ./internal/registry/ -v`
Expected: FAIL — `undefined: ParseHelpTarget`

- [ ] **Step 4: Implement**

```go
// echoedHelp matches `@echo "make <target>   <description>"` inside a help
// recipe. Quotes may be single or double; spacing between the two is
// decorative alignment.
var echoedHelp = regexp.MustCompile(`@echo\s+["']make\s+([A-Za-z0-9_-]+)\s+(.+?)["']\s*$`)

// ParseHelpTarget reads the blog dialect: a `help:` target whose recipe
// echoes its own documentation. The recipe is read statically — `make help`
// is never run, which keeps discovery cheap and safe at shell-init time.
func ParseHelpTarget(src string) []Action {
	var out []Action
	inHelp := false
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "help:") {
			inHelp = true
			continue
		}
		if !inHelp {
			continue
		}
		// A recipe line starts with a tab. Anything else ends the recipe.
		if !strings.HasPrefix(line, "\t") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			break
		}
		if m := echoedHelp.FindStringSubmatch(line); m != nil {
			out = append(out, Action{Name: m[1], Description: strings.TrimSpace(m[2])})
		}
	}
	return out
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/registry/ -v`
Expected: PASS (4 tests)

- [ ] **Step 6: Verify against a real blog Makefile**

```bash
cat > /tmp/volinit_smoke2.go <<'EOF'
package main

import (
	"fmt"
	"os"

	"github.com/lowcache/volinit/internal/registry"
)

func main() {
	b, _ := os.ReadFile("/home/lowcache/CodeRepo/sites/blogs/wiki/Makefile")
	for _, a := range registry.ParseHelpTarget(string(b)) {
		fmt.Printf("%-12s %s\n", a.Name, a.Description)
	}
}
EOF
go run /tmp/volinit_smoke2.go
```

Expected: six rows — `serve`, `build`, `deploy`, `verify`, `mod-update`, `clean`.

- [ ] **Step 7: Commit**

```bash
git add internal/registry/
git commit -m "feat(registry): parse the help: Makefile dialect used by the blogs"
```

---

## What this plan deliberately excludes

Continues in a follow-up plan, in this order:

1. Fleet discovery and the three-branch tree (system / writing / code)
2. Sidecar `.volinit/actions.toml` and `volinit doctor`
3. Execution: `tea.ExecProcess` and the detached `systemd-run --user` path
4. Minimal Bubble Tea runtime and shell wiring, with the startup budget benchmark

**Then: CHECKPOINT — operator sign-off on graphics and animation direction
before any hero work begins.**

---

## Amendment (2026-09-18, after API verification)

Bubble Tea v2 restores the terminal on panic and SIGINT **by default**
(opt out via `tea.WithoutCatchPanics()` / `tea.WithoutSignalHandler()`).

Task 2's `term.Guard` is therefore **not** for the Bubble Tea program. It
covers the paths that run without one:

* `greet_mode = "frame"` — hero replay writes escape sequences directly and
  exits, never starting a program
* the fast banner path on shell init
* raw sequence work during terminal capability probing

Keep Task 2 as written; its justification is narrower than the spec implied.
Do not add signal handling around the Bubble Tea program itself.
