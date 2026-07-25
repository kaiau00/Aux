package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/cost"
	"github.com/aux-ai/aux-cli/internal/eventstore"
	"github.com/aux-ai/aux-cli/internal/ids"
	llmcontext "github.com/aux-ai/aux-cli/internal/llm/context"
	"github.com/aux-ai/aux-cli/internal/llm/models"
	"github.com/aux-ai/aux-cli/internal/llm/prompt"
	"github.com/aux-ai/aux-cli/internal/llm/provider"
	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/logging"
	"github.com/aux-ai/aux-cli/internal/message"
	"github.com/aux-ai/aux-cli/internal/permission"
	"github.com/aux-ai/aux-cli/internal/promptcompiler"
	"github.com/aux-ai/aux-cli/internal/pubsub"
	"github.com/aux-ai/aux-cli/internal/runtime"
	"github.com/aux-ai/aux-cli/internal/session"
)

// agent implements the runtime.Runner turn seam.
var _ runtime.Runner = (*agent)(nil)

// Common errors
var (
	ErrRequestCancelled = errors.New("request cancelled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
)

type AgentEventType string

const (
	AgentEventTypeError     AgentEventType = "error"
	AgentEventTypeResponse  AgentEventType = "response"
	AgentEventTypeSummarize AgentEventType = "summarize"
)

type AgentEvent struct {
	Type    AgentEventType
	Message message.Message
	Error   error

	// When summarizing
	SessionID string
	Progress  string
	Done      bool
}

type Service interface {
	pubsub.Suscriber[AgentEvent]
	Model() models.Model
	Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error)
	Cancel(sessionID string)
	IsSessionBusy(sessionID string) bool
	IsBusy() bool
	Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error)
	Summarize(ctx context.Context, sessionID string) error
}

// TaskCoordinator turns a user objective into a first-class task before tool use
// and finalizes it afterward. It is optional (only the top-level agent uses it).
type TaskCoordinator interface {
	Begin(ctx context.Context, sessionID, objective string) (context.Context, string, error)
	Finish(ctx context.Context, taskID, outcome string)
	Fail(ctx context.Context, taskID string, cause error)
}

// Deps groups the runtime services an agent needs, so wiring stays readable as
// more services are added (roadmapplan.md §3.2).
type Deps struct {
	Sessions    session.Service
	Messages    message.Service
	Ledger      cost.Service
	Events      eventstore.Service
	Recorder    tools.Recorder
	Coordinator TaskCoordinator         // optional; top-level agent only
	Compiler    promptcompiler.Compiler // optional; defaults to compatibility mode
	Virtualizer tools.Virtualizer       // optional; large tool-output virtualization
}

type agent struct {
	*pubsub.Broker[AgentEvent]
	sessions    session.Service
	messages    message.Service
	ledger      cost.Service
	events      eventstore.Service
	coordinator TaskCoordinator
	compiler    promptcompiler.Compiler
	executor    *tools.Executor

	tools    []tools.BaseTool
	provider provider.Provider

	titleProvider     provider.Provider
	summarizeProvider provider.Provider

	activeRequests sync.Map
}

func NewAgent(
	agentName config.AgentName,
	deps Deps,
	agentTools []tools.BaseTool,
) (Service, error) {
	agentProvider, err := createAgentProvider(agentName)
	if err != nil {
		return nil, err
	}
	var titleProvider provider.Provider
	// Only generate titles for the coder agent
	if agentName == config.AgentCoder {
		titleProvider, err = createAgentProvider(config.AgentTitle)
		if err != nil {
			return nil, err
		}
	}
	var summarizeProvider provider.Provider
	if agentName == config.AgentCoder {
		summarizeProvider, err = createAgentProvider(config.AgentSummarizer)
		if err != nil {
			return nil, err
		}
	}

	compiler := deps.Compiler
	if compiler == nil {
		compiler = promptcompiler.NewCompatibilityCompiler()
	}

	agent := &agent{
		Broker:            pubsub.NewBroker[AgentEvent](),
		provider:          agentProvider,
		messages:          deps.Messages,
		sessions:          deps.Sessions,
		ledger:            deps.Ledger,
		events:            deps.Events,
		coordinator:       deps.Coordinator,
		compiler:          compiler,
		executor:          tools.NewExecutor(deps.Recorder, deps.Virtualizer),
		tools:             agentTools,
		titleProvider:     titleProvider,
		summarizeProvider: summarizeProvider,
		activeRequests:    sync.Map{},
	}

	return agent, nil
}

