package promptcompiler

import "context"

type ctxKey string

const (
	projectManifestKey ctxKey = "aux.project_manifest"
	taskSpecKey        ctxKey = "aux.task_spec"
)

// WithProjectContext attaches the compiled project manifest and task spec text to
// ctx so the runtime can offer them to the compiler as available pages without a
// new dependency edge between the task coordinator and the agent.
func WithProjectContext(ctx context.Context, projectManifest, taskSpecText string) context.Context {
	ctx = context.WithValue(ctx, projectManifestKey, projectManifest)
	ctx = context.WithValue(ctx, taskSpecKey, taskSpecText)
	return ctx
}

// ProjectContextFromContext reads the manifest and task spec text set by
// WithProjectContext. Missing values are empty strings.
func ProjectContextFromContext(ctx context.Context) (projectManifest, taskSpecText string) {
	if v, ok := ctx.Value(projectManifestKey).(string); ok {
		projectManifest = v
	}
	if v, ok := ctx.Value(taskSpecKey).(string); ok {
		taskSpecText = v
	}
	return projectManifest, taskSpecText
}
