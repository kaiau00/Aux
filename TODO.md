# Aux — Production Readiness

Rewritten 2026-08-16. Everything already shipped has been removed; see
[Appendix: what was removed](#appendix-what-was-removed) for the commits.

## Verdict

Aux is a strong late alpha. The architecture is coherent, the security posture
is real, and the test suite is race-clean. It is **not production ready**, and
the gap is mostly *evidence* rather than *code*.

Three things drive that judgment:

1. **The core claim has never been tested.** Aux's pitch is a cheaper, smarter
   harness. There is no measurement of that on real work — not weak evidence,
   none. Demand paging, arguably the centerpiece, still defaults to off because
   nothing has shown it lossless.
2. **"Built but never wired" has happened three times.** Four inert systems in
   the first audit; the skill pipeline with zero callers in the second. Each was
   found by deliberately looking, never by something failing. Three for three is
   a pattern, and it implies more.
3. **The reviews were self-reviews.** The audits and fixes came from the same
   process. Real bugs were found and tests were verified to fail against broken
   implementations — but this file's own F-caveats name this exact failure mode,
   and that discount applies here.

## Definition of done

"Production ready" means: **someone who is not the author can install Aux, use
it on their own repository for a week, and depend on the result.** Concretely,
all of P0 and P1 below are closed. This definition exists so the claim stays
falsifiable rather than becoming a vibe.

---

## P0 — Blocking

Nothing in P2 or P3 should be started before these are closed.

### P0.1 — Stand up the task benchmark (was F)

**Everything else in this file is opinion until this exists.** It also directly
gates P2.1, P2.2, and P2.3.

What exists: `aux eval compiler` (compilers over synthetic fixtures, with a
`preservesContent` losslessness check), `aux eval experiment`, and
`eval.ABStores.CompareRuns` for task-level A/B.

What does not exist: a real task suite. Building it needs API spend and a
judgment call about which tasks represent real work, so it is a decision, not a
chore:

1. Pick 20–30 tasks across genuinely different repositories, weighted toward
   ones where the agent was previously corrected — those encode what it gets
   wrong.
2. Record a baseline per task: tokens, turns, success rate, wall-clock.
3. Re-run after each change. Pass criteria: **success rate ≥ baseline (hard
   floor)**, tokens ≤ 0.7×, turns ≤ 1.1×.
4. A change that improves tokens and regresses success rate does not ship.

Keep this file's original F-caveats in view: team-written evals test what the
team thinks matters; 20–30 tasks is a thin sample; "≥ baseline" assumes the
baseline was correct; and evals never capture knowing when to stop or when to
ask. Pair the suite with a hard policy layer that is *not* subject to eval
outcomes — destructive operations always require explicit user intent, whatever
the numbers say.

### P0.2 — SQLite concurrency: fix, then measure (was A6)

No longer merely unverified. Two concrete findings:

- **No `busy_timeout` is set.** [`internal/db/connect.go:39-47`](internal/db/connect.go)
  sets `journal_mode = WAL`, `synchronous = NORMAL`, and others, but never
  `busy_timeout`. Under WAL, writers serialize on a write lock; without a busy
  timeout a blocked writer can fail immediately instead of waiting.
- **The connection pool is unbounded.** Nothing calls `SetMaxOpenConns` or
  `SetMaxIdleConns` anywhere, so Go's `database/sql` defaults apply.

The agent and the dashboard share one database, and **parallel read-only tool
execution increased the number of concurrent writers** to `tool_executions`
(the recorder writes on both `Start` and `Finish`). That change made this
exposure larger. The recorder logs write failures rather than propagating them,
so the failure mode is silently missing observability rather than corruption —
which is worse for diagnosis, not better.

Also: pragma failures are logged and execution continues
([`connect.go:48-54`](internal/db/connect.go)). If `journal_mode = WAL` ever
fails to apply, Aux runs in rollback-journal mode and nothing says so.

Work:
1. Set `busy_timeout` explicitly and bound the pool.
2. Make a failed pragma fatal, or at minimum surfaced — silent degradation of a
   durability setting is not acceptable.
3. **Then measure**: a long agent session with the dashboard open and polling,
   asserting no dropped tool-execution records and no `SQLITE_BUSY`.

Do the measurement even if the fixes look obviously right. Reasoning about
contention is what produced this gap in the first place.

### P0.3 — Systematic reachability audit

Three rounds of "built, tested, never called" is a process failure, not three
coincidences. Targeted auditing found them; targeted auditing does not scale and
will miss the next one.

1. Add `golang.org/x/tools/cmd/deadcode` (or equivalent) to CI, reporting
   functions reachable from no entry point.
2. Expect noise on a first run. Trim it to a real signal, then **fail the build**
   on new unreachable exported constructors and services.
3. Manually audit whatever the tool cannot see — a service constructed and
   stored on a struct but whose methods are never invoked looks reachable to a
   call-graph tool. That is exactly the shape of every instance found so far.

This is the single highest-leverage process fix in the file, because it converts
a recurring class of defect into a build failure.

### P0.4 — Merge PR #1

Nine milestones, three rounds of security and correctness fixes, and this
session's work all sit on `feat/phase1-runtime-foundation`. Nothing is
production anything while it is unmerged.

---

## P1 — Required before it runs on someone else's machine

### P1.1 — External review

The most important item after P0.1, and the one this file cannot self-assess.
Priority order for outside eyes:

1. **The security surface** — permission fingerprinting, read confinement, the
   dashboard's auth. All of it is days old and has never met an adversary.
   Correct-on-review is not proven.
2. **The agent loop** — `RunTurn`, the coalescing write buffer, parallel tool
   execution.
3. **One real user on a real repository for a week.** Someone who does not know
   where the sharp edges are will find things no audit will.

### P1.2 — Coverage gate in CI

`.github/workflows/test.yml` runs gofmt, `go vet`, and `go test -race ./...`.
There is no coverage measurement, against a stated 80% bar. Add
`-coverprofile`, publish the number, and fail below an agreed floor. Start the
floor at wherever the tree currently sits so it ratchets rather than blocks.

### P1.3 — Upgrade and migration safety

`goose.Up` runs on every `Connect` ([`connect.go:63`](internal/db/connect.go)).
Untested: what happens when a user upgrades with an existing database, when a
migration fails halfway, or when a newer binary opens an older database and
vice versa. Releases already ship via `.goreleaser.yml`, so users *will* upgrade
across schema changes.

Needs: a test that migrates a populated database from an older schema forward, a
clear failure message when migration fails, and a documented recovery path.

### P1.4 — Failure behaviour

- **Panic recovery.** A panic in the agent goroutine should not take down the
  TUI and lose the session.
- **Silent-failure sweep.** The pragma case above is one instance; audit for
  others where an error is logged and execution proceeds as if nothing happened.
- **First-run and misconfiguration.** No provider key, unwritable data
  directory, not a git repository — each should produce a message that says what
  to do.

### P1.5 — First-run documentation

README documents commands and keybindings well. Missing: what Aux *is*, what to
expect on a first session, what it sends to a provider, and where its data
lives. A new user's first question is "what is this about to do on my machine,"
and the security posture is a selling point that is currently undocumented.

### P1.6 — User-defined hooks: build them or drop them

The `hooks` package dispatches seven lifecycle points and has registered
observability handlers, but there is **no hook configuration in
`internal/config`** — users cannot register anything. Running commands from a
config file is arbitrary code execution and needs its own security review, which
is why it was deliberately deferred.

Decide: implement with a real threat model, or remove it from the roadmap so it
stops reading as a shipped capability.

---

## P2 — Unblocked by P0.1, high value

Ordered by value. None should start before the benchmark exists, because none
can be evaluated without it.

### P2.1 — Tool-result eviction with promotion (was B2)

Still the most valuable idea in this file, and the most dangerous. When a tool
result has been consumed, replace its full content with a one-line pointer
(`// file.go (1.2K lines) — read, no action needed`), leaving PageRank retrieval
able to re-pull it. Plausibly 40–70% off history size on long sessions.

The pairing that makes it safe: eviction must ask **"is this a fact about the
project, or about this turn?"** Facts get promoted to memory; turn-local content
is dropped. Blind eviction is how an agent forgets what it was told.

Built before P0.1 exists, there is no way to tell token savings from silent
context loss. That is the entire reason it has not been built.

### P2.2 — Decide demand paging's default

`--paging` exists and defaults to off. `aux eval compiler` measures exactly
whether paging is lossless. Run it, publish the number, flip the default on
evidence. A centerpiece feature that ships disabled is a feature that does not
exist.

### P2.3 — Skill promotion path (was B4)

Candidates are now produced automatically from validated commands, and promotion
correctly requires a passing evaluation. Nothing can currently produce that
evaluation, so no skill can ever be promoted — the pipeline terminates one step
short. P0.1 supplies the missing evidence.

Keep the original risk note in force: a wrong skill is worse than no skill, and
one that fires on every task is the bad outcome to design against.

### P2.4 — `/remember` (was C2)

`/remember always use the new auth API, not legacy` writes to `.aux/memory.md`
(project) or `~/.aux/memory.md` (user), loaded next session as Project Brain
input. Default project scope; explicit opt-in for user scope. Closes the
specific "the agent forgets what I told it" complaint that started this file,
and it is small.

### P2.5 — Memory UX (was E)

Ship before more of C, not after. Without it memory is a black box that
surprises people, which is worse than no memory:

- **Discoverable** — plain markdown, plus `aux memory list`.
- **Editable** — by hand.
- **Deletable** — "forget that", "forget session X", "wipe everything".
- **Provenance visible** — when the agent acts on a memory, show which line.
- **Portable** — export, import, sync.

### P2.6 — Memory primitive gaps (was D)

Most of the original list already exists (`promote`, `confidence`, `provenance`,
project/task scope, revision-based invalidation). Genuinely missing:

| Primitive | Gap |
| --- | --- |
| `supersede` | versions exist, no explicit supersede |
| `expire` | revision-based only, no date-based expiry |
| `scope` | project/task only — no user or org scope |

An afternoon of work, but user/org scope is a real design decision about where
memory lives and should not be inferred from the code.

---

## P3 — Opportunistic

Real, small, or low-confidence. Nothing here blocks production.

- **B3 (view half)** — smart window around the search match in `view`. The bash
  half already shipped.
- **B12 — MCP tool curation.** Worth more than its original position: a 50-tool
  MCP server costs ~15K tokens of definitions per turn. Now that MCP ordering is
  understood to drive cache stability, curation interacts with P0.1 directly.
- **B7 — PageRank as file-inclusion oracle.** Needs a <50ms p99 retriever with
  per-file invalidation first.
- **B8 — trajectory waste detection** ("you grepped this three times").
- **B9 — cost-aware tool selection** (suggest `--testPathPattern` when budget is
  low).
- **B10 — differential inclusion for edits** (send diffs, not whole files).
- **B11 — `--budget strict` preset** wiring existing governor policies.
- **C4 — correction detection.** "Propose, never auto-write" remains right; the
  heuristics are ~70% unreliable at telling one-shot from durable.
- **A8 — grep spawns `rg` per call.** Measure before optimizing.
- **C1 — four-tier context model.** A principle for reviewing the above, not a
  task: saving tokens means moving Tier 2 → Tier 3, not deleting Tier 2.

---

## Sequencing

```
P0.3 (reachability)  ─┐
P0.2 (sqlite)        ─┼─→ can run in parallel, all small
P0.4 (merge PR)      ─┘
P0.1 (benchmark)     ────────→ gates everything in P2
                                    │
P1.1 (external review) ─────────────┤ ← start as soon as PR #1 merges
P1.2–P1.6              ─────────────┤
                                    ↓
                              P2.1 … P2.6
```

P0.2, P0.3, and P0.4 are days of work. P0.1 is the long pole and the only one
that needs a decision from you about scope and spend — start it first even
though it finishes last.

If only four things get done: **P0.1, P0.2, P0.3, P1.1.** The first three make
the system measurable and stop the recurring defect class; the fourth is the
only one that corrects for everything this file cannot see about itself.

---

## Appendix: what was removed

Shipped 2026-08-16, commits `518c85c`, `d4ec855`, `a9f9a85`:

| Was | Shipped as |
| --- | --- |
| A1 | Coalescing write buffer, with the ordering guard (ADR 18) |
| A2 | Parallel read-only tool execution, capped at 4 (ADR 19) |
| A3 | Denial continues reads, cancels effectful calls |
| A7 | Bounded shell capture — the bug was `os.ReadFile`, not streaming |
| B1 | Deterministic MCP ordering; `stablePrefixID` hashes the tool block as sent |
| B5 | `context_exclude` tool, `exclude` command, agent-drop markers in the pane |
| C3 | Skill candidates extracted from validated commands at task completion |
| — | Permission prompt serialization (found as an A2 prerequisite) |

Struck as invalid after verification:

- **A4** — `Compile` is pure over in-memory history. No repo walk, no PageRank,
  no skill scan. There is no full rebuild to cache.
- **A5** — `RankFiles` is a pure function over a graph handed to it. The cadence
  question belongs to whatever builds the graph.
- **G / `internal/db/embed.go`** — six lines of `//go:embed migrations/*.sql`.
  Go's `embed` package, not vector embeddings.
- **B6** — subagents rebuild from scratch *by design*. Statelessness is the
  contract: it keeps a subagent's exploration out of the parent's context, which
  is the reason to spawn one. Not waste.
