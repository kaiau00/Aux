package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aux-ai/aux-cli/internal/db"
)

// Store persists profiles, versions, and entries.
type Store struct {
	db db.DBTX
}

// NewStore returns a profile store backed by the given database handle.
func NewStore(dbtx db.DBTX) *Store {
	return &Store{db: dbtx}
}

// GetOrCreateProfile returns the profile for (ownerType, ownerID, name), creating it if absent.
func (s *Store) GetOrCreateProfile(ctx context.Context, p Profile) (Profile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT profile_id, owner_type, owner_id, name, precedence, enabled, created_at, updated_at
         FROM profiles WHERE owner_type = ? AND owner_id = ? AND name = ?`,
		p.OwnerType, p.OwnerID, p.Name)
	existing, err := scanProfile(row)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Profile{}, fmt.Errorf("failed to look up profile: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO profiles (profile_id, owner_type, owner_id, name, precedence, enabled, created_at, updated_at)
         VALUES (?,?,?,?,?,?,?,?)`,
		p.ID, p.OwnerType, p.OwnerID, p.Name, p.Precedence, boolToInt(p.Enabled), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return Profile{}, fmt.Errorf("failed to create profile: %w", err)
	}
	return p, nil
}

// GetVersionByContentHash returns a version with the given content hash, if any.
func (s *Store) GetVersionByContentHash(ctx context.Context, profileID, contentHash string) (Version, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT profile_version_id, profile_id, source_revision, content_hash, compiler_version, created_at
         FROM profile_versions WHERE profile_id = ? AND content_hash = ?`, profileID, contentHash)
	v, err := scanVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Version{}, false, nil
	}
	if err != nil {
		return Version{}, false, err
	}
	return v, true, nil
}

// InsertVersion inserts a new profile version and its entries in one transaction-like batch.
func (s *Store) InsertVersion(ctx context.Context, v Version, entries []Entry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO profile_versions (profile_version_id, profile_id, source_revision, content_hash, compiler_version, created_at)
         VALUES (?,?,?,?,?,?)`,
		v.ID, v.ProfileID, v.SourceRevision, v.ContentHash, v.CompilerVersion, v.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert profile version: %w", err)
	}
	for _, e := range entries {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO profile_entries (entry_id, profile_version_id, entry_type, entry_key, value_json, source_type, source_ref, confidence, token_estimate)
             VALUES (?,?,?,?,?,?,?,?,?)`,
			e.ID, v.ID, e.Type, e.Key, e.ValueJSON, e.SourceType, e.SourceRef, e.Confidence, e.TokenEstimate)
		if err != nil {
			return fmt.Errorf("failed to insert profile entry: %w", err)
		}
	}
	return nil
}

// ListEntries returns the entries of a profile version.
func (s *Store) ListEntries(ctx context.Context, versionID string) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT entry_id, profile_version_id, entry_type, entry_key, value_json, source_type, source_ref, confidence, token_estimate
         FROM profile_entries WHERE profile_version_id = ? ORDER BY entry_type, entry_key`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ProfileVersionID, &e.Type, &e.Key, &e.ValueJSON, &e.SourceType, &e.SourceRef, &e.Confidence, &e.TokenEstimate); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LatestVersion returns the most recent version for a profile.
func (s *Store) LatestVersion(ctx context.Context, profileID string) (Version, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT profile_version_id, profile_id, source_revision, content_hash, compiler_version, created_at
         FROM profile_versions WHERE profile_id = ? ORDER BY created_at DESC LIMIT 1`, profileID)
	v, err := scanVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Version{}, false, nil
	}
	if err != nil {
		return Version{}, false, err
	}
	return v, true, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProfile(row scanner) (Profile, error) {
	var p Profile
	var enabled int
	if err := row.Scan(&p.ID, &p.OwnerType, &p.OwnerID, &p.Name, &p.Precedence, &enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return Profile{}, err
	}
	p.Enabled = enabled != 0
	return p, nil
}

func scanVersion(row scanner) (Version, error) {
	var v Version
	if err := row.Scan(&v.ID, &v.ProfileID, &v.SourceRevision, &v.ContentHash, &v.CompilerVersion, &v.CreatedAt); err != nil {
		return Version{}, err
	}
	return v, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
