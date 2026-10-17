package runstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const mysqlSchema = `
CREATE TABLE IF NOT EXISTS bi_agent_run (
	id VARCHAR(128) NOT NULL,
	goal TEXT NOT NULL,
	status VARCHAR(32) NOT NULL,
	payload JSON NOT NULL,
	created_time DATETIME(6) NOT NULL,
	updated_time DATETIME(6) NOT NULL,
	PRIMARY KEY (id),
	KEY idx_bi_agent_run_updated (updated_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

type mysqlStore struct {
	db *sql.DB
}

func openMySQL(ctx context.Context, dsn string) (*mysqlStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open run database: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping run database: %w", err)
	}
	if _, err := db.ExecContext(ctx, mysqlSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate bi_agent_run: %w", err)
	}
	return &mysqlStore{db: db}, nil
}

func (s *mysqlStore) Create(ctx context.Context, run *Run) error {
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

func (s *mysqlStore) Get(ctx context.Context, id string) (*Run, error) {
	id = strings.TrimSpace(id)
	if s == nil || s.db == nil || id == "" {
		return nil, ErrNotFound
	}
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM bi_agent_run WHERE id = ?`, id).Scan(&payload)
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

func (s *mysqlStore) Save(ctx context.Context, run *Run) error {
	if run == nil || strings.TrimSpace(run.ID) == "" {
		return ErrNotFound
	}
	run.UpdatedAt = time.Now().UTC()
	return s.upsert(ctx, run)
}

func (s *mysqlStore) Update(ctx context.Context, id string, mut func(*Run)) (*Run, error) {
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

func (s *mysqlStore) List(ctx context.Context, offset, limit int) (Page, error) {
	if s == nil || s.db == nil {
		return Page{}, nil
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bi_agent_run`).Scan(&total); err != nil {
		return Page{}, err
	}
	offset, limit = listWindow(offset, limit)
	rows, err := s.db.QueryContext(ctx, `
SELECT payload FROM bi_agent_run
ORDER BY updated_time DESC
LIMIT ? OFFSET ?`, limit, offset)
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

func (s *mysqlStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *mysqlStore) upsert(ctx context.Context, run *Run) error {
	payload, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO bi_agent_run (id, goal, status, payload, created_time, updated_time)
VALUES (?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	goal = VALUES(goal),
	status = VALUES(status),
	payload = VALUES(payload),
	updated_time = VALUES(updated_time)`,
		run.ID, run.Goal, string(run.Status), payload, run.CreatedAt, run.UpdatedAt)
	return err
}
