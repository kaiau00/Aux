package agent

import (
	"context"

	"github.com/aux-ai/aux-cli/internal/history"
	"github.com/aux-ai/aux-cli/internal/hooks"
	"github.com/aux-ai/aux-cli/internal/impact"
	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/lsp"
	"github.com/aux-ai/aux-cli/internal/permission"
)

func CoderAgentTools(
	deps Deps,
	permissions permission.Service,
	history history.Service,
	lspClients map[string]*lsp.Client,
	hookRegistry *hooks.Registry,
	impactSvc *impact.Service,
) []tools.BaseTool {
	ctx := context.Background()
	otherTools := GetMcpTools(ctx, permissions)
	if len(lspClients) > 0 {
		otherTools = append(otherTools, tools.NewDiagnosticsTool(lspClients))
	}
	return append(
		[]tools.BaseTool{
			tools.NewBashTool(permissions),
			tools.NewEditTool(lspClients, permissions, history),
			tools.NewFetchTool(permissions),
			tools.NewGlobTool(permissions),
			tools.NewGrepTool(permissions),
			tools.NewLsTool(permissions),
			tools.NewSourcegraphTool(),
			tools.NewViewTool(lspClients, permissions),
			tools.NewPatchTool(lspClients, permissions, history),
			tools.NewWriteTool(lspClients, permissions, history),
			NewAgentTool(deps, hookRegistry, permissions, impactSvc, lspClients),
		}, otherTools...,
	)
}

// TaskAgentTools is the read-only tool set given to subagents. permissions is
// required for the same reason the coder agent needs it: a subagent reading
// outside its working directory must prompt rather than silently succeed.
func TaskAgentTools(lspClients map[string]*lsp.Client, permissions permission.Service) []tools.BaseTool {
	return []tools.BaseTool{
		tools.NewGlobTool(permissions),
		tools.NewGrepTool(permissions),
		tools.NewLsTool(permissions),
		tools.NewSourcegraphTool(),
		tools.NewViewTool(lspClients, permissions),
	}
}
