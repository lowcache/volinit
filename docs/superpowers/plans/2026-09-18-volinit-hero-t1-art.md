# volinit Hero T1 — the volnix Assembly — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the placeholder greeting with the volnix exploded assembly drawn in braille cells, and morph it one way into an assembled sidebar strip beside the action list.

**Architecture:** `internal/hero` gains a braille canvas with two ink roles and an isometric plate renderer driven by a `Pose` (axis, centre, size, spread). `internal/runtime` renders the greeting pose full bleed, interpolates poses during the morph, and joins the strip pose beside the list. No image protocols, no Limoni, no new dependencies.

**Tech Stack:** Go 1.26.2 (nix-provided), `charm.land/bubbletea/v2` v2.0.9, `charm.land/lipgloss/v2` v2.0.6, vendored.

**Spec:** `docs/superpowers/specs/2026-09-18-volinit-cockpit-design.md` — read both addenda: the hero interaction model, and the tier revision / hero subject. Continues `docs/superpowers/plans/2026-09-18-volinit-hero-t1.md` (Tasks 1–2 done: `d9a0d68`, `b0103d1`).

## Global Constraints

- Branch `rewrite/go-cockpit`. `main` stays Nim — it is a live flake input at `~/.nix-config/home/pkgs.nix:173`.
- Deps vendored; `vendorHash = null`; the check sandbox has no network and no C compiler. Add NO dependencies. Do not run `go get`, `go mod tidy`, or touch `go.mod`, `go.sum`, `vendor/`.
- Stage by explicit path only. Never `git add -A`, `.`, or `-a`.
- **T1 only.** Braille cells and truecolor SGR. No kitty graphics, sixel, or Limoni.
- Every colour comes from `theme.Palette`: ink is `OnSurface`, faint marks are `Outline`. `Primary` never appears in the art — it is the list caret, the one located item. No hardcoded RGB outside tests.
- **The one-way door:** `stageFullBleed` is assigned only in `New`. Any code path back to it is a defect.
- **Dismissal consumes its key**, and so does a key that interrupts the morph. Neither may run a target.
- `q`, `esc`, `ctrl+c` quit straight from the greeting.
- The greeting renders live on every shell start; `TestGreetingFrameWithinBudget` is the gate that lets the disk cache go unbuilt.
- Tests may invoke a returned `tea.Cmd` only when it is `tea.Quit` or the morph tick. Anything else could start a real process.
- `internal/registry/sidecar.go` and `internal/registry/makefile_test.go` already fail `gofmt -l`; that predates this plan. Do not reformat them. New and touched files must be gofmt-clean.

## Deliberately out of this plan

- The disk frame cache — built only if `TestGreetingFrameWithinBudget` cannot be met.
- The opening sequence (adaptive depth, `$XDG_RUNTIME_DIR` flag).
- A glyph-set fallback for the Linux VT console (unverified for braille; spec OPEN item).
- T2 kitty frames.

---

### Task 3: Greeting quit keys, injected tier, T0 skips the greeting

Review finding on Task 1+2: `New` calls `hero.Detect` itself, so every test's
model depends on how `go test` was invoked (a pipe gives T0, a terminal T1).
Detection moves to `Run`; `New` takes the tier.

**Files:**
- Modify: `internal/runtime/model.go` (`New`, `key`, `Run`)
- Modify: `internal/runtime/model_test.go`

**Interfaces:**
- Consumes: `hero.Tier`, `hero.T0`, `hero.T1`, `hero.Detect` (Task 1)
- Produces: `func New(repos []registry.Repo, p theme.Palette, notices []string, tier hero.Tier) Model`

- [ ] **Step 1: Update every `New(` call in `model_test.go` to pass `hero.T1` as a fourth argument**

There are 11 calls, including `fixture()` and `wide()`. Add the import
`"github.com/lowcache/volinit/internal/hero"`. Example:

```go
func fixture() Model {
	return New([]registry.Repo{
		{Name: "cfg", Branch: registry.BranchSystem, Path: "/tmp/cfg", Actions: []registry.Action{
			{Name: "switch"}, {Name: "build"},
		}},
	}, theme.Default(), nil, hero.T1)
}
```

- [ ] **Step 2: Add the failing tests to `model_test.go`**

```go
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
```

- [ ] **Step 3: Run and confirm failure**

Run: `go test ./internal/runtime/`
Expected: FAIL — compile error `too many arguments in call to New`.

- [ ] **Step 4: Implement in `model.go`**

Replace `New`:

```go
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
```

Replace the full-bleed block at the top of `key`:

```go
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
```

Replace `Run`:

