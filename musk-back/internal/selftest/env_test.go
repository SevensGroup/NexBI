package selftest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvParsesLLMKeysWithoutRequiringLiveNetwork(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "LLM_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai\nLLM_API_KEY=sk-test-not-real\nLLM_MODEL=gemini-3.5-flash\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := LoadDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if m["LLM_BASE_URL"] != ExpectedBaseURL {
		t.Fatalf("base = %q", m["LLM_BASE_URL"])
	}
	if m["LLM_MODEL"] != ExpectedModel {
		t.Fatalf("model = %q", m["LLM_MODEL"])
	}
	if m["LLM_API_KEY"] == "" {
		t.Fatal("key missing")
	}
}

func TestLoadDotEnvIgnoresComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("# secret\nLLM_MODEL=gemini-3.5-flash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := LoadDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["secret"]; ok {
		t.Fatal("comment leaked")
	}
}
