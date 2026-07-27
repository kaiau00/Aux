package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"sync"
	"time"

	"github.com/aux-ai/aux-cli/internal/artifact"
	"github.com/aux-ai/aux-cli/internal/checkpoint"
	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/contextstore"
	"github.com/aux-ai/aux-cli/internal/cost"
	"github.com/aux-ai/aux-cli/internal/dashboard"
	"github.com/aux-ai/aux-cli/internal/db"
	"github.com/aux-ai/aux-cli/internal/eventstore"
	"github.com/aux-ai/aux-cli/internal/format"
	"github.com/aux-ai/aux-cli/internal/govpolicy"
	"github.com/aux-ai/aux-cli/internal/history"
	"github.com/aux-ai/aux-cli/internal/hooks"
	"github.com/aux-ai/aux-cli/internal/impact"
	"github.com/aux-ai/aux-cli/internal/llm/agent"
	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/logging"
	"github.com/aux-ai/aux-cli/internal/lsp"
	"github.com/aux-ai/aux-cli/internal/memory"
	"github.com/aux-ai/aux-cli/internal/message"
	"github.com/aux-ai/aux-cli/internal/mutationcp"
	"github.com/aux-ai/aux-cli/internal/permission"
	"github.com/aux-ai/aux-cli/internal/profile"
	"github.com/aux-ai/aux-cli/internal/project"
	"github.com/aux-ai/aux-cli/internal/promptcompiler"
	"github.com/aux-ai/aux-cli/internal/session"
	"github.com/aux-ai/aux-cli/internal/skill"
	"github.com/aux-ai/aux-cli/internal/task"
	"github.com/aux-ai/aux-cli/internal/toolexec"
	"github.com/aux-ai/aux-cli/internal/tui/theme"
	"github.com/aux-ai/aux-cli/internal/validation"
	"github.com/aux-ai/aux-cli/internal/viewmodel"
)

type App struct {
	Sessions        session.Service
	Messages        message.Service
	History         history.Service
	Permissions     permission.Service
	Cost            cost.Service
	Events          eventstore.Service
	ToolRecorder    tools.Recorder
	Projects        *project.Service
	Profiles        *profile.Service
	Tasks           *task.Store
	TaskCoord       *task.Coordinator
	Artifacts       *artifact.Service
	Pages           *contextstore.Store
	Memories        *memory.Service
	Impact          *impact.Service
	Validations     *validation.Service
	Skills          *skill.Service
	Policies        *govpolicy.Service
	Checkpoints     *checkpoint.Service
	CheckpointStore *checkpoint.Store
	Hooks           *hooks.Registry

	CoderAgent agent.Service
	Dashboard  *dashboard.Server

	LSPClients map[string]*lsp.Client

	clientsMutex sync.RWMutex

	watcherCancelFuncs []context.CancelFunc
	cancelFuncsMutex   sync.Mutex
	watcherWG          sync.WaitGroup
}

