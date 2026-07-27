# Live evaluation gates and deferred remainders

This document records how to run the opt-in live evaluation gates (roadmapplan.md
§9, §10.3, §16.7) and lists the Phase 1.6/1.7 work that remains, by exact plan
section. It is the "leave the harness in place and document the exact command"
deliverable for the live gates: this environment has **no provider credentials**,
so the live gates are intentionally not run here (they must never be required for
normal tests or CI — the whole suite is deterministic and offline).

## Deterministic harness already in place

These run offline, with no credentials, and are covered by tests:

- `aux eval` — prompt-compiler evaluation (control vs optimized) on baseline fixtures.
- `aux eval experiment` — runs and persists the compiler experiment (compatibility vs paging); records evidence before any default changes.
- `aux eval replay <task-id>` — deterministically reconstructs a task's state from its durable events (no provider calls).
- `aux cost <task-id>` — a task's budget usage, trajectory, and waste warnings.

Evaluation-gated promotion is already enforced in code and tested: skills refuse
to promote without a passing evaluation (`skill.Service.Promote` →
`ErrNoEvaluationEvidence`), and proof-of-done never marks a criterion validated
without a passing run.

## Live gates (opt-in, require credentials + budget)

Run these only with a configured provider and an explicit opt-in, on a machine
where spending real budget is acceptable. They compare the **same preferred
model** with a capability on vs off — never per-action multi-model routing.

### §9.6 — Governed vs baseline ("accepted changes per dollar")

The cost governor ships default-off and supports `off` / `observe` / `on`
(`costGovernor.mode` in config; defaults to `off`). To measure the governor's
effect, run a fixed task set twice against the same model and compare accepted
changes per dollar (and task success rate) from the per-task ledger:

1. Baseline: set `costGovernor.mode: off`, run the task set, note each baseline task id.
2. Governed: set `costGovernor.mode: on`, run the identical task set, note each variant task id.
3. Compare: `aux eval ab <baseline-task-id> <variant-task-id>` — this computes
   accepted validated changes per dollar for both runs from durable records
   (ledger, proof-of-done, checkpoints) and reports whether the variant improved.
   It is conservative: an unknown-cost run never counts as an improvement.

The governor must not reduce task success on the fixture set for `on` to become a
default. Until then it stays `observe` (measures, no behavior change).

### §10.3 — Skill vs baseline

For a candidate skill, run its target task set with the skill inactive
(baseline) and active, on the same model, and compare success rate / cost. A
skill is promoted only when the active run beats baseline by the agreed margin —
this is the evaluation evidence `skill.Service.Promote` already requires.

### Note

The **comparison** half of the gate is implemented and tested: `aux eval ab`
computes the metric and improvement decision offline from two recorded runs, and
`govpolicy` / `skill` promotion consume that pass/fail as evidence. The only part
that needs credentials is **driving the two agent runs** with a real provider —
that turnkey `aux eval live` runner is not built here because it cannot be
verified without spending budget.

## Deferred Phase 1.6/1.7 remainders (by section)

- **§9.6 / §10.3 / §16.7 live A/B runner** — the turnkey opt-in command above;
  needs provider credentials to implement and verify.
- **§11.1 first-mutation-time checkpointing** — auto-checkpoint now runs at task
  completion (`Coordinator.captureCheckpoint`), which populates the changes
  workbench truthfully. Snapshotting at the first mutation (for mid-run rollback)
  is the remaining refinement.
- **§11.2 related-project cross-repo graph** — cross-repository dependency graph
  and related-project resolution.
- **§11.4 multi-repository child tasks** — coordinating child tasks across repos.
- **§12.3 lifecycle hooks** — the versioned local read API exists
  (`GET /api/v1/...`, token-gated, read-only); local lifecycle hooks for
  tools/plugins remain.
- **§12.4 runtime adapters** — pluggable runtime adapters.
- **§12.5 shareable / org bundles** — exportable optimization bundles.

## Deferred Phase 2 front-end (by section)

- **§13.12 / §13.14** — the active-task-first dashboard HTML shell (stage rail,
  summary row, timeline, inspector) and deep task/diff/evidence/profile/memory/
  skill/graph views. The read models and `/api/v1` endpoints that back them, plus
  the `css/`/`js/` asset-split foundation, are in place; the remaining work is
  browser markup/CSS/JS not verifiable in this environment.
- **§13.19 browser verification** — DOM/screenshot/contrast/SSE regression. The
  TUI golden-snapshot + semantic-invariant coverage is in place
  (`internal/tui/visual`); browser screenshot regression is not runnable here.
