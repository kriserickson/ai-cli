package cmd

import (
	"net/http"
	"net/http/httptest"
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

	if err := runRoot(nil, []string{"say", "hello"}); err != nil {
		t.Fatalf("runRoot() error: %v", err)
	}
}

func TestRunRootSingleShot_UnexpectedResponseType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"mystery\"}"}}]}`))
	}))
	defer server.Close()

	saveRootConfig(t, server.URL)
	debugFlag = ""

	err := runRoot(nil, []string{"test"})
	if err == nil {
		t.Fatal("runRoot() error = nil, want unexpected response type error")
	}
	if !strings.Contains(err.Error(), "unexpected response type: mystery") {
		t.Fatalf("runRoot() error = %q, want unexpected response type", err.Error())
	}
}

func TestRunRootSingleShot_ConfigResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
