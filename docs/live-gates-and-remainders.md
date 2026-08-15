# Live evaluation gates and deferred remainders

This document records how to run the opt-in live evaluation gates (roadmapplan.md
§9, §10.3, §16.7) and lists the work that remains, by exact plan section. It is
the "leave the harness in place and document the exact command" deliverable
for the live gates: this environment has **no provider credentials**, so the
live gates are intentionally not run here (they must never be required for
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

## Now implemented and wired (this pass)

Everything below is built, tested, **and reachable from a real user path** —
not just present as an isolated, tested package:

- **§9.7 learned policy promotion** — `internal/govpolicy`, evaluation-gated like
  skills, with rollback + evidence trail. Surfaced in the dashboard's
  Optimization view (`/optimization`).
- **§9.6 / §10.3 comparison** — `aux eval ab` (`internal/eval` A/B runner).
  Experiment history surfaced in the dashboard's Optimization view.
- **§11.1 first-mutation checkpointing** — `internal/mutationcp`, in addition to
  completion-time capture. Now also fires for subagent tasks (see §11.3 below),
  since a subagent's task lifecycle runs through the same coordinator.
- **§11.2 related-project graph** — `internal/relatedproject`, now wired into
  `internal/app/app.go` (background derivation from `go.mod` on project
  resolve), `internal/task/coordinator.go` (manifest section for related
  projects), the CLI (`aux project related`), and the dashboard's Project
  Brain view (`/project`). Previously built and tested in isolation only —
  this pass is what made it reachable.
- **§11.3 efficient subagents** — largely built this pass:
  subagents now get real task identity (linked to their parent via
  `tasks.parent_task_id`, `internal/llm/agent/agent-tool.go`), which gives
  them real per-subtask checkpointing and cost attribution "for free" through
  the existing task coordinator lifecycle (`internal/cost.TaskTotals` now
  recursively rolls up descendant tasks). Subagents support 4 specialist
  roles (repo mapper, impact analyst, validation runner, reviewer,
  `internal/llm/agent/subtask.go`) with role-specific tools/prompts, and
  report back through a structured `report` tool instead of free text.
  `SubtaskBegin`/`SubtaskEnd` hooks fire around every subagent invocation.
  **Not done**: git worktree isolation (`internal/worktree` is built and
  tested standalone, but not wired into live tool execution — every tool in
  this codebase reads a single process-global working directory, so wiring
  worktrees in safely requires making tool path resolution instance-scoped
  across ~9 tool files, a separate, larger refactor with its own risk).
  Because of that, subagent tool sets also stay read-only (plus Bash only for
  the validation-runner role) — no role gets Edit/Write, so there is
  currently nothing that would need write-set isolation anyway.
- **§11.4 multi-repo child tasks** — `internal/multirepo` compiler, now wired
  into `internal/task/coordinator.go` (`Coordinator.BeginMultiRepo`) and the
  CLI (`aux task begin --repo <path> ... `, `--auto-related`). Previously
  built and tested in isolation only.
- **§12.3 lifecycle hooks** — `internal/hooks`, dispatched at task boundaries,
  now also at subtask boundaries (§11.3).
- **§12.4 runtime adapters** — `internal/runtime` Adapter + `runtimetest`
  conformance contract.
- **§12.5 shareable bundles** — `internal/bundle` (`aux bundle export|import`);
  imports arrive as candidates.
- **§13.12 / §13.14 dashboard** — the default route (`/`) now serves the
  task-first workspace (previously the legacy session/log view); the
  decorative "Live Core" panel is gone, replaced by a real Live Activity
  panel (connection state, active-session count, last event) on the
  secondary `/sessions` view. All 6 planned dashboard views now exist:
  Tasks (`/tasks`), Project Brain (`/project`), Memory & skills (`/memory`),
  Impact graph (`/impact`), Optimization (`/optimization`), Sessions
  (`/sessions`) — previously only 2 of 6 existed.
- **§13.11 context controls** — the TUI's `x`/`u`/`c` (cross-off/un-cross/
  clear) keybindings now persist a real per-task exclusion
  (`internal/contextstore.Exclude/Include/ClearExclusions`), consulted by
  both prompt compilers on the task's next turn
  (`promptcompiler.applyExclusions`) — a real content change, not a
  display-only checkbox. An expanded, per-page context view (grouped by
  binding state) is available via a new Expand key (`e`) in the context pane.
  Narrow terminals (<80 cols) that drop the context panel can reach it again
  via a context drawer (`ctrl+g`), an overlay rather than a lost panel.
  (Note: the plan referred to this as "pin/exclude/reload," but the TUI never
  had pin or reload controls — only cross-off/un-cross/clear — so those three
  were made real rather than inventing new UI for controls that never
  existed.)
- **§13.18 accessibility** — icons fall back to ASCII when the terminal is
  detected as ASCII-only (`internal/tui/styles.SupportsUnicode`, explicit
  override via `AUX_ASCII_ICONS`). All 9 registered themes now have a
  deterministic, offline contrast test
  (`internal/tui/theme/contrast_test.go`) against the WCAG AA-normal 4.5:1
  floor for text/background — a substitute for the browser-based contrast
  tooling this environment doesn't have.
- **§13.19 visual testing** — golden fixture coverage extended to 12 states ×
  3 widths (was 9): added `permission-waiting`, `cancelled`, and
  `completed-validated` (distinct from `completed-unverified`). Added
  `TestFixturesRenderAcrossThemes`, which forces a real color profile and
  verifies every sampled theme (aux, catppuccin, dracula, tokyonight) renders
  every fixture without panicking and actually applies color, while the
  plain (color-stripped) content stays theme-independent.
- **§21 ADRs** — all 14 topics roadmapplan.md §21 lists are now recorded
  (`docs/adr/0001` through `0015`; `0003` is a bonus decision beyond the
  original 14). Previously only 2 of 14 were written.

## Genuinely remaining (need external resources)

- **§9.6 / §10.3 / §16.7 live A/B *execution*** — driving the two agent runs with
  a real provider to produce the dollar-efficiency *number*. The comparison,
  gates, and promotion that consume it are done; only the credentialed run is not
  (it cannot be verified here without spending budget).
- **§13.19 browser screenshot / contrast / SSE regression** — needs a browser.
  The TUI golden + semantic coverage (`internal/tui/visual`) is in place, the
  dashboard views are DOM/route tested, and theme contrast is now covered by
  a deterministic offline test, but pixel-level regression is still not
  runnable in this environment.
- **Full live multi-repo / cross-repo execution end to end** — the graph
  (§11.2) and multi-repo compiler (§11.4) are now wired into a real user path
  (CLI, coordinator, dashboard); exercising them across several real indexed
  repositories with a live provider is still environment-bound.

## Known, deliberately-scoped gaps (not blocked on external resources)

- **§11.3 git worktree isolation is not wired into live tool execution** —
  see above. `internal/worktree` exists and is tested; making it apply to
  real subagent file edits needs a separate refactor to make tool path
  resolution instance-scoped instead of process-global.
- **Artifact retention/compression** (ADR 0004) and **event-log
  retention** (ADR 0013) are deliberately unimplemented pending real growth
  data from actual usage — not a gap so much as a documented "measure before
  building" decision.