func (a *agent) Model() models.Model {
	return a.provider.Model()
}

func (a *agent) Cancel(sessionID string) {
	// Cancel regular requests
	if cancelFunc, exists := a.activeRequests.LoadAndDelete(sessionID); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			logging.InfoPersist(fmt.Sprintf("Request cancellation initiated for session: %s", sessionID))
			cancel()
		}
	}

	// Also check for summarize requests
	if cancelFunc, exists := a.activeRequests.LoadAndDelete(sessionID + "-summarize"); exists {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			logging.InfoPersist(fmt.Sprintf("Summarize cancellation initiated for session: %s", sessionID))
			cancel()
		}
	}
}

func (a *agent) IsBusy() bool {
	busy := false
	a.activeRequests.Range(func(key, value interface{}) bool {
		if cancelFunc, ok := value.(context.CancelFunc); ok {
			if cancelFunc != nil {
				busy = true
				return false // Stop iterating
			}
		}
		return true // Continue iterating
	})
	return busy
}

func (a *agent) IsSessionBusy(sessionID string) bool {
	_, busy := a.activeRequests.Load(sessionID)
	return busy
}

func (a *agent) generateTitle(ctx context.Context, sessionID string, content string) error {
	if content == "" {
		return nil
	}
	if a.titleProvider == nil {
		return nil
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	parts := []message.ContentPart{message.TextContent{Text: content}}
	response, err := a.titleProvider.SendMessages(
		ctx,
		[]message.Message{
			{
				Role:  message.User,
				Parts: parts,
			},
		},
		make([]tools.BaseTool, 0),
	)
	if err != nil {
		return err
	}

	title := strings.TrimSpace(strings.ReplaceAll(response.Content, "\n", " "))
	if title == "" {
		return nil
	}

	session.Title = title
	_, err = a.sessions.Save(ctx, session)
	return err
}

func (a *agent) err(err error) AgentEvent {
	return AgentEvent{
		Type:  AgentEventTypeError,
		Error: err,
	}
}

func (a *agent) Run(ctx context.Context, sessionID string, content string, attachments ...message.Attachment) (<-chan AgentEvent, error) {
	if !a.provider.Model().SupportsAttachments && attachments != nil {
		attachments = nil
	}
	events := make(chan AgentEvent)
	if a.IsSessionBusy(sessionID) {
		return nil, ErrSessionBusy
	}

	genCtx, cancel := context.WithCancel(ctx)

	a.activeRequests.Store(sessionID, cancel)
	go func() {
		logging.Debug("Request started", "sessionID", sessionID)
		defer logging.RecoverPanic("agent.Run", func() {
			events <- a.err(fmt.Errorf("panic while running the agent"))
		})
		var attachmentParts []message.ContentPart
		for _, attachment := range attachments {
			attachmentParts = append(attachmentParts, message.BinaryContent{Path: attachment.FilePath, MIMEType: attachment.MimeType, Data: attachment.Content})
		}
		result := a.processGeneration(genCtx, sessionID, content, attachmentParts)
		if result.Error != nil && !errors.Is(result.Error, ErrRequestCancelled) && !errors.Is(result.Error, context.Canceled) {
			logging.ErrorPersist(result.Error.Error())
		}
		logging.Debug("Request completed", "sessionID", sessionID)
		a.activeRequests.Delete(sessionID)
		cancel()
		a.Publish(pubsub.CreatedEvent, result)
		events <- result
		close(events)
	}()
	return events, nil
}

func (a *agent) processGeneration(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) (result AgentEvent) {
	cfg := config.Get()

	// Turn this user objective into a first-class, versioned task bound to the
	// current project revision and effective profile, before any tool runs. The
	// task is finalized (completed/failed/cancelled) when generation returns.
	var taskID string
	if a.coordinator != nil {
		if taskCtx, id, err := a.coordinator.Begin(ctx, sessionID, content); err != nil {
			logging.Warn("failed to begin task", "error", err)
		} else {
			ctx = taskCtx
			taskID = id
			defer func() {
				if result.Error != nil {
					a.coordinator.Fail(ctx, taskID, result.Error)
				} else {
					a.coordinator.Finish(ctx, taskID, "completed")
				}
			}()
		}
	}

	// List existing messages; if none, start title generation asynchronously.
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to list messages: %w", err))
	}
	if len(msgs) == 0 {
		go func() {
			defer logging.RecoverPanic("agent.Run", func() {
				logging.ErrorPersist("panic while generating title")
			})
			titleErr := a.generateTitle(context.Background(), sessionID, content)
			if titleErr != nil {
				logging.ErrorPersist(fmt.Sprintf("failed to generate title: %v", titleErr))
			}
		}()
	}
	session, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return a.err(fmt.Errorf("failed to get session: %w", err))
	}
	if session.SummaryMessageID != "" {
		summaryMsgInex := -1
		for i, msg := range msgs {
			if msg.ID == session.SummaryMessageID {
				summaryMsgInex = i
				break
			}
		}
		if summaryMsgInex != -1 {
			msgs = msgs[summaryMsgInex:]
			msgs[0].Role = message.User
		}
	}

	userMsg, err := a.createUserMessage(ctx, sessionID, content, attachmentParts)
	if err != nil {
		return a.err(fmt.Errorf("failed to create user message: %w", err))
	}
	ctx = llmcontext.WithState(ctx, llmcontext.NewState(content))
	// Append the new user message to the conversation history.
	msgHistory := append(msgs, userMsg)

	for {
		// Check for cancellation before each iteration
		select {
		case <-ctx.Done():
			return a.err(ctx.Err())
		default:
			// Continue processing
		}
		turn, err := a.RunTurn(ctx, sessionID, msgHistory)
		agentMessage, toolResults := turn.Assistant, turn.ToolResults
		if err != nil {
			if errors.Is(err, context.Canceled) {
				agentMessage.AddFinish(message.FinishReasonCanceled)
				a.messages.Update(context.Background(), agentMessage)
				return a.err(ErrRequestCancelled)
			}
			return a.err(fmt.Errorf("failed to process events: %w", err))
		}
		if cfg.Debug {
			seqId := (len(msgHistory) + 1) / 2
			toolResultFilepath := logging.WriteToolResultsJson(sessionID, seqId, toolResults)
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", "{}", "filepath", toolResultFilepath)
		} else {
			logging.Info("Result", "message", agentMessage.FinishReason(), "toolResults", toolResults)
		}
		if (agentMessage.FinishReason() == message.FinishReasonToolUse) && toolResults != nil {
			// We are not done, we need to respond with the tool response
			msgHistory = append(msgHistory, agentMessage, *toolResults)
			continue
		}
		return AgentEvent{
			Type:    AgentEventTypeResponse,
			Message: agentMessage,
			Done:    true,
		}
	}
}

