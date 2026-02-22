package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
)

func TestNewClient_MissingAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.APIKey = ""

	_, err := NewClient(cfg, nil)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "no API key") {
		t.Errorf("error = %q, want it to mention missing API key", err.Error())
	}
}

func TestNewClient_UnknownProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.Default = "invalid"

	_, err := NewClient(cfg, nil)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error = %q, want it to mention unknown provider", err.Error())
	}
}

func TestChat_Success(t *testing.T) {
	llmResponse := Response{
		Type: "commands",
		Commands: []Command{
			{Command: "ls", Description: "list files", Risk: "safe", Certainty: 95},
		},
	}
	respContent, _ := json.Marshal(llmResponse)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("path = %s, want /chat/completions", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q, want %q", r.Header.Get("Authorization"), "Bearer test-key")
		}

		chatResp := ChatResponse{
			Choices: []Choice{{Message: Message{Content: string(respContent)}}},
		}
		if err := json.NewEncoder(w).Encode(chatResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.APIKey = "test-key"
	cfg.Provider.OpenAI.BaseURL = server.URL

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	resp, err := client.Chat("system prompt", "list files")
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want %q", resp.Type, "commands")
	}
	if resp.Commands[0].Command != "ls" {
		t.Errorf("command = %q, want %q", resp.Commands[0].Command, "ls")
	}
}

func TestChat_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "invalid api key"}}`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.APIKey = "bad-key"
	cfg.Provider.OpenAI.BaseURL = server.URL

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, err = client.Chat("system", "test")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want it to mention 401", err.Error())
	}
}

func TestChat_DebugOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chatResp := ChatResponse{
			Choices: []Choice{{Message: Message{Content: `{"type":"commands","commands":[{"command":"pwd","description":"d","risk":"safe","certainty":99}]}`}}},
		}
		if err := json.NewEncoder(w).Encode(chatResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.APIKey = "test-key"
	cfg.Provider.OpenAI.BaseURL = server.URL

	var buf strings.Builder
	client, err := NewClient(cfg, &buf)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, err = client.Chat("sys", "test")
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "--- REQUEST ---") {
		t.Error("debug output missing REQUEST section")
	}
	if !strings.Contains(output, "--- RESPONSE") {
		t.Error("debug output missing RESPONSE section")
	}
	if !strings.Contains(output, "chat/completions") {
		t.Error("debug output missing endpoint URL")
	}
}

func TestChat_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chatResp := ChatResponse{Choices: []Choice{}}
		if err := json.NewEncoder(w).Encode(chatResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.APIKey = "test-key"
	cfg.Provider.OpenAI.BaseURL = server.URL

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "no response") {
		t.Errorf("error = %q, want mention of no response", err.Error())
	}
}
