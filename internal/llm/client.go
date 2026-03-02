package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kriserickson/ai-cli/internal/config"
)

type Client interface {
	Chat(systemPrompt, userMessage string) (*Response, error)
	ChatWithTrace(systemPrompt, userMessage string) (*ChatResult, error)
}

// DebugWriter returns an io.Writer based on the debug mode string.
// "screen" -> stderr, "file" -> ~/.ai-cli/llm.log (append), anything else -> nil.
func DebugWriter(mode string) (io.Writer, func(), error) {
	switch mode {
	case "screen":
		return os.Stderr, func() {}, nil
	case "file":
		dir, err := config.Dir()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot determine config dir for log: %w", err)
		}
		logPath := filepath.Join(dir, "llm.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot open log file %s: %w", logPath, err)
		}
		return f, func() { f.Close() }, nil
	default:
		return nil, func() {}, nil
	}
}

func NewClient(cfg *config.Config, debugOut io.Writer) (Client, error) {
	var baseURL, apiKey string

	switch cfg.Provider.Default {
	case config.ProviderOpenAI:
		baseURL = cfg.Provider.OpenAI.BaseURL
		apiKey = cfg.Provider.OpenAI.APIKey
	case config.ProviderOpenRouter:
		baseURL = cfg.Provider.OpenRouter.BaseURL
		apiKey = cfg.Provider.OpenRouter.APIKey
	case config.ProviderLocal:
		return &ollamaClient{
			provider:   cfg.Provider.Default,
			baseURL:    strings.TrimRight(cfg.Provider.Local.BaseURL, "/"),
			apiKey:     cfg.Provider.Local.APIKey,
			model:      cfg.Provider.Model,
			debugOut:   debugOut,
			httpClient: &http.Client{Timeout: 120 * time.Second},
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider.Default)
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key configured for provider %q. Run: ai config set %s_key YOUR_KEY", cfg.Provider.Default, cfg.Provider.Default)
	}

	return &openAIClient{
		provider:   cfg.Provider.Default,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      cfg.Provider.Model,
		debugOut:   debugOut,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type openAIClient struct {
	provider   string
	baseURL    string
	apiKey     string
	model      string
	debugOut   io.Writer
	httpClient *http.Client
}

func (c *openAIClient) Chat(systemPrompt, userMessage string) (*Response, error) {
	result, err := c.ChatWithTrace(systemPrompt, userMessage)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (c *openAIClient) ChatWithTrace(systemPrompt, userMessage string) (*ChatResult, error) {
	req := ChatRequest{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	trace := ChatTrace{
		Provider:    c.provider,
		Model:       c.model,
		Endpoint:    c.baseURL + "/chat/completions",
		RequestBody: string(body),
	}

	if c.debugOut != nil {
		prettyReq, _ := json.MarshalIndent(req, "", "  ")
		ts := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(c.debugOut, "\n[%s] --- REQUEST ---\nPOST %s/chat/completions\n%s\n--- END REQUEST ---\n\n", ts, c.baseURL, string(prettyReq))
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	trace.ResponseBody = string(respBody)
	trace.HTTPStatus = resp.StatusCode

	if c.debugOut != nil {
		ts := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(c.debugOut, "\n[%s] --- RESPONSE (HTTP %d) ---\n%s\n--- END RESPONSE ---\n\n", ts, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if resp.StatusCode != http.StatusOK {
		return &ChatResult{Trace: trace}, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return &ChatResult{Trace: trace}, errors.New("no response from LLM")
	}

	content := chatResp.Choices[0].Message.Content
	trace.RawContent = content
	parsed, err := parseResponse(content)
	if err != nil {
		return &ChatResult{Trace: trace}, err
	}
	return &ChatResult{Response: parsed, Trace: trace}, nil
}

// ollamaClient implements the Client interface for Ollama-compatible local servers.
type ollamaClient struct {
	provider   string
	baseURL    string
	apiKey     string
	model      string
	debugOut   io.Writer
	httpClient *http.Client
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

func (c *ollamaClient) Chat(systemPrompt, userMessage string) (*Response, error) {
	result, err := c.ChatWithTrace(systemPrompt, userMessage)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (c *ollamaClient) ChatWithTrace(systemPrompt, userMessage string) (*ChatResult, error) {
	req := ollamaRequest{
		Model:  c.model,
		Prompt: userMessage,
		System: systemPrompt,
		Stream: false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	trace := ChatTrace{
		Provider:    c.provider,
		Model:       c.model,
		Endpoint:    c.baseURL,
		RequestBody: string(body),
	}

	if c.debugOut != nil {
		prettyReq, _ := json.MarshalIndent(req, "", "  ")
		ts := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(c.debugOut, "\n[%s] --- REQUEST ---\nPOST %s\n%s\n--- END REQUEST ---\n\n", ts, c.baseURL, string(prettyReq))
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("local server request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	trace.ResponseBody = string(respBody)
	trace.HTTPStatus = resp.StatusCode

	if c.debugOut != nil {
		ts := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(c.debugOut, "\n[%s] --- RESPONSE (HTTP %d) ---\n%s\n--- END RESPONSE ---\n\n", ts, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if resp.StatusCode != http.StatusOK {
		return &ChatResult{Trace: trace}, fmt.Errorf("local server error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if ollamaResp.Error != "" {
		return &ChatResult{Trace: trace}, fmt.Errorf("local server error: %s", ollamaResp.Error)
	}

	trace.RawContent = ollamaResp.Response
	parsed, err := parseResponse(ollamaResp.Response)
	if err != nil {
		return &ChatResult{Trace: trace}, err
	}
	return &ChatResult{Response: parsed, Trace: trace}, nil
}

func parseResponse(content string) (*Response, error) {
	content = strings.TrimSpace(content)

	// Strip markdown code fences if present
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		// Remove first line (```json or ```)
		if len(lines) > 2 {
			lines = lines[1:]
		}
		// Remove last line (```)
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		content = strings.Join(lines, "\n")
	}

	var resp Response
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w\nRaw response: %s", err, content)
	}

	return &resp, nil
}