```go
// Run starts the cockpit. Bubble Tea restores the terminal on panic and
// SIGINT itself, so no guard is wrapped around this.
func Run(repos []registry.Repo, p theme.Palette, notices []string) error {
	tier := hero.Detect(os.Getenv, term.IsTerminal(os.Stdout.Fd()))
	_, err := tea.NewProgram(New(repos, p, notices, tier)).Run()
	return err
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/runtime/model.go internal/runtime/model_test.go
git commit -m "feat(runtime): quit keys at the greeting; tier injected, T0 skips it"
```

---

### Task 4: The braille canvas

**Files:**
- Create: `internal/hero/canvas.go`
- Create: `internal/hero/canvas_test.go`

**Interfaces:**
- Consumes: `theme.Palette` (`OnSurface`, `Outline` hex strings like `"#dcd8cd"`)
- Produces:
  - `type Role uint8` with `RoleNone`, `RoleFaint`, `RoleInk` (ordered by priority)
  - `func NewCanvas(cols, rows int) *Canvas`
  - `func (c *Canvas) DotW() int`, `DotH() int`
  - `func (c *Canvas) Set(x, y int, r Role)`, `At(x, y int) Role`
  - `func (c *Canvas) Line(x0, y0, x1, y1 int, r Role, dash []int)`
  - `func (c *Canvas) Text(col, row int, s string, r Role)`
  - `func (c *Canvas) Plain() string`, `Render(p theme.Palette) string`

- [ ] **Step 1: Write the failing tests**

```go
package hero

import (
	"strings"
	"testing"

	"github.com/lowcache/volinit/internal/theme"
)

func TestBrailleDotsEncode(t *testing.T) {
	c := NewCanvas(1, 1)
	c.Set(0, 0, RoleInk)
	if got := c.Plain(); got != "⠁" {
		t.Fatalf("top-left dot = %q, want U+2801", got)
	}
	c.Set(1, 3, RoleInk)
	if got := c.Plain(); got != "⢁" {
		t.Fatalf("adding the bottom-right dot = %q, want U+2881", got)
	}
}

func TestPlainRowsAreFullWidth(t *testing.T) {
	if got := NewCanvas(3, 2).Plain(); got != "   \n   " {
		t.Fatalf("blank 3x2 = %q", got)
	}
}

func TestOutOfRangeDotsAreIgnored(t *testing.T) {
	c := NewCanvas(2, 2)
	c.Set(-1, 0, RoleInk)
	c.Set(0, -1, RoleInk)
	c.Set(4, 0, RoleInk)
	c.Set(0, 8, RoleInk)
	if strings.TrimSpace(strings.ReplaceAll(c.Plain(), "\n", "")) != "" {
		t.Fatal("a dot outside the canvas landed inside it")
	}
}

func TestLineReachesBothEnds(t *testing.T) {
	c := NewCanvas(10, 10)
	c.Line(1, 2, 17, 9, RoleInk, nil)
	if c.At(1, 2) != RoleInk || c.At(17, 9) != RoleInk {
		t.Fatal("line missed an endpoint")
	}
}

func TestDashedLineLeavesGaps(t *testing.T) {
	c := NewCanvas(4, 1)
	c.Line(0, 0, 7, 0, RoleFaint, []int{2, 2})
	want := []Role{RoleFaint, RoleFaint, RoleNone, RoleNone, RoleFaint, RoleFaint, RoleNone, RoleNone}
	for x, w := range want {
		if got := c.At(x, 0); got != w {
			t.Errorf("dot %d = %v, want %v", x, got, w)
		}
	}
}

func TestInkWinsASharedCell(t *testing.T) {
	c := NewCanvas(1, 1)
	c.Set(0, 0, RoleFaint)
	c.Set(1, 0, RoleInk)
	got := c.Render(theme.Default())
	if !strings.Contains(got, "\x1b[38;2;220;216;205m") { // on_surface #dcd8cd
		t.Errorf("shared cell not drawn in ink: %q", got)
	}
	if strings.Contains(got, "\x1b[38;2;107;96;87m") { // outline #6b6057
		t.Errorf("shared cell also drew the faint colour: %q", got)
	}
}

func TestTextReplacesBraille(t *testing.T) {
	c := NewCanvas(4, 1)
	c.Line(0, 0, 7, 3, RoleInk, nil)
	c.Text(1, 0, "ab", RoleInk)
	c.Text(3, 0, "overflow", RoleInk)
	got := []rune(c.Plain())
	if len(got) != 4 || got[1] != 'a' || got[2] != 'b' || got[3] != 'o' {
		t.Fatalf("text overlay = %q", string(got))
	}
}

func TestBadPaletteColourEmitsNoEscape(t *testing.T) {
	c := NewCanvas(1, 1)
	c.Set(0, 0, RoleInk)
	p := theme.Default()
	p.OnSurface = "not-a-colour"
	if got := c.Render(p); strings.Contains(got, "38;2") {
		t.Errorf("an unparseable colour emitted an escape: %q", got)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/hero/`
