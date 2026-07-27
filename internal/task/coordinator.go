package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aux-ai/aux-cli/internal/checkpoint"
	"github.com/aux-ai/aux-cli/internal/eventstore"
	"github.com/aux-ai/aux-cli/internal/history"
	"github.com/aux-ai/aux-cli/internal/hooks"
	"github.com/aux-ai/aux-cli/internal/ids"
	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/memory"
	"github.com/aux-ai/aux-cli/internal/profile"
	"github.com/aux-ai/aux-cli/internal/project"
	"github.com/aux-ai/aux-cli/internal/promptcompiler"
	"github.com/aux-ai/aux-cli/internal/validation"
)

// FileLister lists the recorded file versions for a session (history service).
type FileLister interface {
	ListBySession(ctx context.Context, sessionID string) ([]history.File, error)
	ListLatestSessionFiles(ctx context.Context, sessionID string) ([]history.File, error)
}

// CheckpointCreator captures a checkpoint from a set of file changes.
type CheckpointCreator interface {
	Create(ctx context.Context, taskID, parentID, label, revision string, changes []checkpoint.FileChange) (checkpoint.Checkpoint, error)
}

// ProjectResolver resolves a working directory to a project identity.
type ProjectResolver interface {
	Resolve(ctx context.Context, dir string) (project.Resolution, error)
}

// ProfileCompiler compiles a project's effective profile.
type ProfileCompiler interface {
	CompileEffective(ctx context.Context, projectID, revisionID, root, sourceRevision, taskMode string) (profile.Effective, error)
}

// EventSink appends domain events.
type EventSink interface {
	Append(ctx context.Context, in eventstore.Append) (eventstore.Event, error)
}

// Coordinator turns each new user objective into a compiled, versioned task
// bound to the current project revision and effective profile, before any tool
// runs (roadmapplan.md §3.5, §6.5). It caches project resolution per session.
type Coordinator struct {
	resolver    ProjectResolver
	profiles    ProfileCompiler
	store       *Store
	events      EventSink
	memories    *memory.Service
	validations *validation.Service
	history     FileLister
	checkpoints CheckpointCreator
	hooks       *hooks.Registry
	workdir     string

	mu     sync.Mutex
	cached *project.Resolution
}

// NewCoordinator builds a task coordinator. events may be nil.
func NewCoordinator(resolver ProjectResolver, profiles ProfileCompiler, store *Store, events EventSink, workdir string) *Coordinator {
	return &Coordinator{resolver: resolver, profiles: profiles, store: store, events: events, workdir: workdir}
}

// WithMemory attaches a memory service so completed tasks produce episodic
// memory candidates and active memories surface as available context. Optional.
func (c *Coordinator) WithMemory(m *memory.Service) *Coordinator {
	c.memories = m
	return c
}

// WithValidation attaches a validation service so procedural memory can be
// extracted from commands that validated successfully during a task. Optional.
func (c *Coordinator) WithValidation(v *validation.Service) *Coordinator {
	c.validations = v
	return c
}

// WithCheckpoints wires history + checkpoint capture so a completed task
// automatically records what it changed (roadmapplan.md §11.1). Both are
// required for capture; either nil disables it. Optional.
func (c *Coordinator) WithCheckpoints(files FileLister, checkpoints CheckpointCreator) *Coordinator {
	c.history = files
	c.checkpoints = checkpoints
	return c
}

// WithHooks wires a lifecycle-hook registry so local extensions can observe task
// begin/end (roadmapplan.md §12.3). Optional; nil dispatch is a no-op.
func (c *Coordinator) WithHooks(r *hooks.Registry) *Coordinator {
	c.hooks = r
	return c
}

func (c *Coordinator) resolution(ctx context.Context) (project.Resolution, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil {
		return *c.cached, nil
	}
	res, err := c.resolver.Resolve(ctx, c.workdir)
	if err != nil {
		return project.Resolution{}, err
	}
	c.cached = &res
	return res, nil
}

