package message

import (
	"context"
	"sync"
	"time"
)

// DefaultFlushWindow is how long a streamed message's durable write is deferred
// so that the deltas arriving during the window collapse into a single write.
//
// 50ms is short enough that a crash loses at most one window of tokens (which
// the event log can replay anyway, since SQLite is a projection of it) and long
// enough that a 50-150 tok/s stream collapses roughly 20 writes into one.
const DefaultFlushWindow = 50 * time.Millisecond

// streamWriter coalesces the durable writes produced by token-level streaming.
//
// A streaming turn calls Update once per content, thinking, and tool-call delta.
// At typical streaming rates that is thousands of synchronous SQLite writes per
// turn, every one of them rewriting the entire parts blob and blocking the
// agent's event loop. Only the final state of that blob is meaningful, so the
// writes are collapsed: the first delta arms a timer, later deltas within the
// window replace the pending value, and one write lands at the deadline.
//
// The whole parts blob is rewritten on every write, so last-write-wins is not
// merely acceptable here — it is exactly the existing semantics.
type streamWriter struct {
	// mu guards pending AND is held across the durable write itself. That is
	// deliberate: it gives all writes through this service a single total
	// order, which is what makes it impossible for a stale coalesced write to
	// land after a later direct Update (see discardLocked). SQLite serializes
	// writes regardless, so the contention this adds is not real.
	mu      sync.Mutex
	pending map[string]*pendingWrite
	window  time.Duration
	write   func(context.Context, Message) error
}

type pendingWrite struct {
	msg   Message
	timer *time.Timer
}

func newStreamWriter(window time.Duration, write func(context.Context, Message) error) *streamWriter {
	return &streamWriter{
		pending: make(map[string]*pendingWrite),
		window:  window,
		write:   write,
	}
}

// schedule records the latest state of a streaming message and ensures a write
// is armed. An already-armed timer keeps its original deadline, so a continuous
// stream flushes every window rather than never (which is what refreshing the
// deadline on each delta would do).
func (w *streamWriter) schedule(msg Message) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if p, ok := w.pending[msg.ID]; ok {
		p.msg = msg
		return
	}
	id := msg.ID
	p := &pendingWrite{msg: msg}
	p.timer = time.AfterFunc(w.window, func() { _ = w.flush(id) })
	w.pending[id] = p
}

// flush writes a pending message now, if one is still pending.
func (w *streamWriter) flush(id string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked(id)
}

// flushAll writes every pending message now.
func (w *streamWriter) flushAll() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var firstErr error
	for id := range w.pending {
		if err := w.flushLocked(id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// flushLocked performs the write while holding mu, so it cannot interleave with
// a direct Update on the same message. Callers must hold mu.
func (w *streamWriter) flushLocked(id string) error {
	p, ok := w.pending[id]
	if !ok {
		// Either nothing was pending, or a direct Update already superseded it.
		// Both mean there is nothing left to write.
		return nil
	}
	delete(w.pending, id)
	p.timer.Stop()
	// The originating request context may already be cancelled (the user hit
	// escape); the transcript still has to be persisted, so this write is not
	// tied to it.
	return w.write(context.Background(), p.msg)
}

// discardLocked drops any pending coalesced write for a message. Callers must
// hold mu and must themselves write newer content, which is why this is only
// ever called from Update.
//
// This is the ordering guard. Without it, a delta buffered 50ms ago could land
// after the finish marker written by finishMessage and silently revert the
// message to an unfinished state.
func (w *streamWriter) discardLocked(id string) {
	if p, ok := w.pending[id]; ok {
		p.timer.Stop()
		delete(w.pending, id)
	}
}
