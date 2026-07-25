package profile

import (
	"context"
	"time"

	"github.com/aux-ai/aux-cli/internal/ids"
)

// Service compiles and reads project profiles.
type Service struct {
	store   *Store
	builder *Builder
}

// NewService returns a profile service.
func NewService(store *Store, builder *Builder) *Service {
	return &Service{store: store, builder: builder}
}

// CompileProject builds (or reuses) the project-layer profile version for a
// project root at a given source revision.
func (s *Service) CompileProject(ctx context.Context, projectID, root, sourceRevision string) (Version, []Entry, error) {
	now := time.Now().UnixMilli()
	profile, err := s.store.GetOrCreateProfile(ctx, Profile{
		ID:         ids.New(),
		OwnerType:  OwnerProject,
		OwnerID:    projectID,
		Name:       "project",
		Precedence: Precedence[OwnerProject],
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		return Version{}, nil, err
	}
	return s.builder.Build(ctx, profile.ID, root, sourceRevision)
}

// InputFingerprint returns the current profile-input fingerprint for a root.
func (s *Service) InputFingerprint(ctx context.Context, root string) (string, error) {
	return s.builder.InputFingerprint(ctx, root)
}

// Store exposes the underlying store for read-only callers.
func (s *Service) Store() *Store { return s.store }
