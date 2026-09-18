# volinit Hero, Tier 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the full-bleed greeting, the sidebar working state, and the one-way morph between them — entirely in the character grid, with no image protocols.

**Architecture:** A new `internal/hero` owns art, tier detection, the frame cache and the morph. `internal/runtime` gains the two states and the one-way transition. Nothing in this plan touches image protocols; T2/T3 land in a later plan.

**Tech Stack:** Go 1.26.2 (nix-provided), `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, vendored.

**Spec:** `docs/superpowers/specs/2026-09-18-volinit-cockpit-design.md`, including the 2026-09-18 hero-interaction addendum.

## Global Constraints

- Branch `rewrite/go-cockpit`. `main` stays Nim — it is a live flake input at `~/.nix-config/home/pkgs.nix:173`.
- Deps vendored; `vendorHash = null`; the check sandbox has no network and no C compiler (`CGO_ENABLED=0` is already set in the flake's checks).
- **Never blanket-add.** `README.md`, `src/volinit.nim` and `src/volinitpkg/menu.nim` hold the author's uncommitted work. Stage by explicit path; check `git status` before every commit. Two implementers have already committed that work by accident on this branch.
- **T1 only.** No kitty graphics, no sixel, no image protocols, no Limoni. Character cells and TrueColor escapes.
- Every colour comes from `theme.Palette` roles. No hardcoded RGB anywhere.
- **The one-way door:** a dismissal takes full-bleed to sidebar and there is NO route back within an invocation. Any code path that returns to full-bleed is a defect.
- The morph must be interruptible: a keypress during it lands in the settled state immediately.
- Shell-init budget still governs. Nothing on the greeting path may render live if it can be cached.

## Why T1 carries the quality bar

SSH, tmux, the TTY before the graphical session, and `anon-shell` inside the
workstation VM will only ever see T1. If T1 is not beautiful on its own, the
project has failed regardless of what the image tiers do later. Build this as
the product, not as a fallback.

---

### Task 1: Tier detection

**Files:**
- Create: `internal/hero/tier.go`, `internal/hero/tier_test.go`

**Interfaces:**
- Produces: `type Tier int` with `T0, T1, T2, T3`; `func Detect(env func(string) string, isTTY bool) Tier`

Detection is by terminal QUERY in later plans; this plan needs only the
environment-and-TTY floor so the rest of the work has something to branch on.

- [ ] **Step 1: Write the failing test**

```go
package hero

import "testing"

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestNotATTYIsT0(t *testing.T) {
	if got := Detect(envFrom(map[string]string{"TERM": "xterm-kitty"}), false); got != T0 {
		t.Errorf("piped output must be T0, got %v", got)
	}
}

func TestDumbTerminalIsT0(t *testing.T) {
	if got := Detect(envFrom(map[string]string{"TERM": "dumb"}), true); got != T0 {
		t.Errorf("TERM=dumb must be T0, got %v", got)
	}
}

func TestTruecolorIsT1(t *testing.T) {
	env := envFrom(map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"})
	if got := Detect(env, true); got != T1 {
		t.Errorf("truecolor tty must be T1, got %v", got)
	}
}

func TestNoColortermStillT1WhenTTY(t *testing.T) {
	env := envFrom(map[string]string{"TERM": "xterm-256color"})
	if got := Detect(env, true); got != T1 {
		t.Errorf("a normal tty floors at T1, got %v", got)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/hero/ -v`
Expected: FAIL — `undefined: Detect`

- [ ] **Step 3: Implement**

```go
// Package hero owns the visual identity: tier detection, art, the frame
// cache and the morph between the greeting and the working state.
package hero

// Tier is how much the terminal can render. Higher tiers are strictly
// additive: every tier can do everything below it.
type Tier int

const (
	T0 Tier = iota // plain text, no escapes — pipes, cron, dumb terminals
	T1             // truecolor character cells
	T2             // kitty graphics: prerendered image frames
	T3             // kitty graphics + live 3D
)

// Detect returns the floor tier from environment and TTY state alone.
// T2/T3 additionally require a terminal QUERY, which happens elsewhere —
// this never promotes above T1 so that a non-graphical terminal can never
// be mistaken for a graphical one on env vars alone.
func Detect(env func(string) string, isTTY bool) Tier {
	if !isTTY {
		return T0
	}
	switch env("TERM") {
	case "", "dumb":
		return T0
	}
	return T1
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/hero/ -v`
Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/hero/tier.go internal/hero/tier_test.go
git commit -m "feat(hero): tier detection floor from env and tty state"
```

---

### Task 2: The two states and the one-way door

This task is the interaction skeleton. Art is a placeholder; the morph is a
cut. Both land in later tasks. What matters here is that the state machine
is correct and the door provably does not reopen.

**Files:**
- Modify: `internal/runtime/model.go`
- Modify: `internal/runtime/model_test.go`

**Interfaces:**
- Consumes: `hero.Tier`
- Produces: `type stage int` with `stageFullBleed`, `stageSidebar` on the model

- [ ] **Step 1: Write the failing tests**

```go
func TestStartsFullBleed(t *testing.T) {
	m := fixture()
	if m.stage != stageFullBleed {
		t.Fatal("a fresh cockpit must open at full bleed")
	}
}

func TestAnyKeyDismissesToSidebar(t *testing.T) {
	m := fixture()
	next, _ := m.Update(keyPress("x"))
	if next.(Model).stage != stageSidebar {
		t.Fatal("any key must dismiss the greeting")
	}
}

func TestTheDoorIsOneWay(t *testing.T) {
	m := fixture()
	next, _ := m.Update(keyPress("x"))
	// Every key the model handles, plus a resize, must leave us in sidebar.
	for _, k := range []string{"x", "enter", "j", "k", "esc", "y", "n", " "} {
		next, _ = next.(Model).Update(keyPress(k))
		if next.(Model).stage != stageSidebar {
			t.Fatalf("key %q reopened the greeting", k)
		}
	}
	next, _ = next.(Model).Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next.(Model).stage != stageSidebar {
		t.Fatal("a resize reopened the greeting")
	}
}

func TestFullBleedShowsNoActions(t *testing.T) {
	v := fixture().View()
	if strings.Contains(v.Content, "switch") {
		t.Fatal("the greeting must show art only, no action list")
	}
}
```

Add a `keyPress(s string) tea.KeyPressMsg` helper matching however the
existing tests construct key messages — reuse that construction exactly
rather than inventing a second form.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/runtime/ -v`
Expected: FAIL — `undefined: stageFullBleed`

- [ ] **Step 3: Implement**

Add to the model:

```go
type stage int

const (
	stageFullBleed stage = iota // the greeting: art edge to edge
	stageSidebar                // the working state: art compressed, list live
)
```

`New` starts at `stageFullBleed`. In `Update`, any `tea.KeyPressMsg` while
at `stageFullBleed` sets `stageSidebar` and consumes the key — it does not
also act on it, so dismissing never runs an action by accident. No code
path anywhere may assign `stageFullBleed` after initialisation; that is
what the one-way test pins.

`View` branches on stage: full bleed renders art only, sidebar renders the
existing list.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runtime/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/model.go internal/runtime/model_test.go
git commit -m "feat(runtime): full-bleed greeting and the one-way door to sidebar"
```

---

## Tasks 3 onward — written after the Limoni spike reports

The remaining tasks are the sidebar layout at T1, the cell-art full bleed,
the frame cache, and the morph itself. They are deliberately not written
yet: the spike determines whether the art pipeline needs to produce frames
for an image tier as well, and that changes the cache's shape and the
morph's implementation. Writing them now would be guessing.
