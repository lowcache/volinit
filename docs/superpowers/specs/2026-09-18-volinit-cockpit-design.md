# volinit cockpit — design

Date: 2026-09-18
Status: approved, pre-implementation

## What this is

volinit is an **identity project first**. It is the front door to every
terminal session on volnix, seen a hundred times a day, and its job is to be
a mind-bending, cutting-edge, one-of-a-kind animated TUI. Its functional
requirement is deliberately modest: be "relatively productive and workflow
direction leaning".

This framing is the tiebreaker for every scope decision below. When the
aesthetic and the feature set compete, the aesthetic wins.

**Functionally** it is a Makefile fleet browser: it shows the repositories
lowcache works on, the documented targets each exposes, and runs them.

## Why not measured as a tool

The operator already wrote every Makefile in the fleet and has the commands
in muscle memory. A menu's value is discovery; there is nothing here to
discover. Judged on efficiency this would be a slower way to type `make
anon-arm`, and judging it that way would wrongly conclude it failed. It is
the greeting that justifies it.

## Non-goals for v1

Explicitly deferred, not designed away — the schema leaves room so adding
them later is not a rewrite:

* MCP action kind and live gateway readouts
* `probe` action kind and live data widgets (cacheflow-style)
* Per-subsystem bespoke views (anon-box ladder state, VM health)
* Interactive 3D (see Hero, tier T3)

## Decisions

| # | Decision | Rationale |
|---|---|---|
| 1 | **Go**, not Nim | Nim's TUI ecosystem is thin; illwill is 8/16-colour (`ForegroundColor` enum, no RGB in 1648 lines), disqualifying for a TrueColor identity |
| 2 | **Bubble Tea v2 + Lipgloss v2** | Already implements kitty image passthrough with ID reuse, kitty keyboard protocol, mode 2026 sync and mode 2027 — the bleeding-edge checklist, done |
| 3 | Greeting **settles into the cockpit** | The thing built should be the thing seen. `greet_mode = "frame"` renders-and-exits for those who tire of it. One flag, not an architecture |
| 4 | Never **owns** the shell | No history, job control or pipes. Draws, hands back, restores the terminal |
| 5 | Registry from **Makefiles already written** | Zero new convention required of any repo |
| 6 | Discovery **never executes** | Parse statically, including `help:` recipes. Keeps shell-init cheap and safe |
| 7 | Hero is **prerendered and cached** | A greeting is a fixed animation, so it never needs real-time rendering |
| 8 | Develop **outside the volnix closure** | Do not let the TUI become a reason `make switch` fails |

## Architecture

Four packages. `registry` and the execution path are headless and testable
with no terminal.

| package | owns | knows nothing about |
|---|---|---|
| `registry` | Makefile parsing, sidecar, visibility rules → action tree | rendering, execution |
| `theme` | noctalia M3 roles → Lipgloss styles, live reload | what is being styled |
| `runtime` | Bubble Tea model/update/view, nav, focus | how actions run |
| `hero` | animation: render, cache, replay, tier detection | everything else |

Data flow: `registry` builds the tree at startup → `runtime` renders →
selection emits a `tea.Cmd` → the action runs → results return as messages.
Animation is a separate `tea.Tick` stream that never blocks on any of it.

## Registry

### Discovery

Scans `~/.nix-config` and `~/CodeRepo` (to depth 4 — the blogs live at
`sites/blogs/<name>`). Nine Makefiles exist today.

**Two Makefile dialects, one provider:**

1. `## Section` headers plus `## :target: ....: description`
   — nix-config: 9 sections, 45 targets.
2. A `help:` target whose recipe echoes `make <target>   <description>`
   — the blogs. Parsed **statically from the recipe**, never by running it.

Falls back to bare target enumeration for Makefiles using neither
(`mobile-cacheflow`, `lowcache`, `seeksascha`, `claude-companion` plugins).
Secondary providers: `nix flake show` apps, and `.volinit/actions.toml`
standing alone for repos with no Makefile.

### Tree

Three branches: **system** (nix-config), **writing** (wiki,
hotelevangelism, volnixos-blog), **code** (everything else).

### Sidecar

`<repo>/.volinit/actions.toml` annotates only exceptions. Absent, everything
still works as a plain runner.

```toml
[switch]       detach  = true
[anon-arm]     confirm = "Runs the L0-L4 ladder. Continue?"
               sudo    = true
[anon-run]     param   = { name = "CMD", prompt = "Command to jail" }
[deploy]       confirm = "Rescue-path deploy to Cloudflare Workers. Normal route is git push. Continue?"
[droid-switch] when    = "host == nix-on-droid"
```