func (a *agent) createUserMessage(ctx context.Context, sessionID, content string, attachmentParts []message.ContentPart) (message.Message, error) {
	parts := []message.ContentPart{message.TextContent{Text: content}}
	parts = append(parts, attachmentParts...)
	return a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: parts,
	})
}

// RunTurn executes one turn (one model call plus its tool results) behind the
// runtime.Runner seam. In compatibility mode it passes the stored history to the
// provider unchanged; later phases substitute the prompt compiler here.
func (a *agent) RunTurn(ctx context.Context, sessionID string, history []message.Message) (runtime.TurnResult, error) {
	assistant, toolResults, err := a.streamAndHandleEvents(ctx, sessionID, history)
	return runtime.TurnResult{Assistant: assistant, ToolResults: toolResults}, err
}

func (a *agent) streamAndHandleEvents(ctx context.Context, sessionID string, msgHistory []message.Message) (message.Message, *message.Message, error) {
	// A turn is one iteration of the agent loop: one model call plus the tool
	// results it produced. Correlation IDs let every downstream record (model
	// call, tool execution, event) be reconstructed without parsing message JSON.
	turnID := ids.New()
	ctx = context.WithValue(ctx, tools.SessionIDContextKey, sessionID)
	ctx = context.WithValue(ctx, tools.TurnIDContextKey, turnID)
	a.emit(ctx, eventstore.Append{
		Type:    eventstore.TurnStarted,
		TurnID:  turnID,
		Payload: eventstore.TurnPayload{TurnID: turnID},
	})

	assistantMsg, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{},
		Model: a.provider.Model().ID,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create assistant message: %w", err)
	}

	// Add the session and message ID into the context if needed by tools.
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, assistantMsg.ID)

	// Open the per-call ledger record and expose its id to tools for correlation.
	tracker := a.startCall(ctx, sessionID, turnID, assistantMsg.ID)
	ctx = context.WithValue(ctx, tools.ModelCallIDContextKey, tracker.id)

	// Compile the model-facing prompt from durable history, separately from how
	// that history is stored/displayed. In compatibility mode the compiled
	// messages equal the cleaned transcript, so behaviour is unchanged; the
	// manifest is recorded for inspection and reconciliation.
	corr := tools.CorrelationFromContext(ctx)
	compiled := a.compiler.Compile(promptcompiler.Input{
		TaskID:  corr.TaskID,
		CallID:  tracker.id,
		History: msgHistory,
		Tools:   a.tools,
	})
	a.emit(ctx, eventstore.Append{
		Type:   eventstore.ContextCompiled,
		TurnID: turnID,
		Payload: eventstore.ContextPayload{
			CallID:         tracker.id,
			MessageCount:   len(compiled.Messages),
			ToolCount:      compiled.Manifest.ToolCount,
			TokenEstimate:  compiled.EstimatedTokens,
			StablePrefixID: compiled.StablePrefixID,
		},
	})
	eventChan := a.provider.StreamResponse(ctx, compiled.Messages, compiled.ToolSet)

	// Process each event in the stream.
	for event := range eventChan {
		if processErr := a.processEvent(ctx, sessionID, tracker, &assistantMsg, event); processErr != nil {
			a.finishMessage(ctx, &assistantMsg, message.FinishReasonCanceled)
			a.abortCall(ctx, tracker, processErr)
			return assistantMsg, nil, processErr
		}
		if ctx.Err() != nil {
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			a.abortCall(ctx, tracker, ctx.Err())
			return assistantMsg, nil, ctx.Err()
		}
	}
	// Safety net: if the stream closed without an explicit completion event, close
	// the ledger record so it never remains in the `started` state.
	a.finalizeCallIfOpen(context.Background(), tracker, sessionID)

	toolResults := make([]message.ToolResult, len(assistantMsg.ToolCalls()))
	toolCalls := assistantMsg.ToolCalls()
	for i, toolCall := range toolCalls {
		select {
		case <-ctx.Done():
			a.finishMessage(context.Background(), &assistantMsg, message.FinishReasonCanceled)
			// Make all future tool calls cancelled
			for j := i; j < len(toolCalls); j++ {
				toolResults[j] = message.ToolResult{
					ToolCallID: toolCalls[j].ID,
					Content:    "Tool execution canceled by user",
					IsError:    true,
				}
			}
			goto out
		default:
			// Continue processing
			var tool tools.BaseTool
			for _, availableTool := range a.tools {
				if availableTool.Info().Name == toolCall.Name {
					tool = availableTool
					break
				}
				// Monkey patch for Copilot Sonnet-4 tool repetition obfuscation
				// if strings.HasPrefix(toolCall.Name, availableTool.Info().Name) &&
				// 	strings.HasPrefix(toolCall.Name, availableTool.Info().Name+availableTool.Info().Name) {
				// 	tool = availableTool
				// 	break
				// }
			}

			// Tool not found
			if tool == nil {
				toolResults[i] = message.ToolResult{
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("Tool not found: %s", toolCall.Name),
					IsError:    true,
				}
				continue
			}
			toolResult, toolErr := a.executor.Execute(ctx, tool, tools.ToolCall{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Input: toolCall.Input,
			})
			if toolErr != nil {
				if errors.Is(toolErr, permission.ErrorPermissionDenied) {
					toolResults[i] = message.ToolResult{
						ToolCallID: toolCall.ID,
						Content:    "Permission denied",
						IsError:    true,
					}
					for j := i + 1; j < len(toolCalls); j++ {
						toolResults[j] = message.ToolResult{
							ToolCallID: toolCalls[j].ID,
							Content:    "Tool execution canceled by user",
							IsError:    true,
						}
					}
					a.finishMessage(ctx, &assistantMsg, message.FinishReasonPermissionDenied)
					break
				}
			}
			toolResults[i] = message.ToolResult{
				ToolCallID: toolCall.ID,
				Content:    toolResult.Content,
				Metadata:   toolResult.Metadata,
				IsError:    toolResult.IsError,
			}
		}
	}