Expected: FAIL — `undefined: NewCanvas`.

- [ ] **Step 3: Implement `internal/hero/canvas.go`**

```go
package hero

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lowcache/volinit/internal/theme"
)

// Role is what a mark means, which decides its colour. Higher roles win a
// cell shared with lower ones, since a cell carries one foreground.
type Role uint8

const (
	RoleNone  Role = iota
	RoleFaint      // hatching, leaders, construction lines: outline
	RoleInk        // part edges and lettering: on_surface
)

func (r Role) color(p theme.Palette) string {
	switch r {
	case RoleInk:
		return p.OnSurface
	case RoleFaint:
		return p.Outline
	}
	return ""
}

// Canvas is a grid of braille cells, each 2 dots wide and 4 tall, plus a
// text overlay that replaces a cell's braille where set.
type Canvas struct {
	cols, rows int
	dots       []Role // DotW × DotH, row-major
	text       []rune // cols × rows; 0 = no overlay
	textRole   []Role
}

func NewCanvas(cols, rows int) *Canvas {
	cols, rows = max(cols, 0), max(rows, 0)
	return &Canvas{
		cols:     cols,
		rows:     rows,
		dots:     make([]Role, 8*cols*rows),
		text:     make([]rune, cols*rows),
		textRole: make([]Role, cols*rows),
	}
}

func (c *Canvas) DotW() int { return 2 * c.cols }
func (c *Canvas) DotH() int { return 4 * c.rows }

// Set marks one dot; RoleNone clears it. Dots off the canvas are ignored so
// geometry may run past the edge.
func (c *Canvas) Set(x, y int, r Role) {
	if x < 0 || y < 0 || x >= c.DotW() || y >= c.DotH() {
		return
	}
	c.dots[y*c.DotW()+x] = r
}

// At reports one dot's role, RoleNone off the canvas.
func (c *Canvas) At(x, y int) Role {
	if x < 0 || y < 0 || x >= c.DotW() || y >= c.DotH() {
		return RoleNone
	}
	return c.dots[y*c.DotW()+x]
}

// Line draws both endpoints inclusive (Bresenham). dash is an on/off run
// pattern in dots, repeated along the line; nil is solid.
func (c *Canvas) Line(x0, y0, x1, y1 int, r Role, dash []int) {
	period := 0
	for _, d := range dash {
		period += d
	}
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for i := 0; ; i++ {
		if on(dash, period, i) {
			c.Set(x0, y0, r)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err // once per step: both tests use the same error
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// on reports whether step i of a dashed line is inked.
func on(dash []int, period, i int) bool {
	if period == 0 {
		return true
	}
	i %= period
	for k, d := range dash {
		if i < d {
			return k%2 == 0
		}
		i -= d
	}
	return true
}

// Text overlays s from cell (col, row), clipped to the canvas.
func (c *Canvas) Text(col, row int, s string, r Role) {
	if row < 0 || row >= c.rows {
		return
	}
	for _, ch := range s {
		if col >= 0 && col < c.cols {
			c.text[row*c.cols+col] = ch
			c.textRole[row*c.cols+col] = r
		}
		col++
	}
}

// brailleBit is the U+2800-block bit for the dot at [dy][dx] within a cell.
var brailleBit = [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

// cell returns a cell's glyph and the highest role among its marks.
func (c *Canvas) cell(col, row int) (rune, Role) {
	i := row*c.cols + col
	if c.text[i] != 0 {
		return c.text[i], c.textRole[i]
	}
	var mask rune
	role := RoleNone
	for dy := 0; dy < 4; dy++ {
		for dx := 0; dx < 2; dx++ {
			r := c.dots[(4*row+dy)*c.DotW()+2*col+dx]
			if r == RoleNone {
				continue
			}
			mask |= brailleBit[dy][dx]
			role = max(role, r)
		}
	}
	if mask == 0 {
		return ' ', RoleNone
	}
	return 0x2800 + mask, role
}

// Plain is the canvas as glyphs with no escapes: rows joined by newlines,
// each exactly cols wide.
func (c *Canvas) Plain() string {
	var b strings.Builder
	for row := 0; row < c.rows; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		for col := 0; col < c.cols; col++ {
			ch, _ := c.cell(col, row)
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// Render is Plain with each run of same-role cells coloured from the
// palette. An unparseable palette colour falls back to the default fg.
func (c *Canvas) Render(p theme.Palette) string {
	sgr := map[Role]string{}
	for _, r := range []Role{RoleFaint, RoleInk} {
		if rgb, ok := parseHex(r.color(p)); ok {
			sgr[r] = fmt.Sprintf("\x1b[38;2;%d;%d;%dm", rgb[0], rgb[1], rgb[2])
		}
	}
	const reset = "\x1b[39m"
	var b strings.Builder
	for row := 0; row < c.rows; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		cur := RoleNone
		for col := 0; col < c.cols; col++ {
			ch, r := c.cell(col, row)
			if r != cur {
				if s, ok := sgr[r]; ok {
					b.WriteString(s)
				} else {
					b.WriteString(reset)
				}
				cur = r
			}
			b.WriteRune(ch)
		}
		if cur != RoleNone {
			b.WriteString(reset)
		}
	}
	return b.String()
}

func parseHex(s string) ([3]uint8, bool) {
	var rgb [3]uint8
	if len(s) != 7 || s[0] != '#' {
		return rgb, false
	}
	for i := range rgb {
		v, err := strconv.ParseUint(s[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return rgb, false
		}
		rgb[i] = uint8(v)
	}
	return rgb, true
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/hero/ -v && go vet ./internal/hero/ && gofmt -l internal/hero/`
Expected: PASS (4 tier tests + 8 canvas tests); gofmt prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/hero/canvas.go internal/hero/canvas_test.go
git commit -m "feat(hero): braille canvas with ink and faint roles"
```

---

### Task 5: The volnix assembly

The drawing from `wiki.infernalcode.com`: ten plates on a vertical axis,
base first; the six from tmpfs root up are volatile and hatched; the phone
is a detached sub-assembly tied to the MicroVM gateways. Projection is
2:1 isometric (the wiki's plates are 380×190 px), so a plate of half-depth
`R` dots is `4R` wide and `2R` deep on screen. Plates draw bottom up and
each erases its own silhouette first: for convex plates stacked on one axis
under a fixed view from above, that is exact hidden-line removal.

**Files:**
- Create: `internal/hero/assembly.go`
- Create: `internal/hero/assembly_test.go`

**Interfaces:**
- Consumes: the Task 4 canvas
- Produces:
  - `type Part struct { Name string; Volatile bool }`, `var Volnix []Part`, `var Phone Part`
  - `type Pose struct { CX, CY, R int; Spread float64 }`
  - `func GreetingPose(cols, rows int) (Pose, bool)` — bool: room for lettering
  - `func StripPose(cols, rows int) Pose`
  - `func Lerp(a, b Pose, t float64) Pose`
  - `func Draw(cols, rows int, p Pose, labels bool) *Canvas`
  - `func Frame(cols, rows int, p Pose, labels bool, pal theme.Palette) string`

- [ ] **Step 1: Write the failing tests**

```go
package hero

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/lowcache/volinit/internal/theme"
)

