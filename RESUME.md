# volinit — resume here

Written 2026-09-18 at the end of a session that ran out of budget. Read this
first; it is enough to start cold.

## What volinit is now

A Go terminal cockpit replacing the old Nim banner. It discovers every
documented target across the operator's repositories and runs them.

**Identity first.** It exists to be a mind-bending greeting on every shell;
being a useful Makefile launcher is the excuse, not the goal. When the
aesthetic and the feature set compete, the aesthetic wins. That framing is
the tiebreaker for every scope decision.

## State

Branch `rewrite/go-cockpit`, 26 commits. `main` still builds the Nim banner
and is what `inputs.volinit` resolves to — the operator's system is
unaffected until a merge.

    10 repos / 101 actions discovered      45+ tests, 6 packages
    nix flake check exit 0                 working tree clean

Packages: `internal/registry` (three Makefile dialects + bare-target
fallback, sidecar, doctor), `internal/theme` (noctalia M3 palette),
`internal/run` (foreground + detached commands), `internal/runtime` (Bubble
Tea cockpit), `internal/hero` (tier detection).

## Do these, in order (updated 2026-09-18, second session)

1. **Execute `docs/superpowers/plans/2026-09-18-volinit-hero-t1-art.md`**,
   Tasks 3–7. Tasks 4–5 (canvas, assembly) were compiled and tested from
   the plan text before commit: 22 hero tests green, 609µs per 240×70
   frame. Tasks 3, 6, 7 (runtime) are unverified until implemented.
2. After Task 7: `nix flake check`, then the operator reviews the greeting,
   morph and strip at a real terminal. Known tuning item: the assembled
   strip's plate seams read as a dense texture at R=10.

Done this session — do not redo:

* Hero Tasks 1+2 reviewed and approved (real base `6a7c820`; the
  `f8e0a1c` cited earlier never existed). One finding, folded into Task 3:
  `New` read the env/TTY itself, making tests invocation-dependent.
* Confirm-gate inversion shipped (`4897167`). 29 fleet actions are gated by
  the name heuristic, including newsletter-push, every deploy, sops-rekey,
  backup-force, trash. **The safety defect below is closed.**
* Spec addendum (`0e47a17`, `381e2d6`): T3 removed; the hero is the volnix
  exploded assembly from wiki.infernalcode.com; **in-house isometric braille
  renderer, not Limoni** (operator decision — Limoni draws lit perspective
  solids, no hatching, no depth-tested lines); morph approved; q/esc/ctrl+c
  quit from the greeting; disk cache dropped on measured evidence.

## Delegation policy — the expensive lesson

This session spent ~2.2M subagent tokens across ~28 dispatches and ran out
of budget before the UI existed.

* **Implementers go to `tether`** (off-cap Gemini/opencode), or the cheapest
  Claude tier. Every brief in these plans contains complete code — the work
  is transcription plus a test run, not judgment.
* **Reviewers earn a mid or high tier.** They repeatedly caught real defects
  the implementers missed, including a critical `nix flake check` failure and
  a confirm gate that guarded nothing.
* **Verify delegated facts against a compiler or the real file.** Tether
  fabricated three verifiable facts here — a function signature, a Go version
  floor, and an EVIDENCE section claiming a verification it never ran. All
  three were caught by compiling or by reading the source. Delegate freely;
  trust nothing that a tool can check for you.

## Do not

* Re-dispatch completed tasks. The ledgers list every commit; trust them and
  `git log` over recollection.
* Carry a git-discipline paragraph in dispatches. It was needed while the
  operator's Nim WIP sat uncommitted in the tree; that work was archived,
  then deleted at their instruction (recoverable ~90 days:
  `git branch nim-fallback cfe6490`). The tree is clean.
* Build anything visual beyond what the hero plan specifies without the
  operator's sign-off. They asked explicitly to approve graphics direction.

## Where everything lives

    docs/superpowers/specs/2026-09-18-volinit-cockpit-design.md
        The binding authority, including the hero-interaction addendum:
        full-bleed greeting -> one-way morph -> sidebar + list.
    docs/superpowers/plans/
        foundation.md, foundation-b.md (done), hero-t1.md (tasks 1-2 done)
    .superpowers/sdd/*/progress.md
        Three ledgers: 17 rulings with their costs, every deferred minor,
        two incidents. GITIGNORED — they survive a compaction but not a
        `git clean -fdx`. The reasoning behind every solo decision is here
        and nowhere else.