// Begin resolves the project, compiles the effective profile and a task spec,
// persists the task/spec/budget, emits task lifecycle events, and returns a
// context carrying the task and project ids for downstream correlation.
func (c *Coordinator) Begin(ctx context.Context, sessionID, objective string) (context.Context, string, error) {
	res, err := c.resolution(ctx)
	if err != nil {
		return ctx, "", err
	}
	eff, err := c.profiles.CompileEffective(ctx, res.Project.ID, res.Revision.ID, res.Root.CanonicalPath, res.Revision.VCSRevision, "")
	if err != nil {
		return ctx, "", err
	}

	mode := InferMode(objective)
	spec := Compile(objective, mode, eff)
	taskID := ids.New()
	spec.TaskID = taskID
	spec.SpecVersion = 1
	now := time.Now().UnixMilli()

	t := Task{
		ID:                taskID,
		ProjectID:         res.Project.ID,
		SessionID:         sessionID,
		ProjectRevisionID: res.Revision.ID,
		ProfileVersionSet: eff.VersionSetHash,
		Objective:         spec.Objective,
		Mode:              mode,
		Status:            StatusCompiled,
		CreatedAt:         now,
	}
	if err := c.store.CreateTask(ctx, t); err != nil {
		return ctx, "", err
	}
	if err := c.store.SaveSpec(ctx, spec, "", now); err != nil {
		return ctx, "", err
	}
	if err := c.store.SaveBudget(ctx, taskID, spec.Budget); err != nil {
		return ctx, "", err
	}
	if err := c.store.SetStatus(ctx, taskID, StatusRunning, "", now, 0); err != nil {
		return ctx, "", err
	}

	c.emit(ctx, res.Project.ID, sessionID, eventstore.TaskCreated, eventPayload(t, eff.VersionSetHash))
	c.emit(ctx, res.Project.ID, sessionID, eventstore.TaskCompiled, eventPayload(t, eff.VersionSetHash))
	c.emit(ctx, res.Project.ID, sessionID, eventstore.TaskStarted, eventPayload(t, eff.VersionSetHash))

	ctx = context.WithValue(ctx, tools.TaskIDContextKey, taskID)
	ctx = context.WithValue(ctx, tools.ProjectIDContextKey, res.Project.ID)
	_ = c.hooks.Dispatch(ctx, hooks.Event{Point: hooks.TaskBegin, TaskID: taskID, SessionID: sessionID})
	// Offer the compiled project manifest (plus any relevant prior memory) and the
	// task spec to the prompt compiler as available context pages (§7.1, §8.4).
	manifest := eff.Manifest + c.memorySection(ctx, res.Project.ID)
	ctx = promptcompiler.WithProjectContext(ctx, manifest, spec.RenderText())
	return ctx, taskID, nil
}

// memorySection renders a bounded set of active memories for the manifest, so
// prior project knowledge is available without re-discovery (roadmapplan.md §8.4).
func (c *Coordinator) memorySection(ctx context.Context, projectID string) string {
	if c.memories == nil {
		return ""
	}
	mems, err := c.memories.Retrieve(ctx, projectID, nil, 5)
	if err != nil || len(mems) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Prior knowledge:\n")
	for _, m := range mems {
		fmt.Fprintf(&b, "  - [%s] %s\n", m.Type, m.StableKey)
	}
	return b.String()
}

// Finish marks a task completed and emits task.completed.
func (c *Coordinator) Finish(ctx context.Context, taskID, outcome string) {
	if taskID == "" {
		return
	}
	now := time.Now().UnixMilli()
	if err := c.store.SetStatus(ctx, taskID, StatusCompleted, outcome, 0, now); err != nil {
		return
	}
	projectID, _ := ctx.Value(tools.ProjectIDContextKey).(string)
	sessionID, _ := ctx.Value(tools.SessionIDContextKey).(string)
	c.emit(context.Background(), projectID, sessionID, eventstore.TaskCompleted, eventstore.TaskPayload{
		TaskID: taskID, Status: string(StatusCompleted), Outcome: outcome,
	})
	c.captureCheckpoint(context.Background(), taskID, sessionID)
	c.learnFromTask(context.Background(), taskID, outcome)
	_ = c.hooks.Dispatch(context.Background(), hooks.Event{
		Point: hooks.TaskEnd, TaskID: taskID, SessionID: sessionID, Outcome: outcome,
	})
}

