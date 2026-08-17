# 19. Parallel execution of read-only tool calls

Date: 2026-08-16

## Status

Accepted.

## Context

Models routinely emit several independent tool calls per turn — three greps and
a view to locate something, say. Every one of them ran strictly sequentially, so
a discovery turn cost the sum of its reads when it could have cost the slowest
one. The work is IO-bound, so the serialization bought nothing.

## Decision

Consecutive read-only tool calls run concurrently, capped at four. Everything
else runs one at a time, in order, exactly as before.

### Which tools are read-only

An explicit map: `view`, `ls`, `glob`, `grep`, `sourcegraph`, `diagnostics`.

The set is not derived from a property of the tool, and a name absent from it is
treated as effectful. This matters because the same set answers two different
questions — what may run concurrently, and what survives a denial elsewhere in
the turn — and a tool nobody remembered to classify must fail closed for both.
MCP servers can introduce tool names this codebase has never seen.

`fetch` is excluded deliberately. It reads rather than writes, but it is network
egress behind a user approval prompt, which puts it with the effectful tools on
both counts.

### Result ordering

Each goroutine writes to its own index in a preallocated slice, so results reach
the model in the order it asked for them regardless of completion order. The
model must never have to reason about tool results arriving shuffled.

### Why this is safe

Concurrency here shares no mutable state:

- the executor holds no per-call state;
- the tool recorder writes through SQLite, which serializes;
- the hook registry guards its handler map and copies the handler slice before
  dispatch;
- the mutation checkpointer short-circuits on non-mutating tools before touching
  any state, and mutating tools never run in parallel.

One thing was **not** safe and had to be fixed first: `permission.Request`
published a request and blocked on a channel with nothing serializing it. Two
concurrent callers would have raced two approval dialogs into a UI that can only
show one, leaving at least one caller blocked forever. Permission prompting is
now serialized (ADR 16's grant check moved inside that window, which also
deduplicates identical concurrent requests into a single prompt).

### Denial policy

A denial no longer discards the rest of the turn. The denied call reports the
refusal, later read-only calls still run, and later effectful calls are
cancelled with a result saying why.

The previous behaviour — abort everything remaining — threw away independent
reads the model had already asked for and would simply ask for again, while the
denial's real purpose is to stop state from changing. Splitting on that
distinction preserves the stop signal where it means something.

The turn is not marked `PermissionDenied` and continues, so the model receives
the results and can reformulate. That is the point of continuing at all; a
denial that silently ends the turn teaches the model nothing.

## Consequences

Peak concurrency of four is a constant. The cap exists to avoid a wide fan-out
becoming a thundering herd against the filesystem, and models rarely emit enough
calls per turn for a higher number to matter.

A denial now leaves the turn running, where before it returned control to the
user. Users who deny in order to stop the agent should use cancellation, which
still aborts everything.

Any new read-only tool must be added to the map explicitly to get parallelism.
This is intentional friction: the failure mode of a wrong entry is a data race
or an unwanted write, and the failure mode of a missing entry is only lost
speed.
