package toolexec

import (
	"context"

	"github.com/aux-ai/aux-cli/internal/eventstore"
	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/logging"
)

// recorder implements tools.Recorder by persisting executions to the store and
// emitting tool.* domain events. Failures are logged, never propagated, so tool
// behaviour is never affected by observability.
type recorder struct {
	store  *Store
	events eventstore.Service
}

// NewRecorder returns a tools.Recorder that writes to the store and (if non-nil)
// emits tool lifecycle events.
func NewRecorder(store *Store, events eventstore.Service) tools.Recorder {
	return &recorder{store: store, events: events}
}

func (r *recorder) Start(ctx context.Context, rec tools.ExecutionRecord) {
	if r.store != nil {
		if err := r.store.Insert(ctx, rec); err != nil {
			logging.Error("failed to record tool execution start", "tool", rec.ToolName, "error", err)
		}
	}
	r.emit(eventstore.ToolStarted, rec)
}

func (r *recorder) Finish(ctx context.Context, rec tools.ExecutionRecord) {
	if r.store != nil {
		// Use a detached context so a cancelled request still records the result.
		if err := r.store.Finish(context.Background(), rec); err != nil {
			logging.Error("failed to record tool execution finish", "tool", rec.ToolName, "error", err)
		}
	}
	evType := eventstore.ToolCompleted
	if rec.IsError {
		evType = eventstore.ToolFailed
	}
	r.emit(evType, rec)
}

func (r *recorder) emit(evType eventstore.Type, rec tools.ExecutionRecord) {
	if r.events == nil {
		return
	}
	if _, err := r.events.Append(context.Background(), eventstore.Append{
		Type:      evType,
		ProjectID: rec.Correlation.ProjectID,
		SessionID: rec.Correlation.SessionID,
		TaskID:    rec.Correlation.TaskID,
		TurnID:    rec.Correlation.TurnID,
		Payload: eventstore.ToolPayload{
			ToolExecutionID: rec.ID,
			ToolCallID:      rec.ToolCallID,
			ToolName:        rec.ToolName,
			InputHash:       rec.InputHash,
			Status:          rec.Status,
			LatencyMS:       rec.LatencyMS,
			ResponseBytes:   rec.ResponseBytes,
			IsError:         rec.IsError,
			ArtifactID:      rec.ArtifactID,
		},
	}); err != nil {
		logging.Error("failed to emit tool event", "type", evType, "error", err)
	}
}
