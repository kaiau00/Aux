package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/aux-ai/aux-cli/internal/ids"
)

// Execution status values recorded by the executor.
const (
	ExecStarted   = "started"
	ExecCompleted = "completed"
	ExecError     = "error" // tool returned an error ToolResponse (not a Go error)
	ExecFailed    = "failed" // tool.Run returned a Go error (e.g. permission denied)
)

// ExecutionRecord is the observability record for a single tool run. It is
// passed to a Recorder at start and finish. It intentionally carries only
// identifiers and sizes, never the full tool output (that becomes an artifact in
// a later phase).
type ExecutionRecord struct {
	ID            string
	Correlation   Correlation
	ToolCallID    string
	ToolName      string
	InputHash     string
	Status        string
	StartedAt     int64 // unix millis
	FinishedAt    int64 // unix millis
	LatencyMS     int64
	ResponseBytes int64
	IsError       bool
	ArtifactID    string
	Metadata      string
}

// Recorder persists tool-execution lifecycle and/or emits events. It is
// dependency-inverted so the tools package stays free of db/eventstore imports.
// A nil Recorder disables recording without changing tool behaviour.
type Recorder interface {
	Start(ctx context.Context, rec ExecutionRecord)
	Finish(ctx context.Context, rec ExecutionRecord)
}

// Executor wraps every BaseTool.Run with canonical input hashing, correlation,
// timing, size measurement, and lifecycle recording. It returns exactly the
// tool's response and error so existing behaviour and permission handling are
// unchanged (roadmapplan.md §5.4).
type Executor struct {
	recorder Recorder
}

// NewExecutor returns an executor. recorder may be nil.
func NewExecutor(recorder Recorder) *Executor {
	return &Executor{recorder: recorder}
}

// Execute runs the tool, recording its lifecycle. The returned response and
// error are identical to calling tool.Run directly.
func (e *Executor) Execute(ctx context.Context, tool BaseTool, call ToolCall) (ToolResponse, error) {
	rec := ExecutionRecord{
		ID:          ids.New(),
		Correlation: CorrelationFromContext(ctx),
		ToolCallID:  call.ID,
		ToolName:    call.Name,
		InputHash:   HashToolInput(call.Input),
		Status:      ExecStarted,
		StartedAt:   time.Now().UnixMilli(),
	}
	// Expose the execution id so tools can later attach artifacts to it.
	ctx = context.WithValue(ctx, ToolExecutionIDContextKey, rec.ID)

	if e.recorder != nil {
		e.recorder.Start(ctx, rec)
	}

	start := time.Now()
	resp, err := tool.Run(ctx, call)
	rec.LatencyMS = time.Since(start).Milliseconds()
	rec.FinishedAt = time.Now().UnixMilli()
	rec.ResponseBytes = int64(len(resp.Content))
	rec.Metadata = resp.Metadata
	rec.IsError = resp.IsError || err != nil
	switch {
	case err != nil:
		rec.Status = ExecFailed
	case resp.IsError:
		rec.Status = ExecError
	default:
		rec.Status = ExecCompleted
	}

	if e.recorder != nil {
		e.recorder.Finish(ctx, rec)
	}
	return resp, err
}

// HashToolInput returns a stable SHA-256 hex digest of a tool's input. JSON
// object inputs are canonicalized (Go marshals map keys sorted) so semantically
// equal inputs with different key ordering hash identically; non-JSON inputs are
// hashed as raw bytes.
func HashToolInput(input string) string {
	canonical := []byte(input)
	var v any
	if json.Unmarshal([]byte(input), &v) == nil {
		if b, err := json.Marshal(v); err == nil {
			canonical = b
		}
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}
