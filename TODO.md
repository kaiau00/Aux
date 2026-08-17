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
2. **"Built but never wired" has happened four times.** Four inert systems in
   the first audit; the skill pipeline with zero callers in the second; and the
   entire first-run welcome flow, found by the reachability gate the moment it
   was switched on (P0.3). None was found by something failing — tests pass
   perfectly well on code nothing calls. The gate now catches this class
   mechanically, which is why it was worth doing before anything in P2.
3. **The reviews were self-reviews.** The audits and fixes came from the same
   process. Real bugs were found and tests were verified to fail against broken
   implementations — but this file's own F-caveats name this exact failure mode,
   and that discount applies here.

## Definition of done

"Production ready" means: **someone who is not the author can install Aux, use
it on their own repository for a week, and depend on the result.**

Concretely: all of P0 and P1 closed, **and** a soak period afterwards in which
external findings taper off. The second half is not padding. A checklist
assembled by the people who wrote the code cannot enumerate what they cannot
see, and P1.1 exists specifically to lengthen the list — if outside review
produces no new items, the review was shallow rather than the list complete. So
the signal is not "the list is empty", it is **the rate at which the list grows
falling below the rate it is closed**.

This definition exists so the claim stays falsifiable rather than becoming a
vibe.

---

## P0 — Blocking

Nothing in P2 or P3 should be started before these are closed.

### P0.1 — Stand up the task benchmark (was F)

**The harness is built; the suite is not.** That split is deliberate — the
machinery is engineering, the task list is a judgement call that should not be
made by whoever wrote the harness.

Built (`aux eval suite`, `aux eval gate`, `internal/evalsuite`):

- Suite format with per-task pinned revisions and command-based success. A task
  with no success command, or no base revision, is rejected at load — the first
  can never fail and would inflate the success rate; the second measures a
  moving target.
- Runner that isolates each task (`reset --hard` to its revision, then `clean`),
  runs the agent non-interactively, and reads cost from the ledger rather than
  from agent output. A run whose cost cannot be read is marked unknown, never
  recorded as zero.
- **Gate with success rate as a hard floor**, evaluated before any budget: a
  candidate that halves tokens while solving fewer tasks fails. Per-task
  regressions fail even when the aggregate rate is level. An empty comparison
  never passes. Missing the token target while regressing nothing is a note, not
  a failure.

**What is needed from you: the tasks.** 20–30 across genuinely different
repositories, weighted toward ones where you previously had to correct the agent
— those encode what it gets wrong rather than what it was already good at. Mark
them `corrected: true`; the runner reports how many a suite has, and warns when
it has none. Start from `bench/suite.example.json`.

Then:

```
aux eval suite bench/suite.json --save bench/baseline.json
# make a change
aux eval suite bench/suite.json --save bench/candidate.json
aux eval gate bench/baseline.json bench/candidate.json
```

Keep the original F-caveats in view, because the harness cannot address any of
them: team-written evals test what the team thinks matters; 20–30 tasks is a
thin sample; "≥ baseline" assumes the baseline was correct; and no eval captures
knowing when to stop or when to ask. Pair the suite with a hard policy layer
that is **not** subject to eval outcomes — destructive operations always require
explicit user intent, whatever the numbers say.

One thing to know before running it: `aux -p` calls `AutoApproveSession`, so
non-interactive runs bypass every permission prompt. That is what makes
automation possible and it means benchmark repositories must be scratch
checkouts you are willing to have modified and reset.

### P0.2 — SQLite concurrency ✅ closed, and mostly a false alarm

**The version of this item written on 2026-08-16 was largely wrong, and
measuring it took about twenty minutes.** Recorded here rather than quietly
deleted, because how it was wrong matters more than the fix.

The claim was that `busy_timeout` was unset and foreign keys were therefore at
risk. Probing the driver directly showed otherwise: `ncruces/go-sqlite3`
defaults `foreign_keys` to on and `busy_timeout` to 60s **on every connection**,
and `journal_mode = WAL` is persisted in the database file rather than being
per-connection. None of the correctness-critical settings were ever at risk.

What was actually true, and is now fixed
([`internal/db/pragma.go`](internal/db/pragma.go)):

- `synchronous` and `cache_size` **are** per-connection, and a `db.Exec` against
  a pooled `*sql.DB` reaches only whichever connection serves it. Those two
  applied to one connection. Now set via the DSN so every connection carries
  them. Note the trap this creates: adding any `_pragma` to the DSN **disables**
  the driver's automatic busy timeout, so it must now be set explicitly — there
  is a test pinning that.
- The pool was unbounded and, at the `database/sql` default of 2 idle
  connections, a burst of parallel work closed and reopened connections rather
  than reusing them. Measured: 6 closures per burst at idle=2, zero at idle=8.
- Pragma failures were logged and ignored. SQLite silently accepts unknown
  pragmas and unparseable values — only a *syntax* error fails the open — so
  `verifyPragmas` now reads the critical settings back at startup and refuses to
  run a database that did not get them.