out:
	a.emit(ctx, eventstore.Append{
		Type:   eventstore.TurnCompleted,
		TurnID: turnID,
		Payload: eventstore.TurnPayload{
			TurnID:       turnID,
			MessageID:    assistantMsg.ID,
			ToolCalls:    len(assistantMsg.ToolCalls()),
			FinishReason: string(assistantMsg.FinishReason()),
		},
	})
	if len(toolResults) == 0 {
		return assistantMsg, nil, nil
	}
	parts := make([]message.ContentPart, 0)
	for _, tr := range toolResults {
		parts = append(parts, tr)
	}
	msg, err := a.messages.Create(context.Background(), assistantMsg.SessionID, message.CreateMessageParams{
		Role:  message.Tool,
		Parts: parts,
	})
	if err != nil {
		return assistantMsg, nil, fmt.Errorf("failed to create cancelled tool message: %w", err)
	}

	return assistantMsg, &msg, err
}

func (a *agent) finishMessage(ctx context.Context, msg *message.Message, finishReson message.FinishReason) {
	msg.AddFinish(finishReson)
	_ = a.messages.Update(ctx, *msg)
}

func (a *agent) processEvent(ctx context.Context, sessionID string, tracker *callTracker, assistantMsg *message.Message, event provider.ProviderEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue processing.
	}

	switch event.Type {
	case provider.EventThinkingDelta:
		thinking := event.Thinking
		if thinking == "" {
			thinking = event.Content
		}
		if thinking == "" {
			return nil
		}
		a.onFirstToken(ctx, tracker)
		assistantMsg.AppendReasoningContent(thinking)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventContentDelta:
		a.onFirstToken(ctx, tracker)
		assistantMsg.AppendContent(event.Content)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventToolUseStart:
		a.onFirstToken(ctx, tracker)
		assistantMsg.AddToolCall(*event.ToolCall)
		return a.messages.Update(ctx, *assistantMsg)
	// TODO: see how to handle this
	// case provider.EventToolUseDelta:
	// 	tm := time.Unix(assistantMsg.UpdatedAt, 0)
	// 	assistantMsg.AppendToolCallInput(event.ToolCall.ID, event.ToolCall.Input)
	// 	if time.Since(tm) > 1000*time.Millisecond {
	// 		err := a.messages.Update(ctx, *assistantMsg)
	// 		assistantMsg.UpdatedAt = time.Now().Unix()
	// 		return err
	// 	}
	case provider.EventToolUseStop:
		assistantMsg.FinishToolCall(event.ToolCall.ID)
		return a.messages.Update(ctx, *assistantMsg)
	case provider.EventError:
		if errors.Is(event.Error, context.Canceled) {
			logging.InfoPersist(fmt.Sprintf("Event processing canceled for session: %s", sessionID))
			return context.Canceled
		}
		logging.ErrorPersist(event.Error.Error())
		return event.Error
	case provider.EventComplete:
		assistantMsg.SetToolCalls(event.Response.ToolCalls)
		assistantMsg.AddFinish(event.Response.FinishReason)
		if err := a.messages.Update(ctx, *assistantMsg); err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
		return a.completeCall(ctx, tracker, sessionID, event.Response.Usage)
	}

	return nil
}

