package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/dashboard"
	"github.com/aux-ai/aux-cli/internal/db"
	"github.com/aux-ai/aux-cli/internal/format"
	"github.com/aux-ai/aux-cli/internal/history"
	"github.com/aux-ai/aux-cli/internal/llm/agent"
	"github.com/aux-ai/aux-cli/internal/logging"
	"github.com/aux-ai/aux-cli/internal/lsp"
	"github.com/aux-ai/aux-cli/internal/message"
	"github.com/aux-ai/aux-cli/internal/permission"
	"github.com/aux-ai/aux-cli/internal/session"
	"github.com/aux-ai/aux-cli/internal/tui/theme"
	"github.com/aux-ai/aux-cli/internal/welcome"
)

type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service

	CoderAgent agent.Service
	Dashboard  *dashboard.Server

	LSPClients map[string]*lsp.Client

	// BootstrapSession is the session the TUI should select on first launch,
	// if any (e.g. the first-boot welcome session).
	BootstrapSession session.Session

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

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Permissions: permission.NewPermissionService(),
		LSPClients:  make(map[string]*lsp.Client),
	}

	// Initialize theme based on configuration
	app.initTheme()

	// Initialize LSP clients in the background
	go app.initLSPClients(ctx)

	agent.GetMcpTools(ctx, app.Permissions)

	var err error
	app.CoderAgent, err = agent.NewAgent(
		config.AgentCoder,
		app.Sessions,
		app.Messages,
		agent.CoderAgentTools(
			app.Permissions,
			app.Sessions,
			app.Messages,
			app.History,
			app.LSPClients,
		),
	)
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
	}, dashboardOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to start dashboard: %w", err)
	}

	// First-boot welcome: creates a session with an intro message that the
	// TUI will auto-select on startup. Failures are swallowed inside the
	// welcome package so this never blocks startup.
	if wres := welcome.MaybeShow(ctx, app.Sessions, app.Messages, app.Dashboard); wres.Shown {
		app.BootstrapSession = wres.Session
	}

	return app, nil
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
