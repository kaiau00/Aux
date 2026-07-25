package tools_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aux-ai/aux-cli/internal/llm/tools"
)

type fakeTool struct {
	name string
	resp tools.ToolResponse
	err  error
	// gotExecID captures the execution id the executor injected into the context.
	gotExecID string
}

func (f *fakeTool) Info() tools.ToolInfo { return tools.ToolInfo{Name: f.name} }

func (f *fakeTool) Run(ctx context.Context, _ tools.ToolCall) (tools.ToolResponse, error) {
	if v, ok := ctx.Value(tools.ToolExecutionIDContextKey).(string); ok {
		f.gotExecID = v
	}
	return f.resp, f.err
}

type captureRecorder struct {
	start  tools.ExecutionRecord
	finish tools.ExecutionRecord
	starts int
	fins   int
}

func (c *captureRecorder) Start(_ context.Context, rec tools.ExecutionRecord)  { c.start = rec; c.starts++ }
func (c *captureRecorder) Finish(_ context.Context, rec tools.ExecutionRecord) { c.finish = rec; c.fins++ }

func TestExecutorRecordsCompletion(t *testing.T) {
	rec := &captureRecorder{}
	ex := tools.NewExecutor(rec)
	ft := &fakeTool{name: "grep", resp: tools.NewTextResponse("hello world")}

	ctx := context.WithValue(context.Background(), tools.SessionIDContextKey, "s1")
	ctx = context.WithValue(ctx, tools.TurnIDContextKey, "t1")

	resp, err := ex.Execute(ctx, ft, tools.ToolCall{ID: "call-1", Name: "grep", Input: `{"q":"x"}`})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Legacy response returned unchanged.
	if resp.Content != "hello world" {
		t.Fatalf("response mutated: %q", resp.Content)
	}
	if rec.starts != 1 || rec.fins != 1 {
		t.Fatalf("recorder not called once each: starts=%d fins=%d", rec.starts, rec.fins)
	}
	if rec.finish.Status != tools.ExecCompleted {
		t.Fatalf("status = %q, want completed", rec.finish.Status)
	}
	if rec.finish.ResponseBytes != int64(len("hello world")) {
		t.Fatalf("response bytes = %d", rec.finish.ResponseBytes)
	}
	if rec.finish.Correlation.SessionID != "s1" || rec.finish.Correlation.TurnID != "t1" {
		t.Fatalf("correlation not captured: %+v", rec.finish.Correlation)
	}
	if rec.finish.ToolCallID != "call-1" || rec.finish.ToolName != "grep" {
		t.Fatalf("tool identity not captured: %+v", rec.finish)
	}
	if rec.finish.InputHash == "" {
		t.Fatalf("input not hashed")
	}
	if ft.gotExecID == "" || ft.gotExecID != rec.start.ID {
		t.Fatalf("execution id not propagated to tool context: got %q want %q", ft.gotExecID, rec.start.ID)
	}
}

func TestExecutorRecordsToolErrorResponse(t *testing.T) {
	rec := &captureRecorder{}
	ex := tools.NewExecutor(rec)
	ft := &fakeTool{name: "bash", resp: tools.NewTextErrorResponse("boom")}

	resp, err := ex.Execute(context.Background(), ft, tools.ToolCall{ID: "c", Name: "bash", Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected go error: %v", err)
	}
	if !resp.IsError {
		t.Fatalf("expected error response preserved")
	}
	if rec.finish.Status != tools.ExecError || !rec.finish.IsError {
		t.Fatalf("expected error status, got %q isError=%v", rec.finish.Status, rec.finish.IsError)
	}
}

func TestExecutorPassesThroughGoError(t *testing.T) {
	rec := &captureRecorder{}
	ex := tools.NewExecutor(rec)
	sentinel := errors.New("permission denied")
	ft := &fakeTool{name: "edit", err: sentinel}

	_, err := ex.Execute(context.Background(), ft, tools.ToolCall{ID: "c", Name: "edit", Input: "{}"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("go error not passed through: %v", err)
	}
	if rec.finish.Status != tools.ExecFailed {
		t.Fatalf("status = %q, want failed", rec.finish.Status)
	}
}

func TestExecutorNilRecorderRunsTool(t *testing.T) {
	ex := tools.NewExecutor(nil)
	ft := &fakeTool{name: "ls", resp: tools.NewTextResponse("ok")}
	resp, err := ex.Execute(context.Background(), ft, tools.ToolCall{ID: "c", Name: "ls", Input: "{}"})
	if err != nil || resp.Content != "ok" {
		t.Fatalf("nil recorder should still run tool: resp=%q err=%v", resp.Content, err)
	}
}

func TestHashToolInputCanonicalizesKeyOrder(t *testing.T) {
	a := tools.HashToolInput(`{"a":1,"b":2}`)
	b := tools.HashToolInput(`{"b":2,"a":1}`)
	if a != b {
		t.Fatalf("key-order-different JSON should hash equal: %s != %s", a, b)
	}
	c := tools.HashToolInput(`{"a":1,"b":3}`)
	if a == c {
		t.Fatalf("different values should hash differently")
	}
	// Non-JSON input is stable.
	if tools.HashToolInput("not json") != tools.HashToolInput("not json") {
		t.Fatalf("non-json hash unstable")
	}
}
