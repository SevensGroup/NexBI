package loginlog

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryRecordOmitsNothingButKeepsFields(t *testing.T) {
	t.Setenv("BI_DATABASE_DSN", "")
	s, err := Open(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if s.UsesDatabase() {
		t.Fatal("empty dsn should stay in memory")
	}
	entry, err := s.Record(context.Background(), " demo ", false, " 203.0.113.8 ", strings.Repeat("a", 600))
	if err != nil {
		t.Fatal(err)
	}
	if entry.Account != "demo" || entry.Success || entry.IP != "203.0.113.8" {
		t.Fatalf("entry = %+v", entry)
	}
	if len(entry.UserAgent) != 512 {
		t.Fatalf("user agent len = %d", len(entry.UserAgent))
	}
	got, err := s.Entries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("entries = %+v", got)
	}
}
