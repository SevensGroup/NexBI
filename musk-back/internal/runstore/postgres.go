package runstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const pgSchema = `
CREATE TABLE IF NOT EXISTS agent_runs (
	id TEXT PRIMARY KEY,
	goal TEXT NOT NULL,
	status TEXT NOT NULL,
	payload JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs (status);
CREATE INDEX IF NOT EXISTS idx_agent_runs_updated_at ON agent_runs (updated_at);
`

type pgStore struct {
	db *sql.DB
}

func openPostgres(ctx context.Context, dsn string) (*pgStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if _, err := db.ExecContext(ctx, pgSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate agent_runs schema: %w", err)
	}
	return &pgStore{db: db}, nil
}

func (s *pgStore) Create(ctx context.Context, run *Run) error {
	if run == nil || strings.TrimSpace(run.ID) == "" {
		return ErrNotFound
	}
	now := time.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now
	if run.Status == "" {
		run.Status = StatusCreated
	}
	return s.upsert(ctx, run)
}

func (s *pgStore) Get(ctx context.Context, id string) (*Run, error) {
	id = strings.TrimSpace(id)
	if s == nil || id == "" {
		return nil, ErrNotFound
	}
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM agent_runs WHERE id = $1`, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var run Run
	if err := json.Unmarshal(payload, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *pgStore) Save(ctx context.Context, run *Run) error {
	if run == nil || strings.TrimSpace(run.ID) == "" {
		return ErrNotFound
	}
	run.UpdatedAt = time.Now().UTC()
	return s.upsert(ctx, run)
}

func (s *pgStore) Update(ctx context.Context, id string, mut func(*Run)) (*Run, error) {
	run, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if mut != nil {
		mut(run)
	}
	run.ID = id
	run.UpdatedAt = time.Now().UTC()
	if err := s.upsert(ctx, run); err != nil {
		return nil, err
	}
	return run.Clone(), nil
}

func (s *pgStore) upsert(ctx context.Context, run *Run) error {
	payload, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO agent_runs (id, goal, status, payload, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
	goal = EXCLUDED.goal,
	status = EXCLUDED.status,
	payload = EXCLUDED.payload,
	updated_at = EXCLUDED.updated_at
`, run.ID, run.Goal, string(run.Status), payload, run.CreatedAt, run.UpdatedAt)
	return err
}

func (s *pgStore) List(ctx context.Context, offset, limit int) (Page, error) {
	if s == nil || s.db == nil {
		return Page{}, nil
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_runs`).Scan(&total); err != nil {
		return Page{}, err
	}
	offset, limit = listWindow(offset, limit)
	rows, err := s.db.QueryContext(ctx, `
SELECT payload FROM agent_runs
ORDER BY updated_at DESC
LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	runs, err := summariesFromPayloads(rows)
	if err != nil {
		return Page{}, err
	}
	return Page{Runs: runs, Total: total}, nil
}

func summariesFromPayloads(rows *sql.Rows) ([]Summary, error) {
	out := make([]Summary, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var run Run
		if err := json.Unmarshal(payload, &run); err != nil {
			return nil, err
		}
		out = append(out, Summarize(&run))
	}
	return out, rows.Err()
}

func (s *pgStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
