package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"autonomous-agent/internal/logger"
)

const pgSchema = `
CREATE TABLE IF NOT EXISTS memory (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	content JSONB NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_memory_run_id ON memory (run_id);
CREATE INDEX IF NOT EXISTS idx_memory_kind ON memory (kind);
CREATE INDEX IF NOT EXISTS idx_memory_expires_at ON memory (expires_at);

CREATE TABLE IF NOT EXISTS task_memory (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	task_id TEXT,
	kind TEXT NOT NULL,
	content JSONB NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_task_memory_run_id ON task_memory (run_id);
CREATE INDEX IF NOT EXISTS idx_task_memory_task_id ON task_memory (task_id);
CREATE INDEX IF NOT EXISTS idx_task_memory_kind ON task_memory (kind);
CREATE INDEX IF NOT EXISTS idx_task_memory_expires_at ON task_memory (expires_at);

CREATE TABLE IF NOT EXISTS long_term_memory (
	id TEXT PRIMARY KEY,
	run_id TEXT,
	kind TEXT NOT NULL,
	content JSONB NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_long_term_memory_run_id ON long_term_memory (run_id);
CREATE INDEX IF NOT EXISTS idx_long_term_memory_kind ON long_term_memory (kind);
CREATE INDEX IF NOT EXISTS idx_long_term_memory_expires_at ON long_term_memory (expires_at);
`

type pgStore struct {
	db  *sql.DB
	log logger.Logger
}

func openPostgres(ctx context.Context, dsn string, log logger.Logger) (*pgStore, error) {
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
		return nil, fmt.Errorf("migrate memory schema: %w", err)
	}
	return &pgStore{db: db, log: log}, nil
}

func (s *pgStore) Save(ctx context.Context, rec Record) error {
	content, err := json.Marshal(rec.Content)
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}
	meta, err := json.Marshal(rec.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	var expires any
	if rec.ExpiresAt != nil {
		expires = rec.ExpiresAt.UTC()
	}

	switch rec.Layer {
	case LayerShortTerm:
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO memory (id, run_id, kind, content, metadata, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				run_id = EXCLUDED.run_id,
				kind = EXCLUDED.kind,
				content = EXCLUDED.content,
				metadata = EXCLUDED.metadata,
				created_at = EXCLUDED.created_at,
				expires_at = EXCLUDED.expires_at
		`, rec.ID, rec.RunID, string(rec.Kind), content, meta, rec.CreatedAt.UTC(), expires)
	case LayerTask:
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO task_memory (id, run_id, task_id, kind, content, metadata, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO UPDATE SET
				run_id = EXCLUDED.run_id,
				task_id = EXCLUDED.task_id,
				kind = EXCLUDED.kind,
				content = EXCLUDED.content,
				metadata = EXCLUDED.metadata,
				created_at = EXCLUDED.created_at,
				expires_at = EXCLUDED.expires_at
		`, rec.ID, rec.RunID, nullString(rec.TaskID), string(rec.Kind), content, meta, rec.CreatedAt.UTC(), expires)
	case LayerLongTerm:
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO long_term_memory (id, run_id, kind, content, metadata, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				run_id = EXCLUDED.run_id,
				kind = EXCLUDED.kind,
				content = EXCLUDED.content,
				metadata = EXCLUDED.metadata,
				created_at = EXCLUDED.created_at,
				expires_at = EXCLUDED.expires_at
		`, rec.ID, nullString(rec.RunID), string(rec.Kind), content, meta, rec.CreatedAt.UTC(), expires)
	default:
		return ErrInvalidLayer
	}
	if err != nil {
		return fmt.Errorf("save memory: %w", err)
	}
	return nil
}

func (s *pgStore) Get(ctx context.Context, id string) (*Record, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, run_id, kind, content, metadata, created_at, expires_at, NULL::text AS task_id, 'short_term' AS layer
		FROM memory WHERE id = $1
		UNION ALL
		SELECT id, run_id, kind, content, metadata, created_at, expires_at, task_id, 'task' AS layer
		FROM task_memory WHERE id = $1
		UNION ALL
		SELECT id, run_id, kind, content, metadata, created_at, expires_at, NULL::text AS task_id, 'long_term' AS layer
		FROM long_term_memory WHERE id = $1
		LIMIT 1
	`, id)
	rec, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *pgStore) Search(ctx context.Context, q Query) (SearchResult, error) {
	where, args := searchWhere(q)
	countSQL := `SELECT COUNT(*) FROM (` + unionSQL() + `) AS m WHERE ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return SearchResult{}, fmt.Errorf("count memory: %w", err)
	}

	args = append(args, q.Limit, q.Offset)
	listSQL := `SELECT id, run_id, kind, content, metadata, created_at, expires_at, task_id, layer
		FROM (` + unionSQL() + `) AS m WHERE ` + where + `
		ORDER BY created_at DESC, id ASC
		LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := s.db.QueryContext(ctx, listSQL, args...)
	if err != nil {
		return SearchResult{}, fmt.Errorf("search memory: %w", err)
	}
	defer rows.Close()

	out := make([]Record, 0)
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return SearchResult{}, err
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Records: out, Total: total, Limit: q.Limit, Offset: q.Offset}, nil
}