func TestVolnixMatchesTheWiki(t *testing.T) {
	if len(Volnix) != 10 {
		t.Fatalf("the wiki draws ten plates, got %d", len(Volnix))
	}
	if Volnix[0].Name != "UEFI + Lanzaboote" || Volnix[9].Name != "niri + Noctalia v5" {
		t.Errorf("stack runs boot chain to shell, got %q .. %q", Volnix[0].Name, Volnix[9].Name)
	}
	for i, p := range Volnix {
		if want := i >= datum; p.Volatile != want {
			t.Errorf("%s: volatile = %v, want %v (tmpfs root is the datum)", p.Name, p.Volatile, want)
		}
	}
	if Volnix[datum].Name != "tmpfs root" || Volnix[phoneJoin].Name != "MicroVM gateways" {
		t.Error("datum or phone join points at the wrong plate")
	}
}

func TestPlateHatchesOnlyWhenVolatile(t *testing.T) {
	for _, hatched := range []bool{true, false} {
		c := NewCanvas(40, 20)
		plate(c, 40, 30, 16, 3, hatched)
		faint := 0
		for y := 0; y < c.DotH(); y++ {
			for x := 0; x < c.DotW(); x++ {
				if c.At(x, y) == RoleFaint {
					faint++
				}
			}
		}
		if hatched && faint == 0 {
			t.Error("a volatile plate drew no hatching")
		}
		if !hatched && faint != 0 {
			t.Errorf("a persistent plate drew %d hatch dots", faint)
		}
	}
}

// The top plate has nothing above it, so the only marks inside its face may
// be its own hatching: ink there is a lower plate showing through.
func TestTopPlateFaceIsHatchedAndOpaque(t *testing.T) {
	p := StripPose(24, 40)
	c := Draw(24, 40, p, false)
	cy := p.center(len(Volnix) - 1)
	hatch := 0
	for y := cy - p.R; y <= cy+p.R; y++ {
		for x := p.CX - 2*p.R; x <= p.CX+2*p.R; x++ {
			if abs(x-p.CX)/2+abs(y-cy) > p.R-2 {
				continue // on or next to the face's own edges
			}
			switch c.At(x, y) {
			case RoleInk:
				t.Fatalf("ink at (%d,%d) inside the top face: a lower plate shows through", x, y)
			case RoleFaint:
				hatch++
			}
		}
	}
	if hatch == 0 {
		t.Error("the top plate is volatile but its face carries no hatching")
	}
}