func New(ctx context.Context, conn *sql.DB) (*App, error) {
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	files := history.NewService(q, conn)
	// The ledger and event store run raw SQL directly against the connection
	// (see ADR 0003). The event store is the durable, ordered runtime log.
	ledger := cost.NewService(conn)
	events := eventstore.NewService(conn)
	projects := project.NewService(project.NewStore(conn), project.GitVCS{})
	profiles := profile.NewService(profile.NewStore(conn), profile.NewBuilder(profile.NewStore(conn), profile.DefaultScanners()))
	taskStore := task.NewStore(conn)
	memories := memory.NewService(memory.NewStore(conn), events)
	validations := validation.NewService(validation.NewStore(conn), events)
	hookRegistry := hooks.NewRegistry()
	taskCoord := task.NewCoordinator(projects, profiles, taskStore, events, config.WorkingDirectory()).
		WithMemory(memories).WithValidation(validations).WithHooks(hookRegistry)
	// Content-addressed artifact store, with bytes under the app data directory.
	artifacts := artifact.NewService(
		artifact.NewFSBackend(filepath.Join(config.Get().Data.Directory, "artifacts")),
		artifact.NewStore(conn),
	)
	pages := contextstore.NewStore(conn)
	impactSvc := impact.NewService(impact.NewStore(conn))
	skills := skill.NewService(skill.NewStore(conn), events)
	policies := govpolicy.NewService(govpolicy.NewStore(conn), events)
	checkpointStore := checkpoint.NewStore(conn)
	checkpoints := checkpoint.NewService(checkpointStore, artifacts, events)
	// A completed task automatically checkpoints what it changed, using the file
	// versions the edit/write tools already recorded (roadmapplan.md §11.1).
	taskCoord.WithCheckpoints(files, checkpoints)

	// The tool recorder persists tool_executions and emits tool.* events; the
	// executor (built inside the agent) routes every tool call through it. Its
	// observer captures a first-mutation baseline checkpoint (§11.1).
	mutationCheckpointer := mutationcp.New(files, mutationcp.StoreAdapter{Store: checkpointStore, Service: checkpoints})
	toolRecorder := toolexec.NewRecorder(toolexec.NewStore(conn), events,
		toolexec.WithObserver(func(ctx context.Context, rec tools.ExecutionRecord) {
			_ = mutationCheckpointer.OnToolCompleted(ctx, rec.Correlation.TaskID, rec.Correlation.SessionID, rec.ToolName)
		}))

	app := &App{
		Sessions:        sessions,
		Messages:        messages,
		History:         files,
		Permissions:     permission.NewPermissionService(),
		Cost:            ledger,
		Events:          events,
		ToolRecorder:    toolRecorder,
		Projects:        projects,
		Profiles:        profiles,
		Tasks:           taskStore,
		TaskCoord:       taskCoord,
		Artifacts:       artifacts,
		Pages:           pages,
		Memories:        memories,
		Impact:          impactSvc,
		Validations:     validations,
		Skills:          skills,
		Policies:        policies,
		Checkpoints:     checkpoints,
		CheckpointStore: checkpointStore,
		Hooks:           hookRegistry,
		LSPClients:      make(map[string]*lsp.Client),
	}

	// Initialize theme based on configuration
	app.initTheme()

	// Resolve project identity and compile its profile in the background so
	// startup is not blocked on git/filesystem work (best-effort; failures are
	// logged and do not affect normal operation).
	go app.resolveProject(ctx)

	// Initialize LSP clients in the background
	go app.initLSPClients(ctx)

	agent.GetMcpTools(ctx, app.Permissions)

	var err error
	virtualizer := artifact.NewVirtualizer(
		app.Artifacts,
		artifact.Mode(config.Get().Context.Virtualization),
		config.Get().Context.ArtifactThresholdBytes,
	)
	// Select the prompt compiler: demand paging when enabled, else compatibility.
	var compiler promptcompiler.Compiler = promptcompiler.NewCompatibilityCompiler()
	if config.Get().Context.Paging == "on" {
		compiler = promptcompiler.NewPagingCompiler()
	}
	coderDeps := agent.Deps{
		Sessions:     app.Sessions,
		Messages:     app.Messages,
		Ledger:       app.Cost,
		Events:       app.Events,
		Recorder:     app.ToolRecorder,
		Coordinator:  app.TaskCoord,
		Virtualizer:  virtualizer,
		Pages:        app.Pages,
		Compiler:     compiler,
		GovernorMode: cost.GovernorMode(config.Get().CostGovernor.Mode),
	}
	coderTools := append(
		agent.CoderAgentTools(coderDeps, app.Permissions, app.History, app.LSPClients),
		artifact.NewViewTool(app.Artifacts),
	)
	app.CoderAgent, err = agent.NewAgent(config.AgentCoder, coderDeps, coderTools)
	if err != nil {
		logging.Error("Failed to create coder agent", err)
		return nil, err
	}

	dashboardOptions := dashboard.Options{
		Enabled:     config.Get().Dashboard.Enabled,
		Host:        config.Get().Dashboard.Host,
		Port:        config.Get().Dashboard.Port,
		Redaction:   dashboard.RedactionMode(config.Get().Dashboard.Redaction),
		FullContent: config.Get().Dashboard.FullContent,
	}
	app.Dashboard, err = dashboard.Start(ctx, dashboard.Services{
		Sessions: app.Sessions,
		Messages: app.Messages,
		History:  app.History,
		Agent:    app.CoderAgent,
		Tasks: viewmodel.Stores{
			Tasks:       app.Tasks,
			Events:      app.Events,
			Validations: app.Validations,
			Checkpoints: app.CheckpointStore,
			Pages:       app.Pages,
			Ledger:      app.Cost,
		},
	}, dashboardOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to start dashboard: %w", err)
	}

	return app, nil
}

