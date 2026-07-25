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
	"fmt"
	"sort"
	"strings"

	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/message"
)

// Section describes one compiled region for the manifest.
type Section struct {
	Kind          string `json:"kind"`
	MessageCount  int    `json:"messageCount,omitempty"`
	TokenEstimate int64  `json:"tokenEstimate"`
}

// PageDescriptor describes one typed page the compiler accounted for: resident
// pages are in the prompt; available pages are known but not loaded (compat
// mode). It carries the rendered content so the caller can persist a
// content-addressed page version.
type PageDescriptor struct {
	Kind          string `json:"kind"`
	StableKey     string `json:"stableKey"`
	ContentHash   string `json:"contentHash"`
	TokenEstimate int64  `json:"tokenEstimate"`
	State         string `json:"state"` // resident | available
	Reason        string `json:"reason,omitempty"`
	Content       string `json:"-"` // not serialized into the manifest
}

// ContextManifest records exactly what the compiler put into a prompt so it can
// be inspected and reconciled with the rendered token count.
type ContextManifest struct {
	TaskID         string           `json:"taskId,omitempty"`
	CallID         string           `json:"callId,omitempty"`
	Sections       []Section        `json:"sections"`
	Pages          []PageDescriptor `json:"pages"`
	ToolCount      int              `json:"toolCount"`
	TokenEstimate  int64            `json:"tokenEstimate"`
	StablePrefixID string           `json:"stablePrefixId"`
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
	// ProjectManifest and TaskSpecText are the compiled project/task knowledge.
	// In compatibility mode they are recorded as available (not-yet-loaded)
	// pages so the manifest shows what is known but not sent.
	ProjectManifest string
	TaskSpecText    string
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
	pages := decomposePages(in, msgs)
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
			Pages:          pages,
			Sections: []Section{
				{Kind: "recent_conversation", MessageCount: len(msgs), TokenEstimate: est},
			},
		},
		EstimatedTokens: est,
	}
}

// decomposePages explains the compiled prompt page by page. Each transcript
// message becomes a resident page (a tool message becomes a tool_digest page);
// compiled project/task knowledge becomes available (not-yet-loaded) pages so
// the manifest shows what is known but not sent in compatibility mode.
func decomposePages(in Input, msgs []message.Message) []PageDescriptor {
	pages := make([]PageDescriptor, 0, len(msgs)+2)
	for i, m := range msgs {
		content := renderMessage(m)
		kind := messagePageKind(m)
		key := m.ID
		if key == "" {
			key = fmt.Sprintf("msg-%d", i)
		}
		pages = append(pages, PageDescriptor{
			Kind:          kind,
			StableKey:     "msg:" + key,
			ContentHash:   hashString(content),
			TokenEstimate: EstimateMessages([]message.Message{m}),
			State:         "resident",
			Reason:        "transcript",
			Content:       content,
		})
	}
	if in.ProjectManifest != "" {
		pages = append(pages, PageDescriptor{
			Kind: "project_manifest", StableKey: "project_manifest",
			ContentHash: hashString(in.ProjectManifest), TokenEstimate: estimateText(in.ProjectManifest),
			State: "available", Reason: "known project knowledge not loaded in compatibility mode",
			Content: in.ProjectManifest,
		})
	}
	if in.TaskSpecText != "" {
		pages = append(pages, PageDescriptor{
			Kind: "task_spec", StableKey: "task_spec:" + in.TaskID,
			ContentHash: hashString(in.TaskSpecText), TokenEstimate: estimateText(in.TaskSpecText),
			State: "available", Reason: "compiled task spec not loaded in compatibility mode",
			Content: in.TaskSpecText,
		})
	}
	return pages
}

func messagePageKind(m message.Message) string {
	if m.Role == message.Tool || len(m.ToolResults()) > 0 {
		return "tool_digest"
	}
	return "recent_conversation"
}

func renderMessage(m message.Message) string {
	var b strings.Builder
	b.WriteString(string(m.Role))
	b.WriteByte('\n')
	for _, part := range m.Parts {
		switch p := part.(type) {
		case message.TextContent:
			b.WriteString(p.Text)
		case message.ReasoningContent:
			b.WriteString(p.Thinking)
		case message.ToolCall:
			b.WriteString(p.Name)
			b.WriteString(p.Input)
		case message.ToolResult:
			b.WriteString(p.Content)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func estimateText(s string) int64 { return int64(len(s)+3) / 4 }

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