// callTracker holds the mutable timing state for one in-flight model call so the
// ledger row can record time-to-first-token and total latency.
type callTracker struct {
	id           string
	model        models.Model
	startedAt    time.Time
	firstTokenAt time.Time
	finalized    bool
}

// markFirstToken records the first-token time and reports whether this call was
// the transition (so a model_call.first_token event is emitted exactly once).
func (t *callTracker) markFirstToken() bool {
	if t != nil && t.firstTokenAt.IsZero() {
		t.firstTokenAt = time.Now()
		return true
	}
	return false
}

// emit appends a durable domain event, filling in the session/turn correlation
// from context. It no-ops when no event store is configured and never blocks the
// agent on event-store failures.
func (a *agent) emit(ctx context.Context, ev eventstore.Append) {
	if a.events == nil {
		return
	}
	corr := tools.CorrelationFromContext(ctx)
	if ev.SessionID == "" {
		ev.SessionID = corr.SessionID
	}
	if ev.TurnID == "" {
		ev.TurnID = corr.TurnID
	}
	if ev.TaskID == "" {
		ev.TaskID = corr.TaskID
	}
	if ev.ProjectID == "" {
		ev.ProjectID = corr.ProjectID
	}
	// Append with a detached context so events are still recorded when the
	// request context has been cancelled (e.g. model_call.failed on cancel).
	if _, err := a.events.Append(context.Background(), ev); err != nil {
		logging.Error("failed to append domain event", "type", ev.Type, "error", err)
	}
}

