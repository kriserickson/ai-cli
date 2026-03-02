package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
)

func saveRootConfig(t *testing.T, serverURL string) {
	t.Helper()
	tempHome(t)

	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.Model = "gpt-4o-mini"
	cfg.Provider.OpenAI.APIKey = "sk-test"
	cfg.Provider.OpenAI.BaseURL = serverURL
	cfg.Debug = "none"

	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
}

func TestRunRootSingleShot_CommandsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"commands\",\"commands\":[]}"}}]}`))
	}))
	defer server.Close()

	saveRootConfig(t, server.URL)
	debugFlag = ""
	explainFlag = false

	if err := runRoot(nil, []string{"say", "hello"}); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
}

func TestRunRootSingleShot_UnexpectedResponseType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"mystery\"}"}}]}`))
	}))
	defer server.Close()

	saveRootConfig(t, server.URL)
	debugFlag = ""
	explainFlag = false

	err := runRoot(nil, []string{"test"})
	if err == nil {
		t.Fatal("runRoot() error = nil, want unexpected response type error")
	}
	if !strings.Contains(err.Error(), "unexpected response type: mystery") {
		t.Fatalf("runRoot() error = %q, want unexpected response type", err.Error())
	}
}

func TestRunRootSingleShot_DebugFromConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"commands\",\"commands\":[]}"}}]}`))
	}))
	defer server.Close()

	tempHome(t)
	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderOpenAI
	cfg.Provider.Model = "gpt-4o-mini"
	cfg.Provider.OpenAI.APIKey = "sk-test"
	cfg.Provider.OpenAI.BaseURL = server.URL
	cfg.Debug = "screen" // Set debug in config
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	debugFlag = ""

	if err := runRoot(nil, []string{"hello"}); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
}

func TestRunRootSingleShot_WithDebugFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"commands\",\"commands\":[]}"}}]}`))
	}))
	defer server.Close()

	saveRootConfig(t, server.URL)
	debugFlag = "screen"
	defer func() { debugFlag = "" }()

	if err := runRoot(nil, []string{"say", "hello"}); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
}

func TestRunRootSingleShot_WithMemories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"commands\",\"commands\":[]}"}}]}`))
	}))
	defer server.Close()

	saveRootConfig(t, server.URL)

	// Add a memory to test the injection path
	home, _ := os.UserHomeDir()
	memDir := filepath.Join(home, ".ai-cli")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "memories.json"), []byte(`[{"keyword":"docker","content":"use docker compose v2"}]`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	debugFlag = ""
	// Use a query containing the keyword to trigger memory matching
	if err := runRoot(nil, []string{"run", "docker", "compose"}); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
}

func TestRunRootSingleShot_ConfigResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"config\",\"action\":\"set_model\",\"key\":\"\",\"value\":\"gpt-4o\"}"}}]}`))
	}))
	defer server.Close()

	saveRootConfig(t, server.URL)
	debugFlag = ""

	withCmdStdin(t, "n\n", func() {
		if err := runRoot(nil, []string{"switch", "model"}); err != nil {
			t.Fatalf("runRoot() error: %v", err)
		}
	})
}

func TestRunRootSingleShot_WithExplainFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"commands\",\"commands\":[{\"command\":\"echo hello\",\"description\":\"say hello\",\"risk\":\"safe\",\"certainty\":99,\"explanation\":\"Prints hello to stdout.\"}]}"}}]}`))
	}))
	defer server.Close()

	saveRootConfig(t, server.URL)
	debugFlag = ""
	explainFlag = true
	defer func() { explainFlag = false }()

	if err := runRoot(nil, []string{"say", "hello"}); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
}