func TestFrameIsExactlyTheRequestedSize(t *testing.T) {
	for _, g := range [][2]int{{200, 50}, {80, 24}, {30, 40}, {24, 40}, {5, 3}} {
		cols, rows := g[0], g[1]
		greet, labels := GreetingPose(cols, rows)
		for _, c := range []*Canvas{Draw(cols, rows, greet, labels), Draw(cols, rows, StripPose(cols, rows), false)} {
			lines := strings.Split(c.Plain(), "\n")
			if len(lines) != rows {
				t.Fatalf("%dx%d: %d lines", cols, rows, len(lines))
			}
			for i, l := range lines {
				if n := utf8.RuneCountInString(l); n != cols {
					t.Fatalf("%dx%d: line %d is %d wide", cols, rows, i, n)
				}
			}
		}
	}
}

func TestGreetingLettersThePartsWhenThereIsRoom(t *testing.T) {
	p, labels := GreetingPose(200, 50)
	if !labels {
		t.Fatal("200x50 has room for the parts list")
	}
	out := Draw(200, 50, p, true).Plain()
	for _, want := range []string{"(01) UEFI + Lanzaboote", "(10) niri + Noctalia v5", "(11)", "Nix-on-Droid", "phone tier"} {
		if !strings.Contains(out, want) {
			t.Errorf("greeting missing %q", want)
		}
	}
}

func TestNarrowGreetingIsTheBareStack(t *testing.T) {
	p, labels := GreetingPose(30, 40)
	if labels {
		t.Fatal("30 columns cannot hold the parts list")
	}
	out := Draw(30, 40, p, false).Plain()
	if strings.Contains(out, "(") {
		t.Error("a bare stack carries no lettering")
	}
	if strings.TrimSpace(strings.ReplaceAll(out, "\n", "")) == "" {
		t.Error("the bare stack drew nothing")
	}
}

func TestStripPoseFitsTheStrip(t *testing.T) {
	p := StripPose(24, 40)
	if p.R < 2 {
		t.Fatalf("a 24x40 strip should fit the stack, R = %d", p.R)
	}
	if p.CX-2*p.R < 0 || p.CX+2*p.R >= 48 || p.top() < 0 || p.top()+p.height() >= 160 {
		t.Errorf("stack overflows the strip: %+v", p)
	}
	if p.Spread != 0 {
		t.Error("the strip holds the assembly closed")
	}
}

func TestLerpHitsBothEnds(t *testing.T) {
	a, _ := GreetingPose(200, 50)
	b := StripPose(24, 50)
	if Lerp(a, b, 0) != a || Lerp(a, b, 1) != b {
		t.Error("the morph must start at the greeting and end at the strip exactly")
	}
}