**The lesson is the reusable part.** This item was escalated to P0 on the
strength of reading code and reasoning about it, which is exactly the habit the
rest of this file warns against. Twenty minutes of measurement would have
prevented writing it. Treat every remaining unmeasured claim in this document,
including the ones that sound confident, as a hypothesis.

Still open, and genuinely unmeasured: **a long agent session with the dashboard
open and polling**, asserting no dropped `tool_executions` records under real
load. The unit tests cover a synthetic burst, not a real session. Folded into
P1.1's soak period rather than blocking.

### P0.3 — Systematic reachability audit ✅ closed, and it paid immediately

[`scripts/deadcode.sh`](scripts/deadcode.sh) runs in CI against
[`.deadcode-baseline`](.deadcode-baseline). It ratchets: the build fails when
something becomes newly unreachable, not merely because unreachable code exists.
Verified in both directions — it flags new dead code, and it flags baseline
entries that have since become reachable so the list tightens rather than rots.

**The first run found a fourth instance of the defect class**, in one command,
in seconds — where the previous three took seven parallel hand-audits:

| Dead | Size |
| --- | --- |
| `internal/welcome` | a complete first-run onboarding flow, never called |
| `chat/sidebar.go` | an entire chat component, 13 functions |
| `diff/patch.go` | a public patch-application API with no callers |

72 unreachable functions overall; 49 are design-system and provider-option
surface and are accepted in the baseline with reasons. The 23 above are recorded
as debt, **not** accepted — see P1.5 and P3.

Known limitation, unchanged: `deadcode` traces from `main` and from tests, so a
service that is constructed, stored on a struct, and never invoked still looks
reachable. That is the shape of the earlier validation and governor findings, so
this narrows the problem rather than closing it. Manual review still matters,
and the "constructed but never invoked" pattern deserves its own check
eventually.

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

### P1.5 — First-run experience: wire up what already exists

**Most of this was already built and never called.** `internal/welcome` has
`ShouldShow` (detects first boot via a flag file in the data directory),
`buildIntroBody`, and `MaybeShow` — which creates a welcome session and message
and is careful to be non-fatal so a failed welcome cannot block startup. Nothing
in `internal/app` or `cmd` invokes any of it. The reachability gate found it;
the earlier hand-audits did not.

So this item is smaller than it looked, and split in two:

1. **Wire `welcome.MaybeShow` into startup**, or delete the package. It is a
   real decision — read the intro body first and check it still describes the
   product accurately before switching it on, since it predates the security
   work and the dashboard's current shape.
2. **Then the genuinely missing documentation**: what Aux *is*, what it sends to
   a provider, and where its data lives. A new user's first question is "what is
   this about to do on my machine," and the security posture is a selling point
   that is currently undocumented.

### P1.6 — Define supported platforms

Currently undefined, and the configuration contradicts itself:

- `.goreleaser.yml` builds **linux and darwin only** (`goos: [linux, darwin]`),
  but the `archives` block carries a `goos: windows` override producing a zip —
  for a target the build never emits.
- `internal/llm/tools/shell/shell.go` is Unix-only regardless: `/dev/null`
  redirection, `syscall` process kills.

So the release config gestures at Windows, the code cannot support it, and
nothing states the real answer. This is the first thing a new user hits.

Decide and then make everything agree: drop the dead Windows archive rule and
say "macOS and Linux" in the README, or do the work to support Windows. The
former is a ten-minute change; the latter is a project.

### P1.7 — User-defined hooks: build them or drop them

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
- **Delete the dead chat sidebar.** `internal/tui/components/chat/sidebar.go` is
  an entire component (13 functions) superseded by the context pane and never
  removed. Listed as debt in `.deadcode-baseline`.
- **Delete or use `diff/patch.go`'s patch API.** `AssembleChanges`, `LoadFiles`,
  `ProcessPatch`, `ValidatePatch` and friends have no callers; the patch tool
  takes a different path. Dead code in a file-mutating package is worse than
  dead code elsewhere, because the next person to touch patching may reasonably
  assume it is the real implementation.
- **C1 — four-tier context model.** A principle for reviewing the above, not a
  task: saving tokens means moving Tier 2 → Tier 3, not deleting Tier 2.

---

## Sequencing

```
P0.2 (sqlite)          ── done
P0.3 (reachability)    ── done
P0.4 (merge PR)        ──→ ready; body updated
P0.1 (benchmark)       ────────→ gates everything in P2
                                      │
P1.1 (external review) ───────────────┤ ← start as soon as PR #1 merges
P1.2–P1.7              ───────────────┤
                                      ↓
                                P2.1 … P2.6
                                      ↓
                            soak until findings taper
```

P0.1's harness is built; what remains is the task list and the API spend to run
it, both of which are yours. P0.4 is a click.

If only two things remain: **P0.1 and P1.1.** One makes the system measurable;
the other corrects for what this file cannot see about itself. P0.2 and P0.3
both argued for that pairing — P0.2 by being wrong, P0.3 by finding in seconds
what hand-auditing had missed three times.

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
