# 17. Read-access confinement for filesystem tools

Date: 2026-08-15

## Status

Accepted.

## Context

The read-only tools (`view`, `ls`, `glob`, `grep`) had no permission gate and no
path confinement. An absolute path, or a relative path containing `../..`, was
honored without any prompt. Write-facing tools (Bash, Edit, Write, Patch, Fetch)
all prompt; reads did not.

That asymmetry is backwards for this threat model. The agent's context is fed to
an external provider and rendered into the transcript and dashboard, so an
unconfined read is a direct exfiltration primitive: a prompt injection in any
file or fetched page could ask the agent to view `~/.ssh/id_rsa`,
`~/.aws/credentials`, or `~/.aux.json` — which holds provider API keys — and the
contents would flow out with no user-visible decision point.

## Decision

Filesystem reads are confined to the call's working directory, which is the
project root normally and a subagent's isolated worktree when one is active.

- Inside the working directory: allowed, never prompts. Ordinary work is
  unaffected, which matters because a gate that fires constantly gets
  approved reflexively.
- Outside: requires explicit approval, fingerprinted by the resolved path, so
  approving one file does not authorize its neighbours.

The boundary check canonicalizes both sides through `filepath.EvalSymlinks`
before comparing. A symlink living inside the tree but pointing outside it is
judged by its target, not its location — otherwise the confinement would be
trivially bypassable by anything able to create a link in the workspace.

Paths that do not exist yet resolve through their nearest existing ancestor, so
a read of a missing file is judged by where it would live.

The gate **fails closed**: with no permission service wired, or no session to
prompt in, an outside read is denied rather than allowed. A read gate that
silently disables itself when unwired is not a gate.

## Consequences

Cross-repository reads now prompt. For work that legitimately spans checkouts
this is friction, and the escape hatches are approving per path or using
`AutoApproveSession`.

Tool constructors take a `permission.Service`. `internal/llm/prompt` passes nil
when listing the working directory to build the system prompt: that read is
inside the working directory by construction, so it is unaffected by the gate,
and the nil is documented at the call site.

Confinement is enforced at the tool boundary, not the process boundary. The
Bash tool can still reach outside the working directory, as any shell can; that
is governed by Bash's own approval prompt (ADR 16) rather than by this check.