// startCall opens a ledger record for the model call about to stream and returns
// a tracker. The tracker always carries a fresh id (even when no ledger is
// configured) so tool executions can be correlated to the call.
func (a *agent) startCall(ctx context.Context, sessionID, turnID, messageID string) *callTracker {
	model := a.provider.Model()
	t := &callTracker{
		id:        ids.New(),
		model:     model,
		startedAt: time.Now(),
	}
	if a.ledger == nil {
		return t
	}
	corr := tools.CorrelationFromContext(ctx)
	if _, err := a.ledger.StartCall(ctx, cost.ModelCall{
		ID:        t.id,
		ProjectID: corr.ProjectID,
		TaskID:    corr.TaskID,
		TurnID:    turnID,
		SessionID: sessionID,
		MessageID: messageID,
		Provider:  string(model.Provider),
		Model:     string(model.ID),
		Status:    cost.CallStarted,
		StartedAt: t.startedAt.UnixMilli(),
	}); err != nil {
		logging.Error("failed to record model call start", "error", err)
	}
	a.emit(ctx, eventstore.Append{
		Type:   eventstore.ModelCallStarted,
		TurnID: turnID,
		Payload: eventstore.ModelCallPayload{
			ModelCallID: t.id,
			Provider:    string(model.Provider),
			Model:       string(model.ID),
			Status:      string(cost.CallStarted),
		},
	})
	return t
}

// onFirstToken records the first-token time once and emits a
// model_call.first_token event on the transition.
func (a *agent) onFirstToken(ctx context.Context, tracker *callTracker) {
	if tracker.markFirstToken() {
		a.emit(ctx, eventstore.Append{
			Type: eventstore.ModelCallFirstToken,
			Payload: eventstore.ModelCallPayload{
				ModelCallID: tracker.id,
				TTFTMS:      tracker.firstTokenAt.Sub(tracker.startedAt).Milliseconds(),
			},
		})
	}
}

// completeCall finalizes the ledger record with usage/cost and then re-derives
// the session token and cost totals from the ledger (never overwriting with a
// single call's usage). See roadmapplan.md §5.2.
func (a *agent) completeCall(ctx context.Context, tracker *callTracker, sessionID string, usage provider.TokenUsage) error {
	if tracker != nil && !tracker.finalized && a.ledger != nil {
		tracker.finalized = true
		now := time.Now()
		estCost, state := cost.ComputeCost(tracker.model, usage)
		mc := cost.ModelCall{
			ID:                  tracker.id,
			Status:              cost.CallCompleted,
			CostState:           state,
			PriceCatalogVersion: cost.PriceCatalogVersion,
			FinishedAt:          now.UnixMilli(),
			LatencyMS:           now.Sub(tracker.startedAt).Milliseconds(),
			InputTokens:         usage.InputTokens,
			OutputTokens:        usage.OutputTokens,
			CacheCreationTokens: usage.CacheCreationTokens,
			CacheReadTokens:     usage.CacheReadTokens,
			EstimatedCost:       estCost,
		}
		if !tracker.firstTokenAt.IsZero() {
			mc.FirstTokenAt = tracker.firstTokenAt.UnixMilli()
			mc.TTFTMS = tracker.firstTokenAt.Sub(tracker.startedAt).Milliseconds()
		}
		if err := a.ledger.FinishCall(ctx, mc); err != nil {
			logging.Error("failed to finalize model call", "error", err)
		}
		a.emit(ctx, eventstore.Append{
			Type: eventstore.ModelCallCompleted,
			Payload: eventstore.ModelCallPayload{
				ModelCallID:         tracker.id,
				Status:              string(cost.CallCompleted),
				InputTokens:         usage.InputTokens,
				OutputTokens:        usage.OutputTokens,
				CacheCreationTokens: usage.CacheCreationTokens,
				CacheReadTokens:     usage.CacheReadTokens,
				EstimatedCost:       estCost,
				CostState:           string(state),
				LatencyMS:           mc.LatencyMS,
				TTFTMS:              mc.TTFTMS,
			},
		})
	} else if tracker != nil {
		tracker.finalized = true
	}
	return a.reconcileSession(ctx, sessionID)
}