// resolveProject resolves the working directory to a stable project identity,
// records the current revision, and compiles the project profile. It emits a
// project.resolved-style signal via a domain event so read models can react.
func (app *App) resolveProject(ctx context.Context) {
	defer logging.RecoverPanic("app.resolveProject", nil)
	res, err := app.Projects.Resolve(ctx, config.WorkingDirectory())
	if err != nil {
		logging.Warn("failed to resolve project identity", "error", err)
		return
	}
	logging.Debug("resolved project", "project", res.Project.CanonicalName, "id", res.Project.ID, "branch", res.Revision.BranchName)

	eff, err := app.Profiles.CompileEffective(ctx, res.Project.ID, res.Revision.ID, res.Root.CanonicalPath, res.Revision.VCSRevision, "")
	if err != nil {
		logging.Warn("failed to compile project profile", "error", err)
		return
	}
	// Record the profile-input fingerprint on the revision for staleness checks.
	if fp, ferr := app.Profiles.InputFingerprint(ctx, res.Root.CanonicalPath); ferr == nil {
		_ = app.Projects.Store().SetRevisionProfileInputHash(ctx, res.Revision.ID, fp)
	}
	logging.Debug("compiled effective profile", "entries", len(eff.Entries), "manifestTokens", eff.TokenEstimate)

	// Build the change-impact graph (deterministic AST analysis). Best-effort;
	// impact analysis broadens validation automatically when the graph is absent.
	if n, gerr := app.Impact.Index(ctx, res.Project.ID, res.Root.CanonicalPath, res.Revision.VCSRevision); gerr != nil {
		logging.Warn("failed to build impact graph", "error", gerr)
	} else {
		logging.Debug("built impact graph", "nodes", n)
	}
}

// initTheme sets the application theme based on the configuration
func (app *App) initTheme() {
	cfg := config.Get()
	if cfg == nil || cfg.TUI.Theme == "" {
		return // Use default theme
	}

	// Try to set the theme from config
	err := theme.SetTheme(cfg.TUI.Theme)
	if err != nil {
		logging.Warn("Failed to set theme from config, using default theme", "theme", cfg.TUI.Theme, "error", err)
	} else {
		logging.Debug("Set theme from config", "theme", cfg.TUI.Theme)
	}
}

// RunNonInteractive handles the execution flow when a prompt is provided via CLI flag.
func (a *App) RunNonInteractive(ctx context.Context, prompt string, outputFormat string, quiet bool) error {
	logging.Info("Running in non-interactive mode")

	// Start spinner if not in quiet mode
	var spinner *format.Spinner
	if !quiet {
		spinner = format.NewSpinner("Thinking...")
		spinner.Start()
		defer spinner.Stop()
	}

	const maxPromptLengthForTitle = 100
	titlePrefix := "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := a.Sessions.Create(ctx, title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	logging.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive session
	a.Permissions.AutoApproveSession(sess.ID)

	done, err := a.CoderAgent.Run(ctx, sess.ID, prompt)
	if err != nil {
		return fmt.Errorf("failed to start agent processing stream: %w", err)
	}

	result := <-done
	if result.Error != nil {
		if errors.Is(result.Error, context.Canceled) || errors.Is(result.Error, agent.ErrRequestCancelled) {
			logging.Info("Agent processing cancelled", "session_id", sess.ID)
			return nil
		}
		return fmt.Errorf("agent processing failed: %w", result.Error)
	}

	// Stop spinner before printing output
	if !quiet && spinner != nil {
		spinner.Stop()
	}

	// Get the text content from the response
	content := "No content available"
	if result.Message.Content().String() != "" {
		content = result.Message.Content().String()
	}

	fmt.Println(format.FormatOutput(content, outputFormat))

	logging.Info("Non-interactive run completed", "session_id", sess.ID)

	return nil
}

// Shutdown performs a clean shutdown of the application
func (app *App) Shutdown() {
	if app.Dashboard != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := app.Dashboard.Shutdown(ctx); err != nil {
			logging.Warn("Failed to shutdown dashboard", "error", err)
		}
		cancel()
	}

	// Cancel all watcher goroutines
	app.cancelFuncsMutex.Lock()
	for _, cancel := range app.watcherCancelFuncs {
		cancel()
	}
	app.cancelFuncsMutex.Unlock()
	app.watcherWG.Wait()

	// Perform additional cleanup for LSP clients
	app.clientsMutex.RLock()
	clients := make(map[string]*lsp.Client, len(app.LSPClients))
	maps.Copy(clients, app.LSPClients)
	app.clientsMutex.RUnlock()

	for name, client := range clients {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := client.Shutdown(shutdownCtx); err != nil {
			logging.Error("Failed to shutdown LSP client", "name", name, "error", err)
		}
		cancel()
	}
}
