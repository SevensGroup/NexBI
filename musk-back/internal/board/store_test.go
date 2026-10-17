package board

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestAdsTotalsMatchSeed(t *testing.T) {
	view := New("http://localhost:5173").Ads()
	totals := view["totals"].(map[string]any)
	if totals["spend"] != "274200.00" || totals["revenue"] != "491900.00" || totals["roas"] != "1.79" {
		t.Fatalf("totals = %#v", totals)
	}
	list := view["list"].([]map[string]any)
	if len(list) != 12 {
		t.Fatalf("rows = %d", len(list))
	}
	if list[0]["date"] != "2026-10-14" || list[0]["campaign"] != "Brand Autumn Launch" {
		t.Fatalf("first row = %#v", list[0])
	}
}

func TestFillCreateUsesQueryRows(t *testing.T) {
	raw, err := json.Marshal(New("http://localhost:5173").Ads())
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"action": "create"}
	FillCreate(input, `{"board":`+string(raw)+`}`, "Create the board")
	rows, _ := input["rows"].([]any)
	if input["name"] != "Advertising performance" || len(rows) != 12 {
		t.Fatalf("filled = %#v", input["name"])
	}
	if _, ok := input["summary"].(string); !ok || !strings.Contains(input["summary"].(string), "274200.00") {
		t.Fatalf("summary = %#v", input["summary"])
	}
}

func TestCreatePublishesCodeURL(t *testing.T) {
	s := New("http://localhost:5173")
	got, err := s.Create("Ads", "Meta leads spend", []Metric{{Label: "Spend", Value: "274200.00"}}, []string{"channel"}, []map[string]string{{"channel": "Meta"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Code) != 11 {
		t.Fatalf("code = %q", got.Code)
	}
	for _, ch := range got.Code {
		if ch < 'a' || ch > 'z' {
			t.Fatalf("code = %q", got.Code)
		}
	}
	other, err := s.Create("Ads", "again", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if other.Code == got.Code {
		t.Fatalf("codes collided: %s", got.Code)
	}
	if got.URL != "http://localhost:5173/bi/"+got.Code {
		t.Fatalf("url = %q", got.URL)
	}
	loaded, ok := s.Get(got.Code)
	if !ok || loaded.Name != "Ads" {
		t.Fatalf("loaded = %#v ok=%v", loaded, ok)
	}
}

func TestMySQLPersistsRandomLink(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BI_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("BI_DATABASE_DSN not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, dsn, "http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	got, err := s.Create("Ads", "stored link", []Metric{{Label: "roas", Value: "1.79"}}, []string{"channel"}, []map[string]string{{"channel": "Meta"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.db.ExecContext(context.Background(), `DELETE FROM bi_public_board WHERE code = ?`, got.Code)
	})
	fresh, err := Open(ctx, dsn, "http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fresh.Close() })
	loaded, ok := fresh.Get(got.Code)
	if !ok || loaded.URL != got.URL || loaded.Name != "Ads" {
		t.Fatalf("loaded = %#v ok=%v", loaded, ok)
	}
}
