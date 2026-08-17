// Package ids generates sortable, application-layer identifiers for Aux domain
// records. See docs/adr/0001-identifier-scheme.md.
//
// All cross-system identities (project, task, turn, model_call, tool_execution,
// event, artifact, page, memory, skill, checkpoint, experiment, …) are UUIDv7
// strings so they are time-ordered and index well, and are generated before the
// backing row exists so events and ledger entries can be correlated inside a
// single transaction.
package ids

import "github.com/google/uuid"

// New returns a fresh UUIDv7 identifier as a lowercase hyphenated string.
//
// UUIDv7 embeds a millisecond timestamp in its high bits, so lexical ordering of
// the returned strings approximates creation order. Generation cannot fail in
// practice; the rare error path falls back to a UUIDv4 rather than panicking so
// the runtime never crashes on identifier allocation.
func New() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}
