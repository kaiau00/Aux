package eventstore

// Typed payloads for the events emitted so far. Each event type has a small,
// stable payload struct so consumers never parse free-form JSON.

// ModelCallPayload accompanies model_call.* events.
type ModelCallPayload struct {
	ModelCallID         string  `json:"modelCallId"`
	Provider            string  `json:"provider,omitempty"`
	Model               string  `json:"model,omitempty"`
	Status              string  `json:"status,omitempty"`
	InputTokens         int64   `json:"inputTokens,omitempty"`
	OutputTokens        int64   `json:"outputTokens,omitempty"`
	CacheCreationTokens int64   `json:"cacheCreationTokens,omitempty"`
	CacheReadTokens     int64   `json:"cacheReadTokens,omitempty"`
	EstimatedCost       float64 `json:"estimatedCost,omitempty"`
	CostState           string  `json:"costState,omitempty"`
	LatencyMS           int64   `json:"latencyMs,omitempty"`
	TTFTMS              int64   `json:"ttftMs,omitempty"`
	ErrorCode           string  `json:"errorCode,omitempty"`
}

// TurnPayload accompanies turn.* events.
type TurnPayload struct {
	TurnID       string `json:"turnId"`
	MessageID    string `json:"messageId,omitempty"`
	ToolCalls    int    `json:"toolCalls,omitempty"`
	FinishReason string `json:"finishReason,omitempty"`
}

// ContextPayload accompanies context.* events (what the prompt compiler produced).
type ContextPayload struct {
	CallID         string `json:"callId,omitempty"`
	MessageCount   int    `json:"messageCount,omitempty"`
	ToolCount      int    `json:"toolCount,omitempty"`
	TokenEstimate  int64  `json:"tokenEstimate,omitempty"`
	StablePrefixID string `json:"stablePrefixId,omitempty"`
}

// TaskPayload accompanies task.* events.
type TaskPayload struct {
	TaskID           string `json:"taskId"`
	Objective        string `json:"objective,omitempty"`
	Mode             string `json:"mode,omitempty"`
	Status           string `json:"status,omitempty"`
	Outcome          string `json:"outcome,omitempty"`
	ProfileVersionID string `json:"profileVersionId,omitempty"`
}

// ToolPayload accompanies tool.* events.
type ToolPayload struct {
	ToolExecutionID string `json:"toolExecutionId"`
	ToolCallID      string `json:"toolCallId,omitempty"`
	ToolName        string `json:"toolName"`
	InputHash       string `json:"inputHash,omitempty"`
	Status          string `json:"status,omitempty"`
	LatencyMS       int64  `json:"latencyMs,omitempty"`
	ResponseBytes   int64  `json:"responseBytes,omitempty"`
	IsError         bool   `json:"isError,omitempty"`
	ArtifactID      string `json:"artifactId,omitempty"`
}
