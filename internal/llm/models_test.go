package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchOpenRouterModels_Success(t *testing.T) {
	data := map[string]interface{}{
		"data": []map[string]interface{}{
			{"id": "anthropic/claude-3.5-sonnet", "name": "Claude 3.5 Sonnet"},
			{"id": "openai/gpt-4o", "name": "GPT-4o"},
			{"id": "noslash", "name": "No Slash Model"},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %s, want /models", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(data); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	models, err := FetchOpenRouterModels(server.URL, "")
	if err != nil {
		t.Fatalf("FetchOpenRouterModels error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("len(models) = %d, want 3", len(models))
	}
	if models[0].Company != "Anthropic" {
		t.Errorf("models[0].Company = %q, want %q", models[0].Company, "Anthropic")
	}
	if models[1].Company != "Openai" {
		t.Errorf("models[1].Company = %q, want %q", models[1].Company, "Openai")
	}
	if models[2].Company != "Other" {
		t.Errorf("models[2].Company = %q, want %q", models[2].Company, "Other")
	}
}

func TestFetchOpenRouterModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	_, err := FetchOpenRouterModels(server.URL, "")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to mention 500", err.Error())
	}
}

func TestFetchOpenAIModels_Success(t *testing.T) {
	data := map[string]interface{}{
		"data": []map[string]interface{}{
			{"id": "gpt-4o", "created": 1700000000},
			{"id": "gpt-3.5-turbo", "created": 1600000000},
			{"id": "gpt-4-turbo", "created": 1750000000},
			{"id": "dall-e-3", "created": 1680000000},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %s, want /models", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth = %q, want %q", r.Header.Get("Authorization"), "Bearer test-key")
		}
		if err := json.NewEncoder(w).Encode(data); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	models, err := FetchOpenAIModels(server.URL, "test-key")
	if err != nil {
		t.Fatalf("FetchOpenAIModels error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("len(models) = %d, want 3 (dall-e-3 filtered)", len(models))
	}
	// sorted by created descending
	if models[0].ID != "gpt-4-turbo" {
		t.Errorf("models[0].ID = %q, want gpt-4-turbo (newest first)", models[0].ID)
	}
	if models[1].ID != "gpt-4o" {
		t.Errorf("models[1].ID = %q, want gpt-4o", models[1].ID)
	}
	if models[2].ID != "gpt-3.5-turbo" {
		t.Errorf("models[2].ID = %q, want gpt-3.5-turbo", models[2].ID)
	}
	for _, m := range models {
		if m.Company != "OpenAI" {
			t.Errorf("m.Company = %q, want OpenAI", m.Company)
		}
	}
}

func TestFetchOpenAIModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	_, err := FetchOpenAIModels(server.URL, "bad-key")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want it to mention 401", err.Error())
	}
}

func TestFetchOpenRouterModels_WithAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-or-test" {
			t.Errorf("auth = %q, want %q", auth, "Bearer sk-or-test")
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "anthropic/claude-3.5-sonnet", "name": "Claude"},
			},
		})
	}))
	defer server.Close()

	models, err := FetchOpenRouterModels(server.URL, "sk-or-test")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
}

func TestFetchOpenRouterModels_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := FetchOpenRouterModels(server.URL, "")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode OpenRouter models") {
		t.Errorf("error = %q, want decode error", err.Error())
	}
}

func TestFetchOpenRouterModels_EmptyPrefixModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "/some-model", "name": "Empty Prefix"},
			},
		})
	}))
	defer server.Close()

	models, err := FetchOpenRouterModels(server.URL, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	// Empty prefix should stay as "Other" since prefix is empty
	if models[0].Company != "Other" {
		t.Errorf("company = %q, want %q", models[0].Company, "Other")
	}
}

func TestFetchOpenAIModels_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := FetchOpenAIModels(server.URL, "test-key")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode OpenAI models") {
		t.Errorf("error = %q, want decode error", err.Error())
	}
}

func TestFetchOpenAIModels_NoGPTModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "dall-e-3", "created": 1},
				{"id": "text-embedding-3-small", "created": 2},
			},
		})
	}))
	defer server.Close()

	models, err := FetchOpenAIModels(server.URL, "test-key")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("len(models) = %d, want 0 (no gpt- models)", len(models))
	}
}

// --- FetchLocalModels ---

func TestFetchLocalModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("path = %s, want /api/tags", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []map[string]interface{}{
				{"name": "llama3:latest"},
				{"name": "codellama:7b"},
			},
		})
	}))
	defer server.Close()

	models, err := FetchLocalModels(server.URL + "/api/generate")
	if err != nil {
		t.Fatalf("FetchLocalModels error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
	if models[0].ID != "llama3:latest" {
		t.Errorf("models[0].ID = %q, want %q", models[0].ID, "llama3:latest")
	}
	if models[0].Company != "Local" {
		t.Errorf("models[0].Company = %q, want %q", models[0].Company, "Local")
	}
	if models[1].ID != "codellama:7b" {
		t.Errorf("models[1].ID = %q, want %q", models[1].ID, "codellama:7b")
	}
}

func TestFetchLocalModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	_, err := FetchLocalModels(server.URL + "/api/generate")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "local models API error") {
		t.Errorf("error = %q, want local models API error", err.Error())
	}
}

func TestFetchLocalModels_DecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, err := FetchLocalModels(server.URL + "/api/generate")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode local models") {
		t.Errorf("error = %q, want decode error", err.Error())
	}
}

func TestFetchLocalModels_EmptyModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []map[string]interface{}{},
		})
	}))
	defer server.Close()

	models, err := FetchLocalModels(server.URL + "/api/generate")
	if err != nil {
		t.Fatalf("FetchLocalModels error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("len(models) = %d, want 0", len(models))
	}
}

func TestFetchLocalModels_NonStandardBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags" {
			t.Errorf("path = %s, want /tags", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []map[string]interface{}{
				{"name": "test-model"},
			},
		})
	}))
	defer server.Close()

	// Use a non-standard URL that doesn't end in /api/generate
	models, err := FetchLocalModels(server.URL + "/custom")
	if err != nil {
		t.Fatalf("FetchLocalModels error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
}

func TestGroupByCompany(t *testing.T) {
	models := []ModelInfo{
		{ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Company: "Anthropic"},
		{ID: "anthropic/claude-3-opus", Name: "Claude 3 Opus", Company: "Anthropic"},
		{ID: "openai/gpt-4o", Name: "GPT-4o", Company: "OpenAI"},
	}

	groups := GroupByCompany(models)
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	// alphabetical: Anthropic before OpenAI
	if groups[0].Company != "Anthropic" {
		t.Errorf("groups[0].Company = %q, want Anthropic", groups[0].Company)
	}
	if groups[1].Company != "OpenAI" {
		t.Errorf("groups[1].Company = %q, want OpenAI", groups[1].Company)
	}
	// model order preserved within group
	if groups[0].Models[0].ID != "anthropic/claude-3.5-sonnet" {
		t.Errorf("first Anthropic model = %q, want anthropic/claude-3.5-sonnet", groups[0].Models[0].ID)
	}
	if groups[0].Models[1].ID != "anthropic/claude-3-opus" {
		t.Errorf("second Anthropic model = %q, want anthropic/claude-3-opus", groups[0].Models[1].ID)
	}
}

func TestGroupByCompany_Empty(t *testing.T) {
	groups := GroupByCompany(nil)
	if groups != nil {
		t.Errorf("GroupByCompany(nil) = %v, want nil", groups)
	}
}