// abortCall records a failed or cancelled model call. The ctx is used only to
// read correlation; the ledger/event writes use a detached context because the
// request context is usually already cancelled here.
func (a *agent) abortCall(ctx context.Context, tracker *callTracker, cause error) {
	if tracker == nil || tracker.finalized {
		return
	}
	tracker.finalized = true
	now := time.Now()
	status := cost.CallFailed
	errCode := "error"
	if errors.Is(cause, context.Canceled) {
		status = cost.CallCancelled
		errCode = "cancelled"
	}
	latency := now.Sub(tracker.startedAt).Milliseconds()
	if a.ledger != nil {
		mc := cost.ModelCall{
			ID:         tracker.id,
			Status:     status,
			CostState:  cost.CostKnown,
			FinishedAt: now.UnixMilli(),
			LatencyMS:  latency,
			ErrorCode:  errCode,
		}
		if !tracker.firstTokenAt.IsZero() {
			mc.FirstTokenAt = tracker.firstTokenAt.UnixMilli()
			mc.TTFTMS = tracker.firstTokenAt.Sub(tracker.startedAt).Milliseconds()
		}
		if err := a.ledger.FinishCall(context.Background(), mc); err != nil {
			logging.Error("failed to record aborted model call", "error", err)
		}
	}
	a.emit(ctx, eventstore.Append{
		Type: eventstore.ModelCallFailed,
		Payload: eventstore.ModelCallPayload{
			ModelCallID: tracker.id,
			Status:      string(status),
			ErrorCode:   errCode,
			LatencyMS:   latency,
		},
	})
}

// finalizeCallIfOpen closes a ledger record for a stream that ended cleanly but
// never emitted an explicit completion event.
func (a *agent) finalizeCallIfOpen(ctx context.Context, tracker *callTracker, sessionID string) {
	if tracker == nil || tracker.finalized {
		return
	}
	if err := a.completeCall(ctx, tracker, sessionID, provider.TokenUsage{}); err != nil {
		logging.Error("failed to close open model call", "error", err)
	}
}

// reconcileSession recomputes the session's token and cost totals from the
// durable call ledger so they always reconcile with the underlying records.
func (a *agent) reconcileSession(ctx context.Context, sessionID string) error {
	if a.ledger == nil {
		return nil
	}
	totals, err := a.ledger.SessionTotals(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to compute session totals: %w", err)
	}
	sess, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	sess.PromptTokens = totals.PromptTokens
	sess.CompletionTokens = totals.CompletionTokens
	sess.Cost = totals.Cost
	if _, err := a.sessions.Save(ctx, sess); err != nil {
		return fmt.Errorf("failed to save session totals: %w", err)
	}
	return nil
}

func (a *agent) Update(agentName config.AgentName, modelID models.ModelID) (models.Model, error) {
	if a.IsBusy() {
		return models.Model{}, fmt.Errorf("cannot change model while processing requests")
	}

	if err := config.UpdateAgentModel(agentName, modelID); err != nil {
		return models.Model{}, fmt.Errorf("failed to update config: %w", err)
	}

	provider, err := createAgentProvider(agentName)
	if err != nil {
		return models.Model{}, fmt.Errorf("failed to create provider for model %s: %w", modelID, err)
	}

	a.provider = provider

	return a.provider.Model(), nil
}