// captureCheckpoint records what a completed task changed by snapshotting the
// session's recorded file versions into a checkpoint (roadmapplan.md §11.1). The
// before/after content comes from the history the edit/write tools already wrote,
// so the change set is truthful — never inferred. Best-effort: any failure is
// swallowed so it never affects task completion.
func (c *Coordinator) captureCheckpoint(ctx context.Context, taskID, sessionID string) {
	if c.checkpoints == nil || c.history == nil || sessionID == "" {
		return
	}
	all, err := c.history.ListBySession(ctx, sessionID)
	if err != nil || len(all) == 0 {
		return
	}
	latest, err := c.history.ListLatestSessionFiles(ctx, sessionID)
	if err != nil {
		return
	}

	changes := checkpoint.ChangesFrom(initialContentByPath(all), toFileVersions(latest))
	if len(changes) == 0 {
		return
	}

	rev := ""
	if c.cached != nil {
		rev = c.cached.Revision.VCSRevision
	}
	_, _ = c.checkpoints.Create(ctx, taskID, "", "task-complete", rev, changes)
}

// initialContentByPath maps each path to its earliest ("initial") recorded
// content — the "before" snapshot for a change set.
func initialContentByPath(all []history.File) map[string]string {
	before := make(map[string]string, len(all))
	for _, f := range all {
		if f.Version == history.InitialVersion {
			before[f.Path] = f.Content
		}
	}
	return before
}

// toFileVersions adapts history files to the checkpoint change-set input.
func toFileVersions(files []history.File) []checkpoint.FileVersion {
	out := make([]checkpoint.FileVersion, 0, len(files))
	for _, f := range files {
		out = append(out, checkpoint.FileVersion{Path: f.Path, Content: f.Content})
	}
	return out
}

// learnFromTask extracts deterministic memory candidates from a completed task
// (roadmapplan.md §8.2). This is safe to run after PR 12; earlier slices
// generated the evidence memory needs.
func (c *Coordinator) learnFromTask(ctx context.Context, taskID, outcome string) {
	if c.memories == nil {
		return
	}
	t, err := c.store.GetTask(ctx, taskID)
	if err != nil {
		return
	}
	rev := ""
	if c.cached != nil {
		rev = c.cached.Revision.VCSRevision
	}
	// Commands that validated successfully become procedural-memory candidates.
	var successfulCommands []string
	if c.validations != nil {
		successfulCommands, _ = c.validations.SuccessfulCommands(ctx, t.ID)
	}
	_ = c.memories.Learn(ctx, memory.Extract(memory.ExtractInput{
		ProjectID:          t.ProjectID,
		TaskID:             t.ID,
		Objective:          t.Objective,
		Mode:               string(t.Mode),
		Outcome:            outcome,
		SupportingRevision: rev,
		SuccessfulCommands: successfulCommands,
	}))
}

// Fail marks a task failed or cancelled and emits the corresponding event.
func (c *Coordinator) Fail(ctx context.Context, taskID string, cause error) {
	if taskID == "" {
		return
	}
	status := StatusFailed
	evType := eventstore.TaskFailed
	if errors.Is(cause, context.Canceled) {
		status = StatusCancelled
		evType = eventstore.TaskCancelled
	}
	now := time.Now().UnixMilli()
	if err := c.store.SetStatus(ctx, taskID, status, cause.Error(), 0, now); err != nil {
		return
	}
	projectID, _ := ctx.Value(tools.ProjectIDContextKey).(string)
	sessionID, _ := ctx.Value(tools.SessionIDContextKey).(string)
	c.emit(context.Background(), projectID, sessionID, evType, eventstore.TaskPayload{
		TaskID: taskID, Status: string(status), Outcome: cause.Error(),
	})
}

func (c *Coordinator) emit(ctx context.Context, projectID, sessionID string, evType eventstore.Type, payload eventstore.TaskPayload) {
	if c.events == nil {
		return
	}
	_, _ = c.events.Append(ctx, eventstore.Append{
		Type:      evType,
		ProjectID: projectID,
		SessionID: sessionID,
		TaskID:    payload.TaskID,
		Payload:   payload,
	})
}

func eventPayload(t Task, profileVersionID string) eventstore.TaskPayload {
	return eventstore.TaskPayload{
		TaskID:           t.ID,
		Objective:        t.Objective,
		Mode:             string(t.Mode),
		Status:           string(t.Status),
		ProfileVersionID: profileVersionID,
	}
}
