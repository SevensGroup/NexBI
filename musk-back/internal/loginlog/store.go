package loginlog

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Entry is one sign-in attempt. The password is never stored.
type Entry struct {
	ID        int64     `json:"id"`
	Account   string    `json:"account"`
	Success   bool      `json:"success"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

// Store writes sign-in attempts. With BI_DATABASE_DSN set, rows go to MySQL.
// An empty DSN keeps them in memory so local runs and tests still record them.
type Store struct {
	mu    sync.Mutex
	db    *sql.DB
	items []Entry
}

const loginTableSQL = `
CREATE TABLE IF NOT EXISTS bi_login_log (
	id BIGINT NOT NULL AUTO_INCREMENT,
	account VARCHAR(64) NOT NULL,
	success TINYINT NOT NULL,
	ip VARCHAR(64) NOT NULL,
	user_agent VARCHAR(512) NOT NULL,
	created_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	KEY idx_bi_login_log_created (created_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

// Open connects when dsn is set. An empty dsn reads BI_DATABASE_DSN, then stays in memory.
func Open(ctx context.Context, dsn string) (*Store, error) {
	s := &Store{}
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("BI_DATABASE_DSN"))
	}
	if dsn == "" {
		return s, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open login log database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping login log database: %w", err)
	}
	if _, err := db.ExecContext(ctx, loginTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate bi_login_log: %w", err)
	}
	s.db = db
	return s, nil
}

// UsesDatabase reports whether attempts are written to MySQL.
func (s *Store) UsesDatabase() bool {
	return s != nil && s.db != nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Record appends one attempt. account, ip, and userAgent are truncated to the column widths.
func (s *Store) Record(ctx context.Context, account string, success bool, ip, userAgent string) (Entry, error) {
	if s == nil {
		return Entry{}, fmt.Errorf("login log store is nil")
	}
	entry := Entry{
		Account:   clip(strings.TrimSpace(account), 64),
		Success:   success,
		IP:        clip(strings.TrimSpace(ip), 64),
		UserAgent: clip(strings.TrimSpace(userAgent), 512),
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		entry.ID = int64(len(s.items) + 1)
		s.items = append(s.items, entry)
		return entry, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ok := 0
	if entry.Success {
		ok = 1
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO bi_login_log (account, success, ip, user_agent, created_time)
VALUES (?, ?, ?, ?, ?)`, entry.Account, ok, entry.IP, entry.UserAgent, entry.CreatedAt)
	if err != nil {
		return Entry{}, fmt.Errorf("insert bi_login_log: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Entry{}, fmt.Errorf("login log id: %w", err)
	}
	entry.ID = id
	return entry, nil
}

// Entries returns recorded attempts, newest last. Database mode reads the latest 200 rows.
func (s *Store) Entries(ctx context.Context) ([]Entry, error) {
	if s == nil {
		return nil, nil
	}
	if s.db == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		out := make([]Entry, len(s.items))
		copy(out, s.items)
		return out, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, account, success, ip, user_agent, created_time
FROM bi_login_log
ORDER BY id DESC
LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("list bi_login_log: %w", err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var entry Entry
		var ok int
		if err := rows.Scan(&entry.ID, &entry.Account, &ok, &entry.IP, &entry.UserAgent, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan bi_login_log: %w", err)
		}
		entry.Success = ok != 0
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func clip(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
