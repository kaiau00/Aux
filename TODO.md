# AUX TODO — Perf, Tokens, Memory

Compiled from four brainstorm sessions (2026-08-15), then **verified against the
code on 2026-08-16**. The verification pass answered every open question in the
old section G, which let several items ship, corrected two that were based on
misreads, and struck two that were solving problems that do not exist.

**Status key:** ✅ shipped · ✏️ corrected · ❌ struck · ⬜ open

**Order to read this file:** Shipped → A → B → C → D → E → F → H.

---

## ✅ Shipped (2026-08-16)

Commits `518c85c` (perf/correctness) and `d4ec855` (context + skills).

| Was | What shipped |
|---|---|
| A1 | Coalescing write buffer. Deltas publish immediately; the durable write collapses into one per 50ms window. `Update` supersedes any pending write under the same lock so a buffered delta can never land after a finish marker. |
| A2 | Parallel read-only tool execution, capped at 4, order preserved. |
| A3 | Denial continue policy: later reads still run, later effectful calls are cancelled. |
| A7 ✏️ | Bounded shell output — see the correction below. |
| B1 | Prompt-cache determinism. MCP servers and their tools are sorted at assembly; `stablePrefixID` now hashes the tool block as sent, including descriptions and schemas. |
| B5 | `context_exclude` tool + `/exclude` command + agent-dropped markers in the context pane. |
| C3 | Skill candidates extracted at task completion. |
| — | Permission prompt serialization (prerequisite for A2, and a latent bug on its own). |

---

## Verification results (the old section G)

Every question in the old G list is answered. Nothing here needs re-reading.

- **`internal/skill/`** — the candidate → evaluate → promote pipeline exists and
  is correct, gated on `HasPassingEvaluation`. It had **zero production
  callers**; `memory` was wired at `coordinator.go` finalize but skills never
  were. **Now wired** (C3).
- **`internal/promptcompiler/compiler.go`** — `StablePrefixID` had no
  timestamps, but sorted names before hashing, which made it stable by
  construction and therefore blind to the reorder and description changes that
  actually invalidate a provider cache. **Fixed** (B1).
- **`internal/llm/agent/subtask.go`** — subagents get a fresh session carrying
  only the prompt. They **rebuild from scratch by design**, and the tool
  description says so. This is context isolation, not waste; see B6 below.
- **`internal/eval/`** — real, not a stub: `BaselineFixtures`,
  `CompareCompilers`, `preservesContent`, `ABStores.CompareRuns`,
  `RunCompilerExperiment`. It compares *compilers*, not *tasks*. See F.
- **`internal/memory/`** — most of the section D primitive list already exists.
  See D.
- **`internal/db/embed.go`** ❌ — six lines: `//go:embed migrations/*.sql`. It is
  Go's `embed` package for migration files, **not** vector embeddings. There is
  no embedding generation to schedule. Item struck.
- **`internal/dashboard/`** — reads the same SQLite database as the agent. See
  A6, still open.

---

## A. Perf Hot Path

### ✅ A1. Coalescing write buffer — shipped
Diagnosis was exactly right, including that the 1 Hz throttle was a band-aid.
Worth noting it was *commented out*, so tool-input deltas were not persisted at
all.

### ✅ A2. Parallel read-only tool execution — shipped
Allowlist is `view`/`ls`/`glob`/`grep`/`sourcegraph`/`diagnostics`, explicit and
failing closed: an unclassified tool is treated as effectful. `fetch` is excluded
deliberately — it reads, but it is network egress behind an approval prompt.

The brainstorm missed a hard prerequisite: `permission.Request` published a
dialog and blocked on a channel with nothing serializing it, so parallel callers
would have raced two dialogs into one surface.

### ✅ A3. Permission skip-and-continue — shipped
Shipped as "continue reads, cancel writes" rather than the proposed
trusted-directory scoping, which keeps the stop signal meaningful for state
changes without needing a trust model.

### ❌ A4. Context compiler full-rebuild — struck
`Compile` is pure over in-memory history: clean, apply exclusions, estimate,
hash, decompose pages. No repo walk, no PageRank, no skill scan. Cost is
O(history), not O(repo). An fsnotify-keyed cache would solve a problem that does
not exist.

### ❌ A5. PageRank retrieval cadence — struck as written
`RankFiles(graph, prompt, options)` is a pure function over a graph handed to it.
There is no cadence in that file. The real question — who builds the graph, and
how often — belongs to whatever calls it, and should be re-asked there.

### ⬜ A6. SQLite WAL contention — open, unverified
Confirmed the dashboard reads the same database. Connection pooling, `busy_timeout`,
and actual contention under load are still unmeasured. **This is now the largest
unverified perf claim in the file.**

### ✏️ A7. Bash output backpressure — shipped, but the mechanism was wrong
Output is not streamed through goroutines. It is redirected to temp files and
read back with a bare `os.ReadFile`, so `cat huge.log` loaded the **entire file**
into memory before the tool layer truncated it to 30KB. Worse than described and
in a different place. Capture is now bounded to 256KB of head and tail, trimmed
to valid UTF-8 boundaries.

### ⬜ A8. Grep shelling out per call — open, low priority
Unchanged. Still a cold `rg` spawn per call.

---

## B. Token Savings

