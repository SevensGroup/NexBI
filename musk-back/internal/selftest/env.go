package selftest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ExpectedBaseURL is musk-back/.env LLM_BASE_URL (Gemini OpenAI-compatible endpoint).
	ExpectedBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	// ExpectedModel is musk-back/.env LLM_MODEL.
	ExpectedModel = "gemini-3.5-flash"
	// GoalMessage is the production agent goal.
	GoalMessage    = "Analyze why Meta ad ROI declined yesterday"
	placeholderKey = "sk-your-key"
)

// LLMEnv is loaded from musk-back/.env at runtime. Never log APIKey.
type LLMEnv struct {
	BaseURL string
	APIKey  string
	Model   string
	Path    string
}

// FindEnvFile locates musk-back/.env without embedding secrets.
func FindEnvFile() (string, error) {
	if p := strings.TrimSpace(os.Getenv("MUSK_BACK_ENV")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("MUSK_BACK_ENV=%s: %w", p, err)
		}
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		nested := filepath.Join(dir, "musk-back", ".env")
		if _, err := os.Stat(nested); err == nil {
			return nested, nil
		}
		if filepath.Base(dir) == "musk-back" {
			p := filepath.Join(dir, ".env")
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("musk-back/.env not found; copy musk-back/.env.example and set LLM_* (do not commit .env)")
}

// LoadDotEnv parses KEY=VALUE lines. Values are not logged.
func LoadDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	return out, sc.Err()
}

// LoadLLMEnv reads and validates Gemini settings from .env.
func LoadLLMEnv() (LLMEnv, error) {
	path, err := FindEnvFile()
	if err != nil {
		return LLMEnv{}, err
	}
	m, err := LoadDotEnv(path)
	if err != nil {
		return LLMEnv{}, err
	}
	env := LLMEnv{
		BaseURL: strings.TrimRight(strings.TrimSpace(first(m["LLM_BASE_URL"], os.Getenv("LLM_BASE_URL"))), "/"),
		APIKey:  strings.TrimSpace(first(m["LLM_API_KEY"], os.Getenv("LLM_API_KEY"))),
		Model:   strings.TrimSpace(first(m["LLM_MODEL"], os.Getenv("LLM_MODEL"))),
		Path:    path,
	}
	if env.BaseURL != ExpectedBaseURL {
		return LLMEnv{}, fmt.Errorf("LLM_BASE_URL must be %s (musk-back/.env line 1), got %q", ExpectedBaseURL, env.BaseURL)
	}
	if env.Model != ExpectedModel {
		return LLMEnv{}, fmt.Errorf("LLM_MODEL must be %s (musk-back/.env line 3), got %q", ExpectedModel, env.Model)
	}
	if env.APIKey == "" || env.APIKey == placeholderKey {
		return LLMEnv{}, fmt.Errorf("LLM_API_KEY missing or placeholder in %s (do not commit the key)", path)
	}
	return env, nil
}

func first(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
