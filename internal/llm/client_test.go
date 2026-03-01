package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
)

func TestDebugWriter_Screen(t *testing.T) {
	w, cleanup, err := DebugWriter("screen")
	if err != nil {
		t.Fatalf("DebugWriter(screen) error: %v", err)
	}
	defer cleanup()
	if w == nil {
		t.Fatal("DebugWriter(screen) returned nil writer")
	}
}

func TestDebugWriter_File(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Ensure config dir exists
	configDir, _ := config.Dir()
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	w, cleanup, err := DebugWriter("file")
	if err != nil {
		t.Fatalf("DebugWriter(file) error: %v", err)
	}
	defer cleanup()
	if w == nil {
		t.Fatal("DebugWriter(file) returned nil writer")
	}

	// Write something and verify
	_, err = w.Write([]byte("test log entry\n"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
}

func TestDebugWriter_None(t *testing.T) {
	w, cleanup, err := DebugWriter("none")
	if err != nil {
		t.Fatalf("DebugWriter(none) error: %v", err)
	}
	defer cleanup()
	if w != nil {
		t.Fatal("DebugWriter(none) should return nil writer")
	}
}

func TestDebugWriter_Default(t *testing.T) {
	w, cleanup, err := DebugWriter("")
	if err != nil {
		t.Fatalf("DebugWriter('') error: %v", err)
	}
	defer cleanup()
	if w != nil {
		t.Fatal("DebugWriter('') should return nil writer")
	}
}

func TestNewClient_OpenRouterProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openrouter"
	cfg.Provider.OpenRouter.APIKey = "sk-or-test"

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
}

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
		_, _ = w.Write([]byte(`{"error": {"message": "invalid api key"}}`))
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

func TestChat_APIErrorInBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chatResp := ChatResponse{
			Error: &APIError{Message: "rate limit exceeded"},
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

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for API error in body")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("error = %q, want it to mention rate limit", err.Error())
	}
}

func TestChat_InvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json at all"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.APIKey = "test-key"
	cfg.Provider.OpenAI.BaseURL = server.URL

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "parse response") {
		t.Errorf("error = %q, want parse response error", err.Error())
	}
}

func TestChat_InvalidLLMResponseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chatResp := ChatResponse{
			Choices: []Choice{{Message: Message{Content: "not valid json"}}},
		}
		_ = json.NewEncoder(w).Encode(chatResp)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "openai"
	cfg.Provider.OpenAI.APIKey = "test-key"
	cfg.Provider.OpenAI.BaseURL = server.URL

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for invalid LLM response JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse LLM response") {
		t.Errorf("error = %q, want LLM parse error", err.Error())
	}
}

// --- NewClient: local provider ---

func TestNewClient_LocalProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = "http://localhost:11434/api/generate"

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
}

// --- ollamaClient.Chat tests ---

func TestOllamaChat_Success(t *testing.T) {
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
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.System != "system prompt" {
			t.Errorf("system = %q, want %q", req.System, "system prompt")
		}
		if req.Prompt != "list files" {
			t.Errorf("prompt = %q, want %q", req.Prompt, "list files")
		}
		ollamaResp := ollamaResponse{Response: string(respContent), Done: true}
		if err := json.NewEncoder(w).Encode(ollamaResp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL

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

func TestOllamaChat_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer local-key" {
			t.Errorf("auth = %q, want %q", auth, "Bearer local-key")
		}
		ollamaResp := ollamaResponse{Response: `{"type":"commands","commands":[{"command":"pwd","description":"d","risk":"safe","certainty":99}]}`, Done: true}
		_ = json.NewEncoder(w).Encode(ollamaResp)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL
	cfg.Provider.Local.APIKey = "local-key"

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
}

func TestOllamaChat_NoAPIKey_NoAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("expected no auth header, got %q", auth)
		}
		ollamaResp := ollamaResponse{Response: `{"type":"commands","commands":[{"command":"pwd","description":"d","risk":"safe","certainty":99}]}`, Done: true}
		_ = json.NewEncoder(w).Encode(ollamaResp)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL
	cfg.Provider.Local.APIKey = ""

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
}

func TestOllamaChat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "local server error") {
		t.Errorf("error = %q, want local server error", err.Error())
	}
}

func TestOllamaChat_OllamaError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ollamaResp := ollamaResponse{Error: "model not found"}
		_ = json.NewEncoder(w).Encode(ollamaResp)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for ollama error response")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("error = %q, want model not found", err.Error())
	}
}

func TestOllamaChat_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse response") {
		t.Errorf("error = %q, want parse response error", err.Error())
	}
}

func TestOllamaChat_DebugOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ollamaResp := ollamaResponse{Response: `{"type":"commands","commands":[{"command":"pwd","description":"d","risk":"safe","certainty":99}]}`, Done: true}
		_ = json.NewEncoder(w).Encode(ollamaResp)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL

	var buf strings.Builder
	client, _ := NewClient(cfg, &buf)

	_, err := client.Chat("sys", "test")
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
}

func TestOllamaChat_InvalidLLMResponseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ollamaResp := ollamaResponse{Response: "not valid json", Done: true}
		_ = json.NewEncoder(w).Encode(ollamaResp)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Provider.Default = "local"
	cfg.Provider.Local.BaseURL = server.URL

	client, _ := NewClient(cfg, nil)
	_, err := client.Chat("sys", "test")
	if err == nil {
		t.Fatal("expected error for invalid LLM response JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse LLM response") {
		t.Errorf("error = %q, want LLM parse error", err.Error())
	}
}