func (a *agent) Summarize(ctx context.Context, sessionID string) error {
	if a.summarizeProvider == nil {
		return fmt.Errorf("summarize provider not available")
	}

	// Check if session is busy
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	// Create a new context with cancellation
	summarizeCtx, cancel := context.WithCancel(ctx)

	// Store the cancel function in activeRequests to allow cancellation
	a.activeRequests.Store(sessionID+"-summarize", cancel)

	go func() {
		defer a.activeRequests.Delete(sessionID + "-summarize")
		defer cancel()
		event := AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Starting summarization...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		// Get all messages from the session
		msgs, err := a.messages.List(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to list messages: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		summarizeCtx = context.WithValue(summarizeCtx, tools.SessionIDContextKey, sessionID)

		if len(msgs) == 0 {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("no messages to summarize"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Analyzing conversation...",
		}
		a.Publish(pubsub.CreatedEvent, event)

		// Add a system message to guide the summarization
		summarizePrompt := "Provide a detailed but concise summary of our conversation above. Focus on information that would be helpful for continuing the conversation, including what we did, what we're doing, which files we're working on, and what we're going to do next."

		// Create a new message with the summarize prompt
		promptMsg := message.Message{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: summarizePrompt}},
		}

		// Append the prompt to the messages
		msgsWithPrompt := append(msgs, promptMsg)

		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Generating summary...",
		}

		a.Publish(pubsub.CreatedEvent, event)

		// Send the messages to the summarize provider
		response, err := a.summarizeProvider.SendMessages(
			summarizeCtx,
			msgsWithPrompt,
			make([]tools.BaseTool, 0),
		)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to summarize: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}

		summary := strings.TrimSpace(response.Content)
		if summary == "" {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("empty summary returned"),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		event = AgentEvent{
			Type:     AgentEventTypeSummarize,
			Progress: "Creating new session...",
		}

		a.Publish(pubsub.CreatedEvent, event)
		oldSession, err := a.sessions.Get(summarizeCtx, sessionID)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to get session: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		// Create a message in the new session with the summary
		msg, err := a.messages.Create(summarizeCtx, oldSession.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: summary},
				message.Finish{
					Reason: message.FinishReasonEndTurn,
					Time:   time.Now().Unix(),
				},
			},
			Model: a.summarizeProvider.Model().ID,
		})
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to create summary message: %w", err),
				Done:  true,
			}

			a.Publish(pubsub.CreatedEvent, event)
			return
		}
		oldSession.SummaryMessageID = msg.ID
		oldSession.CompletionTokens = response.Usage.OutputTokens
		oldSession.PromptTokens = 0
		model := a.summarizeProvider.Model()
		usage := response.Usage
		cost := model.CostPer1MInCached/1e6*float64(usage.CacheCreationTokens) +
			model.CostPer1MOutCached/1e6*float64(usage.CacheReadTokens) +
			model.CostPer1MIn/1e6*float64(usage.InputTokens) +
			model.CostPer1MOut/1e6*float64(usage.OutputTokens)
		oldSession.Cost += cost
		_, err = a.sessions.Save(summarizeCtx, oldSession)
		if err != nil {
			event = AgentEvent{
				Type:  AgentEventTypeError,
				Error: fmt.Errorf("failed to save session: %w", err),
				Done:  true,
			}
			a.Publish(pubsub.CreatedEvent, event)
		}

		event = AgentEvent{
			Type:      AgentEventTypeSummarize,
			SessionID: oldSession.ID,
			Progress:  "Summary complete",
			Done:      true,
		}
		a.Publish(pubsub.CreatedEvent, event)
		// Send final success event with the new session ID
	}()

	return nil
}

func createAgentProvider(agentName config.AgentName) (provider.Provider, error) {
	cfg := config.Get()
	agentConfig, ok := cfg.Agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentName)
	}
	model, ok := models.SupportedModels[agentConfig.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not supported", agentConfig.Model)
	}

	providerCfg, ok := cfg.Providers[model.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not supported", model.Provider)
	}
	if providerCfg.Disabled {
		return nil, fmt.Errorf("provider %s is not enabled", model.Provider)
	}
	maxTokens := model.DefaultMaxTokens
	if agentConfig.MaxTokens > 0 {
		maxTokens = agentConfig.MaxTokens
	}
	opts := []provider.ProviderClientOption{
		provider.WithAPIKey(providerCfg.APIKey),
		provider.WithModel(model),
		provider.WithSystemMessage(prompt.GetAgentPrompt(agentName, model.Provider)),
		provider.WithMaxTokens(maxTokens),
	}
	if model.Provider == models.ProviderOpenAI || model.Provider == models.ProviderLocal && model.CanReason {
		opts = append(
			opts,
			provider.WithOpenAIOptions(
				provider.WithReasoningEffort(agentConfig.ReasoningEffort),
			),
		)
	} else if model.Provider == models.ProviderAnthropic && model.CanReason && agentName == config.AgentCoder {
		opts = append(
			opts,
			provider.WithAnthropicOptions(
				provider.WithAnthropicShouldThinkFn(provider.DefaultShouldThinkFn),
			),
		)
	}
	agentProvider, err := provider.NewProvider(
		model.Provider,
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create provider: %v", err)
	}

	return agentProvider, nil
}