// The greeting renders live on every shell start; this budget is what lets
// the disk cache go unbuilt. Its log line is the evidence for that decision.
func TestGreetingFrameWithinBudget(t *testing.T) {
	const cols, rows, runs = 240, 70, 20
	const budget = 4 * time.Millisecond
	p, labels := GreetingPose(cols, rows)
	pal := theme.Default()
	Frame(cols, rows, p, labels, pal) // warm up
	start := time.Now()
	for i := 0; i < runs; i++ {
		Frame(cols, rows, p, labels, pal)
	}
	avg := time.Since(start) / runs
	t.Logf("greeting frame %dx%d: %v", cols, rows, avg)
	if avg > budget {
		t.Fatalf("frame took %v, budget %v", avg, budget)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/hero/`
Expected: FAIL — `undefined: Volnix`.

- [ ] **Step 3: Implement `internal/hero/assembly.go`**

```go
package hero

import (
	"fmt"
	"math"

	"github.com/lowcache/volinit/internal/theme"
)

// Part is one plate of the assembly, named as the wiki's parts list names it.
type Part struct {
	Name     string
	Volatile bool // above the tmpfs datum: re-derived every boot, drawn hatched
}

// Volnix is the general-assembly stack from wiki.infernalcode.com, base first.
var Volnix = []Part{
	{"UEFI + Lanzaboote", false},
	{"CachyOS kernel", false},
	{"Hybrid graphics", false},
	{"/persist", false},
	{"tmpfs root", true},
	{"sops-nix + age", true},
	{"MicroVM gateways", true},
	{"CUDA AI stack", true},
	{"home-manager", true},
	{"niri + Noctalia v5", true},
}

// Phone is the detached sub-assembly, reached over the tailnet.
var Phone = Part{Name: "Nix-on-Droid phone tier"}

const (
	datum     = 4 // tmpfs root: the first volatile plate
	phoneJoin = 6 // MicroVM gateways, where the tailnet meets the phone

	thickRatio = 0.14 // plate thickness per unit R, from the wiki drawing
	gapRatio   = 0.46 // fully exploded gap between plates per unit R
	phoneRatio = 0.5  // phone plate size per unit R

	margin        = 2  // dots kept clear at every canvas edge
	minLabelR     = 8  // smallest plate that still carries lettering (pitch >= 4 dots)
	leaderDots    = 6  // leader length from a plate's right vertex
	labelCols     = 23 // "(10) niri + Noctalia v5"
	labelZone     = leaderDots + 2 + 2*labelCols
	phoneNameCols = 12 // "Nix-on-Droid"
	tieDots       = 8  // room for the tailnet tie between phone and stack
	hatchPitch    = 5
)

var (
	phoneName  = []string{"Nix-on-Droid", "phone tier"}
	centreLine = []int{6, 2, 1, 2} // long dash, short dash: drafting centre line
	leader     = []int{2, 2}
)

// Pose places the assembly in dot coordinates: CX is the axis, CY the
// stack's vertical centre, R the plate half-depth. Spread is 1 fully
// exploded, 0 assembled.
type Pose struct {
	CX, CY, R int
	Spread    float64
}

func thickness(r int) int { return max(2, int(math.Round(thickRatio*float64(r)))) }

func (p Pose) thick() int { return thickness(p.R) }

// pitch is the vertical distance between successive plates' centres.
func (p Pose) pitch() int {
	return p.thick() + int(math.Round(p.Spread*gapRatio*float64(p.R)))
}

// height is the stack's screen height, top vertex to bottom edge.
func (p Pose) height() int { return 2*p.R + p.thick() + (len(Volnix)-1)*p.pitch() }

func (p Pose) top() int { return p.CY - p.height()/2 }

// center is the y of plate i's top-face centre; plate 0 is the base.
func (p Pose) center(i int) int { return p.top() + p.R + (len(Volnix)-1-i)*p.pitch() }

// Lerp moves from a toward b; t is 0 at a and 1 at b.
func Lerp(a, b Pose, t float64) Pose {
	mix := func(x, y int) int { return x + int(math.Round(t*float64(y-x))) }
	return Pose{
		CX:     mix(a.CX, b.CX),
		CY:     mix(a.CY, b.CY),
		R:      mix(a.R, b.R),
		Spread: a.Spread + t*(b.Spread-a.Spread),
	}
}

// GreetingPose is the full-bleed pose: fully exploded and as large as the
// canvas allows. labels reports room for leaders, part names and the phone;
// without that room the bare stack is centred.
func GreetingPose(cols, rows int) (p Pose, labels bool) {
	w, h := 2*cols, 4*rows
	for r := h; r >= minLabelR; r-- {
		p = Pose{R: r, Spread: 1, CY: h / 2}
		if p.height() <= h-2*margin && margin+phoneZone(r)+4*r+1+labelZone+margin <= w {
			p.CX = margin + phoneZone(r) + 2*r
			return p, true
		}
	}
	return Pose{CX: w / 2, CY: h / 2, R: fit(w, h, 1), Spread: 1}, false
}

// StripPose is the assembled pose, centred in a cols×rows sidebar strip.
func StripPose(cols, rows int) Pose {
	w, h := 2*cols, 4*rows
	return Pose{CX: w / 2, CY: h / 2, R: fit(w, h, 0), Spread: 0}
}

// fit is the largest R whose stack fits a w×h dot canvas inside the margin,
// or 0 when none does.
func fit(w, h int, spread float64) int {
	for r := h; r >= 2; r-- {
		p := Pose{R: r, Spread: spread}
		if 4*r+1 <= w-2*margin && p.height() <= h-2*margin {
			return r
		}
	}
	return 0
}

func phoneR(r int) int { return max(2, int(math.Round(phoneRatio*float64(r)))) }

// phoneZone is the width left of the stack given to the phone and its tie.
func phoneZone(r int) int { return max(4*phoneR(r)+1, 2*phoneNameCols) + tieDots }

// Draw renders the assembly at p onto a fresh cols×rows canvas.
func Draw(cols, rows int, p Pose, labels bool) *Canvas {
	c := NewCanvas(cols, rows)
	if p.R < 2 {
		return c // too small to read as anything
	}
	// The axis goes down first; every plate erases it where it passes behind.
	c.Line(p.CX, p.top()-p.R/2, p.CX, p.top()+p.height()+p.R/2, RoleFaint, centreLine)
	if labels {
		// The datum runs through the gap under tmpfs root: volatile above.
		y := p.center(datum) + p.thick() + (p.pitch()-p.thick())/2
		c.Line(margin, y, p.CX+2*p.R+leaderDots, y, RoleFaint, centreLine)
	}
	for i, part := range Volnix {
		plate(c, p.CX, p.center(i), p.R, p.thick(), part.Volatile)
	}
	if labels {
		annotate(c, p)
	}
	return c
}

// Frame renders the assembly at p into exactly cols×rows coloured cells.
func Frame(cols, rows int, p Pose, labels bool, pal theme.Palette) string {
	return Draw(cols, rows, p, labels).Render(pal)
}

// plate draws one plate centred at (cx, cy): a 2:1 rhombus top face 4r wide
// over a side band t deep. It first erases its silhouette, hiding whatever
// lower plate lies behind it.
func plate(c *Canvas, cx, cy, r, t int, hatched bool) {
	for dx := -2 * r; dx <= 2*r; dx++ {
		in := (abs(dx) + 1) / 2
		for y := cy - r + in; y <= cy+r+t-in; y++ {
			c.Set(cx+dx, y, RoleNone)
		}
	}
	if hatched {
		for dx := -2*r + 2; dx <= 2*r-2; dx++ {
			in := (abs(dx)+1)/2 + 1
			for y := cy - r + in; y <= cy+r-in; y++ {
				if mod(cx+dx-y, hatchPitch) == 0 {
					c.Set(cx+dx, y, RoleFaint)
				}
			}
		}
	}
	ink := func(x0, y0, x1, y1 int) { c.Line(x0, y0, x1, y1, RoleInk, nil) }
	ink(cx, cy-r, cx+2*r, cy) // top face
	ink(cx+2*r, cy, cx, cy+r)
	ink(cx, cy+r, cx-2*r, cy)
	ink(cx-2*r, cy, cx, cy-r)
	ink(cx-2*r, cy, cx-2*r, cy+t) // side band
	ink(cx, cy+r, cx, cy+r+t)
	ink(cx+2*r, cy, cx+2*r, cy+t)
	ink(cx-2*r, cy+t, cx, cy+r+t)
	ink(cx, cy+r+t, cx+2*r, cy+t)
}

// annotate adds the drawing's lettering: a dashed leader and a numbered
// balloon per plate, and the phone tied to the gateways it joins.
func annotate(c *Canvas, p Pose) {
	x := p.CX + 2*p.R + 1
	col := (x + leaderDots + 2) / 2
	for i, part := range Volnix {
		y := p.center(i)
		c.Line(x, y, x+leaderDots, y, RoleFaint, leader)
		c.Text(col, y/4, fmt.Sprintf("(%02d) %s", i+1, part.Name), RoleInk)
	}

	pr := phoneR(p.R)
	px := margin + max(4*pr+1, 2*phoneNameCols)/2
	py := p.center(phoneJoin)
	c.Line(px+2*pr+1, py, p.CX-2*p.R-1, py, RoleFaint, leader)
	plate(c, px, py, pr, thickness(pr), Phone.Volatile)
	balloon := fmt.Sprintf("(%02d)", len(Volnix)+1)
	c.Text(px/2-len(balloon)/2, (py-pr)/4-1, balloon, RoleInk)
	for k, line := range phoneName {
		c.Text(px/2-len(line)/2, (py+pr+thickness(pr))/4+1+k, line, RoleInk)
	}
}

func mod(a, n int) int { return (a%n + n) % n }
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/hero/ -v -run . && go vet ./internal/hero/ && gofmt -l internal/hero/`
Expected: PASS. Copy the `greeting frame 240x70: ...` log line into your report — it is the evidence for dropping the disk cache.

- [ ] **Step 5: Commit**

```bash
git add internal/hero/assembly.go internal/hero/assembly_test.go
git commit -m "feat(hero): the volnix exploded assembly, isometric in braille"
```

---

### Task 6: The greeting and the sidebar strip

The morph is still a cut in this task; Task 7 adds it.

**Files:**
- Modify: `internal/runtime/model.go`
- Modify: `internal/runtime/model_test.go`

**Interfaces:**
- Consumes: `hero.GreetingPose`, `hero.StripPose`, `hero.Frame` (Task 5)
- Produces: `Model.width`; `func (m Model) stripCols() int`; constants `sidebarCols = 24`, `minStripWidth = 72`, `defaultWidth = 80`

- [ ] **Step 1: Write the failing tests**

```go
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
	m, _ := mustUpdate(t, dismiss(fixture()), tea.WindowSizeMsg{Width: 120, Height: 30})
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
```

`lipgloss` is already imported by `model.go`; add
`"charm.land/lipgloss/v2"` to the test file's imports.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/runtime/`
Expected: FAIL — `undefined: sidebarCols`.

- [ ] **Step 3: Implement in `model.go`**

Next to `defaultHeight`:

```go
const (
	defaultWidth  = 80 // stands in until the first WindowSizeMsg, like defaultHeight
	sidebarCols   = 24 // the assembled strip beside the list
	minStripWidth = 72 // below this the list gets every column
)
```

Add `width int // terminal columns, from tea.WindowSizeMsg` to `Model`
beside `height`, and `width: defaultWidth` to the literal in `New`.

In `Update`, replace the `tea.WindowSizeMsg` case body:

```go
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
```

Replace `View` and delete `viewFullBleed`:

```go
func (m Model) View() tea.View {
	var content string
	if m.stage == stageFullBleed {
		pose, labels := hero.GreetingPose(m.width, m.height)
		content = hero.Frame(m.width, m.height, pose, labels, m.palette)
	} else {
		content = m.viewSidebar()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// stripCols is the sidebar strip's width, or 0 when there is no art: at T0,
// or when the terminal is too narrow to spare the columns.
func (m Model) stripCols() int {
	if m.tier == hero.T0 || m.width < minStripWidth {
		return 0
	}
	return sidebarCols
}
```

Change `viewSidebar` to return `string`. Keep its body as-is up to and
including `b.WriteString(fg.Render(m.footer()))`, then replace its last
three lines (`v := tea.NewView(...)`, `v.AltScreen = true`, `return v`) with:

```go
	list := b.String()
	if sc := m.stripCols(); sc > 0 {
		strip := hero.Frame(sc, m.height, hero.StripPose(sc, m.height), false, m.palette)
		return lipgloss.JoinHorizontal(lipgloss.Top, strip, "  ", list)
	}
	return list
```

- [ ] **Step 4: Run tests**

Run: `go test ./... && go vet ./... && gofmt -l internal/runtime/`
Expected: PASS; gofmt prints nothing. `TestViewRendersOnlyWhatFits` must
still pass unchanged — the strip is exactly `m.height` rows.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/model.go internal/runtime/model_test.go
git commit -m "feat(runtime): greet with the assembly, keep it in a strip beside the list"
```

---

### Task 7: The morph

Dismissal closes the explosion while the assembly travels into the strip.
The morph interpolates from the greeting pose to the strip pose on one
full-width canvas; the strip sits at column 0, so its dot coordinates are
the same space and the last frame meets the sidebar exactly. Lettering and
the phone leave with the dismissal: they are the drawing's annotations,
not parts.

**Files:**
- Modify: `internal/runtime/model.go`
- Modify: `internal/runtime/model_test.go`

**Interfaces:**
- Consumes: `hero.Lerp` (Task 5), `Model.stripCols` (Task 6)
- Produces: `stageMorph`; `type morphMsg struct{}`; `morphFrames = 18`; `morphInterval = 16 * time.Millisecond`

- [ ] **Step 1: Update the existing tests that the morph changes**

Replace `dismiss` — dismissal now takes the key that starts the morph and
the key that lands it:

```go
// dismiss advances a fresh model past the greeting and the morph so a test
// can exercise the working state without exercising either.
func dismiss(m Model) Model {
	next, _ := m.Update(typed(' '))
	next, _ = next.(Model).Update(typed(' '))
	return next.(Model)
}
```

Replace `TestAnyKeyDismissesToSidebar`, `TestTheDoorIsOneWay` and
`TestDismissDoesNotAlsoAct` with:

```go
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
```

- [ ] **Step 2: Add the new tests**

```go
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
```

- [ ] **Step 3: Run and confirm failure**

Run: `go test ./internal/runtime/`
Expected: FAIL — `undefined: stageMorph`.

- [ ] **Step 4: Implement in `model.go`**

Add `"math"` and `"time"` to the imports. Replace the `stage` constants:

```go
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
```

Add `frame int // morph progress, 0..morphFrames` to `Model`.

In `Update`, add a case before `tea.KeyPressMsg`:

```go
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
```

Replace the stage block at the top of `key` (from Task 3) with:

```go
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
```

In `View`, replace the `if m.stage == stageFullBleed { ... } else { ... }`
block with:

```go
	switch m.stage {
	case stageFullBleed:
		pose, labels := hero.GreetingPose(m.width, m.height)
		content = hero.Frame(m.width, m.height, pose, labels, m.palette)
	case stageMorph:
		content = m.viewMorph()
	default:
		content = m.viewSidebar()
	}
```

Add:

```go
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
```

- [ ] **Step 5: Run tests**

Run: `go test ./... && go vet ./... && gofmt -l internal/runtime/ internal/hero/`
Expected: PASS; gofmt prints nothing.

- [ ] **Step 6: Commit**

```bash
git add internal/runtime/model.go internal/runtime/model_test.go
git commit -m "feat(runtime): the morph — the explosion closes into the strip"
```

---

## After Task 7

1. `nix flake check` must exit 0.
2. Operator review of the rendered greeting, morph and strip at a real
   terminal — the spec requires sign-off on graphics direction. Proportions
   (`thickRatio`, `gapRatio`, `hatchPitch`, `sidebarCols`, `morphFrames`)
   are the tuning knobs.
3. Record the Task 5 budget log line and the cache decision in the spec.
