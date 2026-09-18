# Limoni spike — findings (2026-09-18)

Captured from the spike agent's return message: the harness blocked it from
writing its own findings file, so this is the only durable copy. Evidence
trail (built binary, `.ans` frame samples, flake, source) survives at
`~/Storage/tmp/claude/limoni-spike/` until that scratch is cleared.

## Verdict

**NO-GO on the literal question. Conditional GO on something better.**

Limoni's 3D widgets never touch its own kitty/sixel encoder. `Ascii3D` and
`Viewer3D` rasterize meshes into **Unicode glyphs plus truecolor SGR in text
cells**. Both files were read end to end: neither ever calls
`ctx.RegisterImage` or `graphics.Encode*`. The Kitty/Sixel/iTerm2 encoder is
real but wired only to `widgets.Image`, for raster photos.

So "3D mesh to kitty frames" is not a thing this library does.

## Why this is the better outcome

**Limoni is not a T3 technology. It is a T1 superpower.**

The tier model assumed 3D required kitty graphics, putting it at T3 — visible
only in one terminal. Limoni renders 3D into the character grid, which means
the exploded-assembly hero can run at **T1**: over SSH, inside tmux, on the
TTY before the graphical session, and inside `anon-shell` in the workstation
VM. Every place the spec says T1 must be beautiful on its own.

That inverts the strategy. The original plan treated T1 as the floor to make
tolerable and T3 as the ambition. Limoni makes T1 *the* ambition, and the
image tiers become an enhancement rather than the only route to spectacle.

**Revise the tier model before writing hero Tasks 3+.**

## Criteria

| # | verdict | evidence |
|---|---|---|
| 1 BUILDS | GO | `nix build` with `buildGoModule`, `vendorHash=null`, vendored, `GOTOOLCHAIN=local`, no network in-sandbox. Binary runs. |
| 2 RENDERS | API real, protocol claim false | `Ascii3D`/`Viewer3D` with `Draw(ctx cell.Context, buf *buffer.Buffer)` exist and genuinely rasterize meshes — non-blank cells, real `.ans` output. Text cells only. |
| 3 PERFORMANCE | GO, measured | Draw() alone: 216fps worst case (Braille, 4212-face mesh, 200x60) to 2492fps best, 5 combos, 200-300 samples each. Prerendering 300 frames: 198-271ms. NOT measured in the Nix sandbox or with real terminal I/O. |
| 4 COEXISTS | GO | Raw-mode and SIGWINCH handling live only in `core/driver/backend.go`, reachable only via `limoni.RunProgram`/`Terminal.New` — never through `widgets`, `graphics`, `core/buffer` or `core/cell`. `cell.Context` and `buffer.Buffer` are plain, hand-constructible, side-effect-free. It can be a pure frame producer. |
| 5 PINNABLE | mixed — pin and re-vet | ~6 weeks old, 221 commits, 19 tags, effectively one author (123 vs 6 commits), 0 real issues, CI exists. `viewer3d.go` byte-identical and `ascii3d.go`'s public surface unchanged v0.1.3 → v0.2.7, so pinning v0.2.5+ for just these two widgets is evidenced. |

## Traps to carry into implementation

* **`buffer.DiffFullStream(front, back, ...)` has front/back backwards from
  convention and silently zeroes whichever buffer is passed as "back".**
  Found empirically; cost the spike a debugging pass.
* **`go mod tidy` under `GOTOOLCHAIN=auto` bumps `go.mod` past the local
  toolchain**, risking a network toolchain fetch at build time. Pin manually.
  The flake's devShell provides go1.26.2; Limoni's real floor is go1.25.
* **The author's own commit log records previously publishing fabricated
  benchmark numbers and later correcting them.** Treat upstream performance
  claims as unverified; the numbers above were measured here, not read.
