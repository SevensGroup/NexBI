package board

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
)

// Metric is one KPI card on a published board.
type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Board is a published dashboard addressed by a public code.
type Board struct {
	Code    string              `json:"code"`
	Name    string              `json:"name"`
	Summary string              `json:"summary"`
	Metrics []Metric            `json:"metrics"`
	Columns []string            `json:"columns"`
	Rows    []map[string]string `json:"rows"`
	URL     string              `json:"url"`
}

// Store keeps published boards. With BI_DATABASE_DSN set, each link is stored in MySQL.
type Store struct {
	mu    sync.Mutex
	base  string
	items map[string]*Board
	db    *sql.DB
}

// New builds a store. An empty publicBase reads BI_PUBLIC_BASE_URL, then http://localhost:5173.
func New(publicBase string) *Store {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("BI_PUBLIC_BASE_URL")), "/")
	}
	if base == "" {
		base = "http://localhost:5173"
	}
	return &Store{base: base, items: map[string]*Board{}}
}

const boardTableSQL = `
CREATE TABLE IF NOT EXISTS bi_public_board (
	code VARCHAR(16) NOT NULL,
	name VARCHAR(255) NOT NULL,
	summary TEXT,
	url VARCHAR(512) NOT NULL,
	payload JSON NOT NULL,
	created_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

// Open connects to MySQL when dsn is set and stores every published link there.
// An empty dsn keeps boards in memory. dsn defaults to BI_DATABASE_DSN.
func Open(ctx context.Context, dsn, publicBase string) (*Store, error) {
	s := New(publicBase)
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
		return nil, fmt.Errorf("open board database: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping board database: %w", err)
	}
	if _, err := db.ExecContext(ctx, boardTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate bi_public_board: %w", err)
	}
	s.db = db
	return s, nil
}

// UsesDatabase reports whether published links are written to MySQL.
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

// Ads returns the advertising table the agent queries before it creates a board.
func (s *Store) Ads() map[string]any {
	rows := seedAds()
	list := make([]map[string]any, 0, len(rows))
	var spend, revenue float64
	var impressions, clicks, conversions int64
	for _, row := range rows {
		spend += row.spend
		revenue += row.revenue
		impressions += row.impressions
		clicks += row.clicks
		conversions += row.conversions
		list = append(list, map[string]any{
			"date":        row.date,
			"channel":     row.channel,
			"campaign":    row.campaign,
			"spend":       money(row.spend),
			"revenue":     money(row.revenue),
			"impressions": row.impressions,
			"clicks":      row.clicks,
			"conversions": row.conversions,
			"roas":        ratio(row.revenue, row.spend),
			"ctr":         pct(float64(row.clicks), float64(row.impressions)),
		})
	}
	return map[string]any{
		"list": list,
		"totals": map[string]any{
			"spend":       money(spend),
			"revenue":     money(revenue),
			"impressions": impressions,
			"clicks":      clicks,
			"conversions": conversions,
			"roas":        ratio(revenue, spend),
			"ctr":         pct(float64(clicks), float64(impressions)),
		},
	}
}

// Create publishes a board and returns it with a /bi/<code> URL.
func (s *Store) Create(name, summary string, metrics []Metric, columns []string, rows []map[string]string) (*Board, error) {
	if s == nil {
		return nil, fmt.Errorf("board store is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if metrics == nil {
		metrics = []Metric{}
	}
	if columns == nil {
		columns = []string{}
	}
	if rows == nil {
		rows = []map[string]string{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var item *Board
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		code, codeErr := newCode()
		if codeErr != nil {
			return nil, codeErr
		}
		if _, exists := s.items[code]; exists {
			continue
		}
		item = &Board{
			Code:    code,
			Name:    name,
			Summary: strings.TrimSpace(summary),
			Metrics: metrics,
			Columns: columns,
			Rows:    rows,
			URL:     s.base + "/bi/" + code,
		}
		if s.db == nil {
			break
		}
		err = s.insert(item)
		if err == nil {
			break
		}
		if !isDuplicate(err) {
			return nil, err
		}
		item = nil
	}
	if item == nil {
		return nil, fmt.Errorf("could not allocate a board code")
	}
	s.items[item.Code] = item
	return item, nil
}

func (s *Store) insert(item *Board) error {
	body, err := json.Marshal(struct {
		Metrics []Metric            `json:"metrics"`
		Columns []string            `json:"columns"`
		Rows    []map[string]string `json:"rows"`
	}{item.Metrics, item.Columns, item.Rows})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(context.Background(), `
		INSERT INTO bi_public_board (code, name, summary, url, payload)
		VALUES (?, ?, ?, ?, ?)
	`, item.Code, item.Name, item.Summary, item.URL, body)
	if err != nil {
		return fmt.Errorf("store board link: %w", err)
	}
	return nil
}

// FillCreate copies the latest ads query into a create call that omitted the table.
func FillCreate(input map[string]any, queryJSON, summaryHint string) {
	if input == nil || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(input["action"])), "create") {
		return
	}
	if strings.TrimSpace(fmt.Sprint(input["name"])) != "" && fmt.Sprint(input["name"]) != "<nil>" && present(input["rows"]) {
		return
	}
	var env struct {
		Board struct {
			List   []map[string]any `json:"list"`
			Totals map[string]any   `json:"totals"`
		} `json:"board"`
	}
	if err := json.Unmarshal([]byte(queryJSON), &env); err != nil || len(env.Board.List) == 0 {
		return
	}
	if strings.TrimSpace(fmt.Sprint(input["name"])) == "" || fmt.Sprint(input["name"]) == "<nil>" {
		input["name"] = "Advertising performance"
	}
	if strings.TrimSpace(fmt.Sprint(input["summary"])) == "" || fmt.Sprint(input["summary"]) == "<nil>" {
		input["summary"] = createSummary(env.Board.Totals, summaryHint)
	}
	if !present(input["metrics"]) {
		input["metrics"] = metricsFromTotals(env.Board.Totals)
	}
	if !present(input["columns"]) {
		input["columns"] = []any{"date", "channel", "campaign", "spend", "revenue", "impressions", "clicks", "conversions", "roas", "ctr"}
	}
	if !present(input["rows"]) {
		rows := make([]any, 0, len(env.Board.List))
		for _, row := range env.Board.List {
			rows = append(rows, row)
		}
		input["rows"] = rows
	}
}

func present(v any) bool {
	switch typed := v.(type) {
	case nil:
		return false
	case []any:
		return len(typed) > 0
	case []map[string]any:
		return len(typed) > 0
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		return text != "" && text != "<nil>"
	}
}

func metricsFromTotals(totals map[string]any) []any {
	order := []string{"spend", "revenue", "roas", "clicks", "impressions", "conversions", "ctr"}
	out := make([]any, 0, len(order))
	for _, key := range order {
		value, ok := totals[key]
		if !ok || value == nil {
			continue
		}
		out = append(out, map[string]any{"label": key, "value": fmt.Sprint(value)})
	}
	return out
}

func createSummary(totals map[string]any, hint string) string {
	hint = strings.TrimSpace(hint)
	if strings.ContainsAny(hint, "0123456789") && (strings.Contains(strings.ToLower(hint), "meta") || strings.Contains(strings.ToLower(hint), "tiktok") || strings.Contains(strings.ToLower(hint), "google")) {
		return hint
	}
	spend := fmt.Sprint(totals["spend"])
	roas := fmt.Sprint(totals["roas"])
	return "Meta, TikTok, and Google ads. Total spend is " + spend + " and ROAS is " + roas + "."
}

// Get returns a published board by its public code.
func (s *Store) Get(code string) (*Board, bool) {
	if s == nil {
		return nil, false
	}
	code = strings.TrimSpace(code)
	s.mu.Lock()
	item, ok := s.items[code]
	db := s.db
	s.mu.Unlock()
	if ok {
		return item, true
	}
	if db == nil || code == "" {
		return nil, false
	}
	loaded, found, err := s.load(code)
	if err != nil || !found {
		return nil, false
	}
	s.mu.Lock()
	s.items[code] = loaded
	s.mu.Unlock()
	return loaded, true
}

func (s *Store) load(code string) (*Board, bool, error) {
	var name, summary, url, payload string
	err := s.db.QueryRowContext(context.Background(), `
		SELECT name, summary, url, payload FROM bi_public_board WHERE code = ?
	`, code).Scan(&name, &summary, &url, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var body struct {
		Metrics []Metric            `json:"metrics"`
		Columns []string            `json:"columns"`
		Rows    []map[string]string `json:"rows"`
	}
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return nil, false, err
	}
	return &Board{
		Code:    code,
		Name:    name,
		Summary: summary,
		Metrics: body.Metrics,
		Columns: body.Columns,
		Rows:    body.Rows,
		URL:     url,
	}, true, nil
}

// List returns published boards, newest last.
func (s *Store) List() []*Board {
	if s == nil {
		return []*Board{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Board, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	return out
}

func newCode() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	raw := make([]byte, 11)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	buf := make([]byte, len(raw))
	for i := range raw {
		buf[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(buf), nil
}

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
