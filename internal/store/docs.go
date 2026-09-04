package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// Brief is a distilled, version-pinned integration guide for one provider.
// It is what enters an agent's context — never the raw docs.
type Brief struct {
	Provider   string
	Version    string
	SourceHash string
	Summary    string
	Auth       string
	Endpoints  []string
	Example    string
}

type briefContent struct {
	Summary   string   `json:"summary"`
	Auth      string   `json:"auth"`
	Endpoints []string `json:"endpoints"`
	Example   string   `json:"example"`
}

// PutBrief upserts a distilled brief, keyed by provider+version.
func (s *Store) PutBrief(ctx context.Context, b Brief) error {
	content, _ := json.Marshal(briefContent{b.Summary, b.Auth, b.Endpoints, b.Example})
	_, err := s.pool.Exec(ctx, `
		INSERT INTO doc_briefs(provider, version, source_hash, content)
		VALUES($1,$2,$3,$4)
		ON CONFLICT (provider, version)
		DO UPDATE SET source_hash=EXCLUDED.source_hash, content=EXCLUDED.content`,
		b.Provider, b.Version, b.SourceHash, content)
	return err
}

// GetBrief returns a cached brief for provider@version, if present.
func (s *Store) GetBrief(ctx context.Context, provider, version string) (Brief, bool, error) {
	var b Brief
	var content []byte
	err := s.pool.QueryRow(ctx,
		`SELECT provider, version, source_hash, content FROM doc_briefs WHERE provider=$1 AND version=$2`,
		provider, version).Scan(&b.Provider, &b.Version, &b.SourceHash, &content)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Brief{}, false, nil
		}
		return Brief{}, false, err
	}
	var c briefContent
	if err := json.Unmarshal(content, &c); err != nil {
		return Brief{}, false, err
	}
	b.Summary, b.Auth, b.Endpoints, b.Example = c.Summary, c.Auth, c.Endpoints, c.Example
	return b, true, nil
}
