// Package promptcompiler produces provider-neutral model input from durable
// task/history state, separately from how that history is stored or displayed
// (roadmapplan.md §2 critical constraint, §7.2). Provider adapters translate the
// compiled messages into provider-specific formats but never choose context.
//
// PR 8 ships the compatibility compiler: it renders the stored transcript
// unchanged so history and prompt become distinct code paths with parity. Later
// phases replace the body of Compile with typed pages and demand paging behind
// the same Compiler interface.
package promptcompiler

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/message"
)

// Section describes one compiled region for the manifest.
type Section struct {
	Kind          string `json:"kind"`
	MessageCount  int    `json:"messageCount,omitempty"`
	TokenEstimate int64  `json:"tokenEstimate"`
}

// ContextManifest records exactly what the compiler put into a prompt so it can
// be inspected and reconciled with the rendered token count.
type ContextManifest struct {
	TaskID         string    `json:"taskId,omitempty"`
	CallID         string    `json:"callId,omitempty"`
	Sections       []Section `json:"sections"`
	ToolCount      int       `json:"toolCount"`
	TokenEstimate  int64     `json:"tokenEstimate"`
	StablePrefixID string    `json:"stablePrefixId"`
}

// CompiledPrompt is the compiler's output: the messages and tool set to send,
// plus the manifest and a stable-prefix identity for cache reasoning.
type CompiledPrompt struct {
	Messages        []message.Message
	ToolSet         []tools.BaseTool
	Manifest        ContextManifest
	StablePrefixID  string
	EstimatedTokens int64
}

// Input is the compiler's request.
type Input struct {
	TaskID  string
	CallID  string
	History []message.Message
	Tools   []tools.BaseTool
}

// Compiler produces a CompiledPrompt from durable state. It is a pure function of
// its input (side effects such as page-access logging happen around it).
type Compiler interface {
	Compile(in Input) CompiledPrompt
}

// CompatibilityCompiler renders the stored transcript unchanged (parity mode).
type CompatibilityCompiler struct{}

// NewCompatibilityCompiler returns the parity-mode compiler.
func NewCompatibilityCompiler() *CompatibilityCompiler { return &CompatibilityCompiler{} }

// Compile returns the cleaned history as the prompt. Empty-part messages are
// dropped exactly as the provider adapters do, so the compiled output equals the
// legacy prompt path.
func (c *CompatibilityCompiler) Compile(in Input) CompiledPrompt {
	msgs := cleanMessages(in.History)
	est := EstimateMessages(msgs)
	prefix := stablePrefixID(in.Tools)
	return CompiledPrompt{
		Messages:       msgs,
		ToolSet:        in.Tools,
		StablePrefixID: prefix,
		Manifest: ContextManifest{
			TaskID:         in.TaskID,
			CallID:         in.CallID,
			ToolCount:      len(in.Tools),
			TokenEstimate:  est,
			StablePrefixID: prefix,
			Sections: []Section{
				{Kind: "recent_conversation", MessageCount: len(msgs), TokenEstimate: est},
			},
		},
		EstimatedTokens: est,
	}
}

// cleanMessages drops messages with no content parts, matching provider behaviour.
func cleanMessages(in []message.Message) []message.Message {
	out := make([]message.Message, 0, len(in))
	for _, m := range in {
		if len(m.Parts) == 0 {
			continue
		}
		out = append(out, m)
	}
	return out
}

// EstimateMessages returns a rough token estimate (~4 chars/token) over the text
// content of the messages. Providers report exact usage; this is for budgeting.
func EstimateMessages(msgs []message.Message) int64 {
	var chars int64
	for _, m := range msgs {
		for _, part := range m.Parts {
			switch p := part.(type) {
			case message.TextContent:
				chars += int64(len(p.Text))
			case message.ReasoningContent:
				chars += int64(len(p.Thinking))
			case message.ToolCall:
				chars += int64(len(p.Name) + len(p.Input))
			case message.ToolResult:
				chars += int64(len(p.Content))
			}
		}
	}
	return (chars + 3) / 4
}

// stablePrefixID hashes the tool set (part of the cache-stable prefix). The tool
// names are sorted so ordering does not perturb the identity.
func stablePrefixID(ts []tools.BaseTool) string {
	names := make([]string, 0, len(ts))
	for _, t := range ts {
		names = append(names, t.Info().Name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
