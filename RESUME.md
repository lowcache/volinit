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

## Do these, in order

1. **Review hero tasks 1+2** — commits `d9a0d68`, `b0103d1`. Implemented and
   green but NEVER REVIEWED. Package the diff from `f8e0a1c..b0103d1` and
   dispatch a task reviewer before building on them.

2. **Limoni spike** — findings should be at
   `~/Storage/tmp/claude/limoni-spike/FINDINGS.md`. If absent, the spike died
   with the session; re-run it. It answers GO/NO-GO on render tier T3.
   The criterion that decides it is COEXISTENCE: can Limoni act as a pure
   frame producer into a buffer we own, or does it insist on driving the
   terminal Bubble Tea already owns? If it insists, T3 is dead — and T2
   (prerendered frames) loses nothing, because a greeting is a FIXED
   animation and prerendered frames of a 3D scene look identical to live 3D
   when nothing is interacting with it.

3. **Write hero Tasks 3+** — sidebar layout, cell art, frame cache, the
   morph. Deliberately unwritten: the spike changes the cache's shape.

4. **Confirm-gate inversion — OPERATOR APPROVED, designed, not built.**
   Full design in `.superpowers/sdd/2026-09-18-volinit-foundation/progress.md`
   under `OPERATOR DECISION`. It was queued only to avoid a parallel-
   implementer conflict, not because anything is unresolved.

## The safety issue, stated plainly

The cockpit runs actions on Enter. `Action.Confirm` is set only by a
`.volinit/actions.toml` sidecar, and **no sidecar exists anywhere in the
fleet**. So all 101 actions currently run ungated — including
`hotelevangelism newsletter-push` (real email to real subscribers), three
live blog `deploy`s, `sops-rekey`, `backup-force` and `trash`.

Item 4 fixes this. Until it ships, this is the most important open defect.

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
