package runstore

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"autonomous-agent/internal/logger"
)

// Open returns a Postgres store when dsn is set.
// With an empty dsn and BI_DATABASE_DSN set, runs are stored in MySQL so the history list survives restarts.
// Otherwise it keeps runs in process memory.
func Open(ctx context.Context, dsn string, log logger.Logger) (Store, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		mysqlDSN := strings.TrimSpace(os.Getenv("BI_DATABASE_DSN"))
		if mysqlDSN == "" {
			if log != nil {
				log.InfoContext(ctx, "agent run store using in-process memory")
			}
			return NewMemoryStore(), nil
		}
		store, err := openMySQL(ctx, mysqlDSN)
		if err != nil {
			return nil, err
		}
		if log != nil {
			log.InfoContext(ctx, "agent run store using mysql", "table", "bi_agent_run")
		}
		return store, nil
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("invalid postgres url for agent run store")
	}
	store, err := openPostgres(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if log != nil {
		log.InfoContext(ctx, "agent run store using postgres", "host", u.Host)
	}
	return store, nil
}
