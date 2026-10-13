package runstore

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"autonomous-agent/internal/logger"
)

// Open returns a Postgres-backed store when dsn is set, otherwise in-process memory.
func Open(ctx context.Context, dsn string, log logger.Logger) (Store, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		if log != nil {
			log.InfoContext(ctx, "agent run store using in-process memory")
		}
		return NewMemoryStore(), nil
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
