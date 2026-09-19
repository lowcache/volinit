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

---

# Addendum — hero interaction model (2026-09-18, operator-directed)

Supersedes the Hero section's implicit assumption that the greeting and the
cockpit share one layout.

## The two states and the one-way door

**State A — full bleed.** The greeting is art edge to edge. No list, no
chrome. This is the identity moment the whole project exists for, and it is
what every new terminal opens with.

**State B — sidebar + list.** The art compresses into a narrow left strip;
actions fill the remaining width at full height. This is the working state:
all 101 actions reachable, art still present but subordinate.

**The door is one-way.** A dismissal takes A to B and there is no route
back within that invocation. Not a toggle, not a mode — a progression. The
operator asked for this explicitly, and it is what stops the art becoming a
thing to flip in and out of.

**Dismissal is per-invocation.** The next terminal opens at State A again.
The greeting greets every time; the dismissal only settles the current run.

## The morph is the signature

The art does not cut, fade, or vanish on dismissal. It **compresses into
the sidebar** — the same artwork, reflowed, arriving where it will live for
the rest of the session. This transition is the single piece of animation
seen on every terminal, so it carries more of the project's identity than
the opening sequence does, and it gets the craft budget accordingly.

Constraints the morph must satisfy:

* **Interruptible.** A second keypress during the morph lands in State B
  immediately. Never make the operator wait for their own transition.
* **Degrades by tier.** T3/T2 morph the rendered art; T1 morphs the cell
  composition; T0 prints State B directly with no transition at all.
* **Frame-budgeted.** The morph replays from the same cache as the hero
  (keyed on palette hash, geometry, tier) — it is never rendered live on a
  shell-start path.

## What this changes from the original Hero section

* The "settled frame" is now State B, not a variant of the hero. Its
  quality bar is unchanged and still governing: it must be beautiful at T1,
  because SSH, tmux, the TTY and anon-shell will only ever see T1.
* The adaptive-depth rule (full sequence once per boot, settled frame
  after) applies to the OPENING sequence only. The morph is not adaptive —
  it happens on every dismissal, because it is the transition itself that
  carries the identity.
* Full-bleed means the greeting shows no workflow information. That is the
  accepted cost of the operator's "identity first" framing: the tool
  informs you one keypress later.

---

# Addendum — tier revision and the hero subject (2026-09-18, operator-directed)

Supersedes the Tiers table, the T3 row's Limoni fence rationale, and the
"any keypress dismisses" wording. Grounded in
`2026-09-18-limoni-spike-findings.md`.

## Tiers, revised

Limoni rasterizes 3D into text cells, not kitty frames. Tiers therefore name
the **output medium**, not the technology:

| tier | needs | renders |
|---|---|---|
| T2 | kitty graphics | pixel frames of the same assembly — enhancement, later plan |
| T1 | truecolor cells | **the flagship**: the exploded assembly, drawn isometric in braille cells |
| T0 | anything | plain text, no escapes |

* **T3 is removed.** Live vs replayed is a playback property, not a tier.
* **The T1 renderer is in-house, not Limoni** (operator decision). The
  subject is drafting line art: orthographic isometric plates, hairline
  edges, hatching, leaders. Limoni's widgets draw lit perspective solids;
  its wireframe is single-colour with no depth test on lines, so lower
  plates' edges would show through as the morph closes the gap, and it has
  no hatching. A small isometric braille rasterizer draws the plates bottom
  up, each erasing its own silhouette, which is exact hidden-line removal
  for convex plates stacked on one axis. Limoni stays available for a
  later rotating or lit view.
* **T2 needs its own mesh→pixel rasterizer.** Limoni's kitty/sixel encoder
  takes raster images only. T2 is enhancement, not the route to spectacle.
* **Glyph set is a T1 parameter** (braille / half-block / ascii); braille
  only for now. OPEN: the Linux VT console before the graphical session is
  unverified for braille glyphs and truecolor; do not claim TTY coverage
  until it is checked on the real console.

## The subject: volnix, exploded

The hero is the general-assembly drawing from `wiki.infernalcode.com`, in
cells: ten plates on a vertical assembly axis, signed boot chain at the base
to the niri + Noctalia shell at the top, with the phone as a detached
sub-assembly. The tmpfs root is the datum: the six plates above it are
volatile (hatched), the four below persist (solid). Plates are procedural
geometry, not a modelled asset.

The wiki's drawing language carries over, with colour taken from palette
roles rather than the wiki's fixed ink:

| wiki | palette role |
|---|---|
| ink: part edges, lettering | `on_surface` |
| hatching, dashed leaders | `outline` |
| cyan — construction (axis, datum) | `outline`, drawn as a centre line |
| magenta — the one located item | `primary`: the list caret |

The palette carries no second accent, so construction is told apart from
leaders by dash pattern, not hue. The located-item rule holds: `primary`
marks exactly one thing — the selected row — and never appears in the art. The wiki refuses the terminal-green hero; so does volinit.

## The morph in these terms (APPROVED by the operator, 2026-09-18)

Full bleed is the assembly exploded. Dismissal closes the explosion — the
plates travel together along the axis — while the assembly scales into the
sidebar strip, where it stays assembled. The same parts, arriving where they
will live.

## Cache, revised — deferred on evidence

The disk cache existed because 3D rendering was expected to be expensive.
The in-house renderer draws a frame from arithmetic over a dot grid, and a
file read may cost more than the render. The hero plan benchmarks a full
greeting frame against a hard budget; if it holds, frames render live and
the cache is not built. If it fails, the cache returns keyed on
`<palette-hash>-<cols>x<rows>-<glyphs>-<ver>`.

Measured before the plan was written, from the plan's own code: a full
240×70 greeting frame renders in 609µs against a 4ms budget. The cache is
not built.

## Greeting keys

`q`, `esc` and `ctrl+c` quit straight from State A. Every other key starts
the morph and does nothing else; a key during the morph lands in State B
immediately and is likewise consumed.

---

# Addendum — the menu (2026-09-18, operator-directed)

Supersedes the flat list and gives the **Tree** section its shape. The
operator found 101 flat actions unpalatable and chose three doors.

## Shape

Three levels: **door → subsystem → task.**

| door | subsystems |
|---|---|
| System | one per Makefile section of each system repo, in Makefile order; unsectioned targets group under the repo's name |
| Writing | one per repo, by name |
| Code | one per repo, by name; any repo not classified system or writing lands here |

* Section names show as written — no rename table to keep in sync.
* `help` targets are dropped: they print the Makefile's own list, which the
  menu replaces.
* A door with nothing in it is not shown.

## Keys

| key | door / subsystem level | task level |
|---|---|---|
| `j` `k` `↓` `↑` | move | move |
| `enter` | open | run (param and confirm gates unchanged) |
| `l` `→` | open | nothing — only `enter` runs a target |
| `esc` `h` `←` `backspace` | back; `esc` at the top quits | back |
| `q` `ctrl+c` | quit | quit |

Going back lands the cursor on the entry you came from. A breadcrumb
(`System › Secret Management`) heads every level below the doors. Doors
show a count ("9 subsystems"), subsystems a task count, tasks their
description.

## Out of scope

* Evaluating the sidecar's `when` field: it is decoded but evaluated
  nowhere, so phone-only targets still show on the laptop. Separate work.
* Tying the sidebar assembly to the menu position.