### ✅ B1. Prompt cache stability — shipped
The brainstorm said "MCP is a minefield" without locating the mine. It was
`mcp-tools.go` ranging over `config.Get().MCPServers` as a Go map. With two or
more servers, tool order differed on every process start, so every restart began
on a cold cache — while `StablePrefixID` reported the prefix as unchanged.

### ⬜ B2. Tool result eviction with promotion — open, and gated on F
Still the most valuable idea in this file. Deliberately **not** built yet: it is
also the most dangerous, because without a task-level benchmark there is no way
to tell token savings from silent context loss. Build F first.

### ✅/⬜ B3. Smart truncation
Bash half shipped (A7). Note `bash.go` already had `MaxOutputLength = 30000` with
head/tail preservation, so that part was done before this file was written. The
`view` half — a smart window around the search match — is still open.

### ⬜ B4. Skill reuse across sessions — partially unblocked
C3 now produces candidates. Promotion still requires a passing evaluation, which
requires F. The risk note in the original file is correct and is why extraction
was kept deliberately narrow.

### ✅ B5. Per-page exclude controls — shipped
Confirmed the gap: there was no exclude tool at all. The model could not reach
the feature.

### ✏️ B6. Subagent context sharing — question answered, premise rejected
Subagents rebuild from scratch. The file calls this "likely the single biggest
token waste"; that is wrong. Statelessness is the contract — it is what keeps a
subagent's exploration out of the parent's context, which is the entire reason
to spawn one. The real question is whether the structured report carries enough
back, and since the `subagent.Result` work it does.

### ⬜ B7–B12 — open, unchanged
No new information. B12 (MCP tool curation) is worth more than its position
suggests now that MCP ordering is understood to affect cache.

---

## C. Memory & Persistence

### ⬜ C1. Four-tier context model — open (principle, not a task)
### ⬜ C2. Manual `/remember` command — open, still the highest-value memory item
### ✅ C3. Skill extraction at task completion — shipped
The pipeline existed and was never called. Extraction is deterministic and
narrow: a command either passed validation during the task or it does not
appear. Candidates are inert until an evaluation passes.
### ⬜ C4. Correction detection — open
The "propose, never auto-write" rule remains right.

---

## D. Memory Primitives — mostly already built

The original framing ("1-day design exercise") overestimated the work, because
`internal/memory` already has most of it:

| Primitive | Status |
|---|---|
| `promote` | ✅ `Store.Promote(ctx, memoryID, confidence)` |
| `confidence` | ✅ carried on candidates and promotion |
| `provenance` | ✅ `Sources{Type, ID, Relation}` |
| `scope` | ⚠️ project/task only — no user or org scope |
| `expire` | ⚠️ revision-based (`MarkStaleForChangedRevision`), not date-based |
| `supersede` | ❌ versions exist, no explicit supersede |

Remaining work is an afternoon of gap-filling, not a greenfield design.
Deliberately not done: nothing is blocked on it, and memory scope semantics
deserve their own decision rather than being inferred.

---

## E. Memory UX — open, unchanged

"If you ship C without E, you're shipping a black box" still holds.

---

## F. Eval Discipline — now the top of the list

**This is the most important open item.** Every remaining token and perf claim in
this file is unfalsifiable without it, and B2/B4 are actively blocked on it.

What exists: a *compiler* benchmark (`aux eval compiler`) comparing compatibility
vs paging over fixtures with a `preservesContent` losslessness check, plus
`ABStores.CompareRuns` for task-level A/B.

What does not exist: the 20–30 task suite across real projects. Standing it up
needs real API spend and real repositories, so it is a decision, not a chore:

1. Choose 20–30 tasks across real projects, weighted toward ones where the agent
   was previously corrected.
2. Baseline run — tokens, turns, success rate, latency, per task.
3. Re-run after each change. Pass criteria as originally written: success rate
   ≥ baseline (hard floor), tokens ≤ 0.7×, turns ≤ 1.1×.

The F-caveats in the original file are good and still apply — especially that
"success rate ≥ baseline" assumes the baseline was correct.

Concretely blocked on this: **demand paging still defaults to off.** That default
should flip on evidence from `aux eval compiler`, not on assumption.

---

## H. Do NOT Redo — unchanged and confirmed

Project Brain / Context OS / Cost Governor / Experience Compiler; the provider
abstraction; the subagent contract. The subagent claim in particular was verified
this pass and holds.

---

## Priority Order

1. **F — stand up the task benchmark.** Moved from 6th to 1st. B2 and B4 are
   blocked on it, paging's default is blocked on it, and it is the only thing
   that makes the shipped perf work measurable rather than merely plausible.
2. **A6 — SQLite/WAL contention audit.** The largest remaining unverified claim.
3. **C2 — `/remember`.** Closes the specific "agent forgets what I told it" pain.
4. **B2 — tool result eviction with promotion.** Highest value, but only after F.
5. **E — memory UX**, before shipping more of C.
6. **D gaps** — supersede, date-based expire, user/org scope.
7. B3 (view half), B7–B12, A8, C4 as time allows.

The original file said: "If you only do four things, do G + D + C2 + A1." G is
done, D is smaller than it looked, A1 has shipped. The remaining four are
**F + A6 + C2 + B2**, in that order — and F genuinely gates the other three.
