package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/hooks"
	"github.com/aux-ai/aux-cli/internal/impact"
	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/lsp"
	"github.com/aux-ai/aux-cli/internal/message"
	"github.com/aux-ai/aux-cli/internal/permission"
)

type agentTool struct {
	deps        Deps
	hooks       *hooks.Registry
	permissions permission.Service
	impactSvc   *impact.Service
	lspClients  map[string]*lsp.Client
}

const (
	AgentToolName = "agent"
)

type AgentParams struct {
	Prompt string `json:"prompt"`
	// Role specializes the subagent's tool set and prompt (roadmapplan.md
	// §11.3). Omit for the generic, unspecialized subagent.
	Role Role `json:"role,omitempty"`
}

func (b *agentTool) Info() tools.ToolInfo {
	return tools.ToolInfo{
		Name:        AgentToolName,
		Description: "Launch a new agent that has access to bounded semantic code exploration tools. The agent should prefer codebase-memory-guided retrieval before broad Glob, Grep, LS, or View usage.\n\nUsage notes:\n1. Launch multiple agents concurrently whenever possible, to maximize performance; to do that, use a single message with multiple tool uses\n2. When the agent is done, it reports back via the report tool rather than a free-text message; that structured report is what you receive as this tool's result. To show the user the result, send a text message summarizing it.\n3. Each agent invocation is stateless. You will not be able to send additional messages to the agent, nor will the agent be able to communicate with you outside of its final report. Therefore, your prompt should contain a highly detailed task description for the agent to perform autonomously.\n4. Optionally set role to specialize the agent: repo_mapper (locate relevant files/symbols), impact_analyst (determine blast radius via the impact graph), validation_runner (run validation commands), or reviewer (find issues in current changes). Omit role for a generic exploration agent.\n5. The agent's outputs should generally be trusted\n6. IMPORTANT: Only the validation_runner role can run commands (Bash); no role can Edit or Write files directly. If you want to modify files, do that directly instead of going through the agent.",
		Parameters: map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The task for the agent to perform",
			},
			"role": map[string]any{
				"type":        "string",
				"enum":        []string{"", string(RoleRepoMapper), string(RoleImpactAnalyst), string(RoleValidationRunner), string(RoleReviewer)},
				"description": "Optional specialist role; omit for a generic exploration agent",
			},
		},
		Required: []string{"prompt"},
	}
}

func (b *agentTool) Run(ctx context.Context, call tools.ToolCall) (tools.ToolResponse, error) {
	var params AgentParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return tools.NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}
	if params.Prompt == "" {
		return tools.NewTextErrorResponse("prompt is required"), nil
	}

	sessionID, messageID := tools.GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return tools.ToolResponse{}, fmt.Errorf("session_id and message_id are required")
	}

	parentTaskID := tools.CorrelationFromContext(ctx).TaskID

	// Subagents run with the same task coordinator as the parent (§11.3): a
	// subagent's own Run(...) call begins and finishes a real task, linked to
	// the parent task via tools.ParentTaskIDContextKey (read by
	// task.Coordinator.Begin). This gives each subagent real checkpointing and
	// cost attribution "for free" through the coordinator's existing lifecycle,
	// rather than opening a fresh top-level task tree.
	subDeps := b.deps
	subCtx := ctx
	if parentTaskID != "" {
		subCtx = context.WithValue(subCtx, tools.ParentTaskIDContextKey, parentTaskID)
	}

	collector := &reportCollector{}
	base := TaskAgentTools(b.lspClients)
	var bashTool tools.BaseTool
	if params.Role == RoleValidationRunner && b.permissions != nil {
		bashTool = tools.NewBashTool(b.permissions)
	}
	subTools := roleTools(params.Role, base, b.impactSvc, bashTool, collector)

	subAgent, err := NewAgent(config.AgentTask, subDeps, subTools)
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error creating agent: %s", err)
	}

	session, err := b.deps.Sessions.CreateTaskSession(ctx, call.ID, sessionID, "New Agent Session")
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
	}

	prompt := params.Prompt
	if fragment := params.Role.rolePrompt(); fragment != "" {
		prompt = fragment + "\n\n" + prompt
	}

	_ = b.hooks.Dispatch(subCtx, hooks.Event{
		Point: hooks.SubtaskBegin, TaskID: parentTaskID, ParentTaskID: parentTaskID,
		SessionID: session.ID, Data: map[string]string{"role": string(params.Role)},
	})

	done, err := subAgent.Run(subCtx, session.ID, prompt)
	if err != nil {
		return tools.ToolResponse{}, fmt.Errorf("error generating agent: %s", err)
	}
	result := <-done

	outcome := "completed"
	if result.Error != nil {
		outcome = "failed"
	}
	_ = b.hooks.Dispatch(subCtx, hooks.Event{
		Point: hooks.SubtaskEnd, TaskID: parentTaskID, ParentTaskID: parentTaskID,
		SessionID: session.ID, Outcome: outcome, Data: map[string]string{"role": string(params.Role)},
	})

	if result.Error != nil {
		return tools.ToolResponse{}, fmt.Errorf("error generating agent: %s", result.Error)
	}

	// The parent session's cost is derived from its own ledger plus the cost of
	// direct child (subagent) sessions/tasks in cost.Service.SessionTotals /
	// TaskTotals, so no manual roll-up is needed here.
	if report, ok := collector.get(); ok {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return tools.ToolResponse{}, fmt.Errorf("error encoding report: %s", err)
		}
		return tools.NewTextResponse(string(encoded)), nil
	}

	// The model ended without calling the report tool; fall back to its final
	// message so a missed report tool call is never silently empty.
	response := result.Message
	if response.Role != message.Assistant {
		return tools.NewTextErrorResponse("no response"), nil
	}
	return tools.NewTextResponse(response.Content().String()), nil
}

func NewAgentTool(deps Deps, hookRegistry *hooks.Registry, permissions permission.Service, impactSvc *impact.Service, lspClients map[string]*lsp.Client) tools.BaseTool {
	return &agentTool{
		deps:        deps,
		hooks:       hookRegistry,
		permissions: permissions,
		impactSvc:   impactSvc,
		lspClients:  lspClients,
	}
}
