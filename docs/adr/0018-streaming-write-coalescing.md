# 18. Coalescing durable writes during streaming

Date: 2026-08-16

## Status

Accepted.

## Context

Every provider event during a streaming turn — content delta, thinking delta,
tool-call start — called `messages.Update`, which marshals the message's entire
parts blob and writes it to SQLite. At 50–150 tokens per second over a thirty
second turn that is on the order of three thousand synchronous writes, each one
rewriting the whole blob and blocking the agent's event loop between tokens.

The code already acknowledged the problem: the tool-input delta case carried a
`TODO` and a 1 Hz throttle. That throttle was commented out, so tool-input deltas
were not being persisted at all.

Only the final state of the blob is meaningful. Every intermediate write is
superseded within milliseconds by the next one.

## Decision

Streaming deltas publish immediately and coalesce their durable write.

`UpdateStreamed` publishes to the pubsub broker synchronously — subscribers, and
therefore the rendered transcript, see every delta exactly as before — and
records the message as pending. The first pending write arms a 50ms timer; later
deltas within that window replace the pending value rather than arming a new
timer, so a continuous stream flushes on a cadence instead of deferring forever.

Coalescing is applied only to the token-level path. `Update` keeps its existing
write-through semantics and remains what every other caller uses.

Last-write-wins is safe here rather than merely tolerable, because the whole
parts blob is rewritten on every write. That was already true; nothing about
the storage semantics changes.

### The ordering guard

`Update` supersedes any pending coalesced write for the same message, and both
the discard and the write happen under the stream writer's lock.

This is the load-bearing part of the design. Without it a delta buffered up to
50ms earlier can land *after* the finish marker written by `finishMessage`,
reverting a completed message to an unfinished one — the UI would show a turn
that never ends, and the stored transcript would disagree with the event log.

The lock is held across the durable write itself, not just the map mutation.
Releasing it before writing reintroduces the same race in a narrower window: two
writers could both pass their checks and then land out of order. Holding it gives
every write through the service a single total order. SQLite serializes writes
regardless, so this costs nothing real.

## Consequences

A crash loses at most one flush window of tokens. This is acceptable because the
event log is the source of truth and SQLite is a projection of it; the window is
also bounded by the fact that every terminal path (`EventComplete`,
`finishMessage`, the post-stream flush) goes through `Update` or `FlushStreamed`.

`message.Service` gains two methods. Callers that do not stream are unaffected
and should keep using `Update`.

The 50ms window is a constant rather than configuration. It is short enough to be
invisible and long enough to collapse roughly twenty writes at typical streaming
rates; exposing it as a setting would invite tuning without evidence.

Because the failure this guards against is silent and rare, the ordering
invariant is covered by tests that were verified to fail against a deliberately
broken guard, not merely to pass against the correct one.