func (s *pgStore) Delete(ctx context.Context, id string) error {
	n, err := s.deleteSequential(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) deleteSequential(ctx context.Context, id string) (int64, error) {
	var total int64
	for _, q := range []string{
		`DELETE FROM memory WHERE id = $1`,
		`DELETE FROM task_memory WHERE id = $1`,
		`DELETE FROM long_term_memory WHERE id = $1`,
	} {
		res, err := s.db.ExecContext(ctx, q, id)
		if err != nil {
			return total, fmt.Errorf("delete memory: %w", err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

func (s *pgStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func unionSQL() string {
	return `
		SELECT id, run_id, kind, content, metadata, created_at, expires_at, NULL::text AS task_id, 'short_term'::text AS layer FROM memory
		UNION ALL
		SELECT id, run_id, kind, content, metadata, created_at, expires_at, task_id, 'task'::text AS layer FROM task_memory
		UNION ALL
		SELECT id, run_id, kind, content, metadata, created_at, expires_at, NULL::text AS task_id, 'long_term'::text AS layer FROM long_term_memory
	`
}

func searchWhere(q Query) (string, []any) {
	clauses := []string{"(expires_at IS NULL OR expires_at > NOW())"}
	args := make([]any, 0, 8)

	add := func(clause string, v any) {
		args = append(args, v)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}

	if q.RunID != "" {
		add("run_id = $%d", q.RunID)
	}
	if q.Layer != "" {
		add("layer = $%d", string(q.Layer))
	}
	if q.Kind != "" {
		add("kind = $%d", string(q.Kind))
	}
	if q.TaskID != "" {
		add("task_id = $%d", q.TaskID)
	}
	if q.Text != "" {
		args = append(args, likePattern(q.Text))
		n := len(args)
		clauses = append(clauses, fmt.Sprintf("(content::text ILIKE $%d ESCAPE '\\' OR metadata::text ILIKE $%d ESCAPE '\\' OR kind ILIKE $%d ESCAPE '\\')", n, n, n))
	}
	for k, v := range q.Metadata {
		args = append(args, k, fmt.Sprint(v))
		clauses = append(clauses, fmt.Sprintf("metadata ->> $%d = $%d", len(args)-1, len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func likePattern(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return `%` + q + `%`
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(row scanner) (*Record, error) {
	var (
		id, runID, kind, layer string
		taskID                 sql.NullString
		contentRaw, metaRaw    []byte
		created                time.Time
		expires                sql.NullTime
	)
	if err := row.Scan(&id, &runID, &kind, &contentRaw, &metaRaw, &created, &expires, &taskID, &layer); err != nil {
		return nil, err
	}
	var content any
	if len(contentRaw) > 0 {
		if err := json.Unmarshal(contentRaw, &content); err != nil {
			return nil, fmt.Errorf("decode content: %w", err)
		}
	}
	meta := map[string]any{}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			return nil, fmt.Errorf("decode metadata: %w", err)
		}
	}
	rec := &Record{
		ID:        id,
		RunID:     runID,
		TaskID:    taskID.String,
		Layer:     Layer(layer),
		Kind:      Kind(kind),
		Content:   content,
		Metadata:  meta,
		CreatedAt: created.UTC(),
	}
	if expires.Valid {
		t := expires.Time.UTC()
		rec.ExpiresAt = &t
	}
	return rec, nil
}

func nullString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
