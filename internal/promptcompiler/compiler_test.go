package promptcompiler_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/message"
	"github.com/aux-ai/aux-cli/internal/promptcompiler"
)

type fakeTool struct{ name string }

func (f fakeTool) Info() tools.ToolInfo { return tools.ToolInfo{Name: f.name} }
func (f fakeTool) Run(context.Context, tools.ToolCall) (tools.ToolResponse, error) {
	return tools.ToolResponse{}, nil
}

func history() []message.Message {
	return []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "add a feature"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{
			message.TextContent{Text: "working on it"},
			message.ToolCall{ID: "c1", Name: "grep", Input: `{"q":"x"}`},
		}},
		{Role: message.Tool, Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "c1", Content: "match"},
		}},
	}
}

func TestCompatibilityCompilerParity(t *testing.T) {
	c := promptcompiler.NewCompatibilityCompiler()
	in := history()
	out := c.Compile(promptcompiler.Input{History: in})

	// Golden: compiled messages equal the input transcript (parity mode).
	if !reflect.DeepEqual(out.Messages, in) {
		t.Fatalf("compatibility compiler must render the transcript unchanged")
	}
	if out.Manifest.TokenEstimate != out.EstimatedTokens {
		t.Fatalf("manifest token estimate must match prompt estimate")
	}
	if len(out.Manifest.Sections) == 0 {
		t.Fatalf("manifest should record at least one section")
	}
}

func TestCompilerDropsEmptyMessages(t *testing.T) {
	c := promptcompiler.NewCompatibilityCompiler()
	in := []message.Message{
		{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}},
		{Role: message.Assistant, Parts: []message.ContentPart{}}, // empty -> dropped
	}
	out := c.Compile(promptcompiler.Input{History: in})
	if len(out.Messages) != 1 {
		t.Fatalf("empty-part messages should be dropped, got %d", len(out.Messages))
	}
}

func TestCompilerPreservesToolCallResultOrder(t *testing.T) {
	c := promptcompiler.NewCompatibilityCompiler()
	in := history()
	out := c.Compile(promptcompiler.Input{History: in})

	// The assistant tool_call must precede its tool result in the compiled prompt.
	var callIdx, resultIdx = -1, -1
	for i, m := range out.Messages {
		if len(m.ToolCalls()) > 0 {
			callIdx = i
		}
		if len(m.ToolResults()) > 0 {
			resultIdx = i
		}
	}
	if callIdx == -1 || resultIdx == -1 || callIdx >= resultIdx {
		t.Fatalf("tool_call must precede tool_result: call=%d result=%d", callIdx, resultIdx)
	}
}

func TestStablePrefixDependsOnToolSetNotOrder(t *testing.T) {
	c := promptcompiler.NewCompatibilityCompiler()
	a := c.Compile(promptcompiler.Input{Tools: []tools.BaseTool{fakeTool{"grep"}, fakeTool{"edit"}}})
	b := c.Compile(promptcompiler.Input{Tools: []tools.BaseTool{fakeTool{"edit"}, fakeTool{"grep"}}})
	if a.StablePrefixID != b.StablePrefixID {
		t.Fatalf("stable prefix should be order-independent over the tool set")
	}
	d := c.Compile(promptcompiler.Input{Tools: []tools.BaseTool{fakeTool{"grep"}}})
	if a.StablePrefixID == d.StablePrefixID {
		t.Fatalf("different tool sets should produce different stable prefixes")
	}
}