`volinit doctor` reports sidecar entries with no matching target, and
targets matching a danger heuristic (`sudo`, `rm`, `switch`, `rekey`,
`force`, `deploy`) with no `confirm`. Make targets are not a stable API and
the sidecar will rot.

## Execution

Two paths only:

* **Short** → `tea.ExecProcess`: suspend the TUI, restore the terminal, run
  with a real TTY so `sudo` prompts work, resume.
* **`detach = true`** → `systemd-run --user --unit=volinit-<target>`, then
  follow the journal in a pager view.

Quitting the cockpit must never kill a build. `make switch` ran 4 h 10 m on
2026-09-18; a TUI owning that process would reintroduce the exact failure
`switch-detached` exists to prevent.

## Theme

Noctalia renders templates into app configs (`~/.config/noctalia/templates/`,
today only `starship-m3`), emitting Material 3 semantic roles — `primary`,
`on_primary`, `surface`, `on_surface`, `surface_variant`,
`surface_container`, `outline`, `error` — plus an 18-step neutral tonal ramp.

volinit ships a `volinit-m3` template writing `~/.config/volinit/palette.toml`.
volinit reads roles; it never hardcodes RGB. Wallpaper changes drive the
cockpit palette through the same mechanism starship and kitty already use.

## Hero

### Tiers

Detected by **querying the terminal**, never by trusting `TERM`.

| tier | needs | renders |
|---|---|---|
| T3 | kitty graphics + Limoni | live 3D exploded assembly, reacts to state |
| T2 | kitty graphics | prerendered frames of the same assembly |
| T1 | truecolor cells | half-block/braille, Lipgloss layout, animated transitions |
| T0 | anything | plain text, no escapes |

T2 is visually identical to T3 for a fixed animation. T3 buys only an
interactive hero.

**T1 must be beautiful on its own.** It is what SSH, tmux, the TTY and
`anon-shell` inside the workstation VM will show, and those are real working
contexts. If T1 is not gorgeous the project has failed regardless of the 3D.

### Cache

```
$XDG_CACHE_HOME/volinit/hero/<palette-hash>-<cols>x<rows>-<px>-<tier>-<ver>/
```

Replay is a file read plus a write to stdout.

**A cache miss never blocks the shell.** Miss → render T1 immediately and
fork a detached generator for next time. New wallpaper, resize or tier
change costs one plain-looking terminal, never a wait.

### Playback

* Wrapped in mode 2026 sync — atomic, tear-free.
* **Any keypress cuts to the settled frame.**
* Full sequence once per boot or palette change (flag in
  `$XDG_RUNTIME_DIR`); settled frame thereafter.

The **settled frame is the product** — seen ten thousand times against the
animation's twice a day.

### Limoni spike (fenced)

`hero` exposes one interface:

```go
type Renderer interface {
    Frames(p theme.Palette, g Geometry) ([]Frame, error)
}
```

Limoni appears in exactly one file. Timeboxed to an afternoon; go/no-go
written before starting: builds under `buildGoModule` with a working
`vendorHash`; emits kitty frames; holds 60 fps at real terminal geometry;
API stable enough to pin. Any miss → T2, which costs the greeting nothing.

## Risks

| risk | mitigation |
|---|---|
| Built, admired, bypassed | Accepted — it is the greeting, not the efficiency, that justifies it |
| Startup creep on every shell | Hard budget enforced by a benchmark that fails the build |
| Graphics invisible over SSH/tmux/TTY | T1 is a first-class design target, not a fallback |
| Long builds killed by quitting | `detach = true` by default for `switch`-class targets |
| Terminal left broken by sudo/panic | Save/restore correctness built and tested **first** |
| Limoni unproven, single-author, pre-1.0 | Fenced behind `Renderer`, timeboxed spike, T2 fallback |
| Adds to the surface that broke 2026-09-18 | Developed standalone; not wired into home-manager until stable |

## Success criteria

1. Opening a terminal is a pleasure, a hundred times a day.
2. Shell-init path stays within its budget, enforced by test.
3. T1 alone is beautiful.
4. Running a target from the cockpit is never worse than typing it.
5. Quitting never kills a build.
6. The terminal is always restored clean — including on panic and SIGINT.

## Build order

1. Terminal-correctness core — save/restore/suspend/resume, panic and signal safe
2. `theme` — noctalia template and palette reader
3. `registry` — both Makefile dialects, sidecar, `doctor`
4. Execution — `tea.ExecProcess` and detached paths
5. **CHECKPOINT — operator sign-off on graphics and animation direction**
6. `hero` — T1 first, then spike, then T2/T3
7. Wire into the shell, benchmark the budget
