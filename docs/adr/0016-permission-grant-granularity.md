# 16. Permission grant granularity

Date: 2026-08-15

## Status

Accepted.

## Context

Permission grants were cached by `(tool, action, session, directory)`. That key
works for file-editing tools, whose `Path` is the target file's directory and
therefore says something real about scope. It does not work for Bash or Fetch:
both pass the working directory, which is constant for an entire session.

The consequence was that choosing "allow for session" on a single benign
command authorized *every* later Bash command in that session, with no further
prompt and no indication that this had happened. The same was true of Fetch and
URLs. Since a prompt injection reaching the agent later in the session inherits
that grant, this turned one deliberate approval into a standing grant of
arbitrary command execution — the amplifier that made several other findings
exploitable without any further user interaction.

## Decision

Permission requests carry a `Fingerprint` identifying the specific action being
approved, and it participates in the session-grant cache key.

- Bash sets the command string.
- Fetch sets the URL.
- Out-of-working-directory reads set the canonical resolved path.
- File-editing tools deliberately leave it empty, keeping their existing
  directory-level scope.

Tools whose `Path` is effectively constant for a session **must** set a
fingerprint; that requirement is documented on the field itself, because the
failure mode is silent over-authorization rather than a visible error.

The TUI's approval button reflects the real scope ("Allow this command for
session") whenever a fingerprint is present, so what is being granted is
visible at the moment of granting.

## Consequences

More prompts in long sessions: each distinct command asks once. That is the
intended trade — the alternative is a grant whose scope the user cannot see and
did not intend.

Coarser options remain available deliberately: per-directory grants for file
edits, and `AutoApproveSession` for users who want to opt out entirely and
understand what that means.

Fingerprints are compared by exact string equality. `go test ./...` and
`go  test ./...` are different fingerprints and prompt separately. This is
accepted: normalizing commands invites a bypass where two strings that look
different to the check behave identically to the shell.
