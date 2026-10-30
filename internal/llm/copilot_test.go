package llm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
)

func TestCopilotToken_ConfiguredKeyWins(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.Copilot.APIKey = "gho_configured"

	token, err := CopilotToken(cfg)
	if err != nil {
		t.Fatalf("CopilotToken error: %v", err)
	}
	if token != "gho_configured" {
		t.Errorf("token = %q, want configured key", token)
	}
}

func TestCopilotToken_NoGitHubCLI(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cfg := config.DefaultConfig()

	_, err := CopilotToken(cfg)
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("err = %v, want hint to run gh auth login", err)
	}
}

func TestNewClient_CopilotProvider(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"type\":\"command\",\"command\":\"ls\"}"}}]}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = config.ProviderCopilot
	cfg.Provider.Model = "gpt-4.1"
	cfg.Provider.Copilot.BaseURL = server.URL
	cfg.Provider.Copilot.APIKey = "gho_test"

	client, err := NewClient(cfg, nil, "")
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if _, err := client.Chat("sys", "list files"); err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if gotAuth != "Bearer gho_test" {
		t.Errorf("Authorization = %q, want Bearer gho_test", gotAuth)
	}
}

func TestFetchCopilotModels_OnlyPickerModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"id":"gpt-4.1","name":"GPT-4.1","vendor":"Azure OpenAI","model_picker_enabled":true},
			{"id":"text-embedding-3-small","name":"Embedding","vendor":"Azure OpenAI","model_picker_enabled":false}
		]}`))
	}))
	defer server.Close()

	models, err := FetchCopilotModels(server.URL, "gho_test")
	if err != nil {
		t.Fatalf("FetchCopilotModels error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-4.1" {
		t.Errorf("models = %+v, want only gpt-4.1", models)
	}
}
