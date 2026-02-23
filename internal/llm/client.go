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
}

// DebugWriter returns an io.Writer based on the debug mode string.
// "screen" -> stderr, "file" -> ~/.ai-cli/llm.log (append), anything else -> nil.
func DebugWriter(mode string) (io.Writer, func(), error) {
	switch mode {
	case "screen":
		return os.Stderr, func() {}, nil
	case "file":
		dir, err := config.ConfigDir()
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
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider.Default)
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key configured for provider %q. Run: ai config set %s_key YOUR_KEY", cfg.Provider.Default, cfg.Provider.Default)
	}

	return &openAIClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      cfg.Provider.Model,
		debugOut:   debugOut,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type openAIClient struct {
	baseURL    string
	apiKey     string
	model      string
	debugOut   io.Writer
	httpClient *http.Client
}

func (c *openAIClient) Chat(systemPrompt, userMessage string) (*Response, error) {
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

	if c.debugOut != nil {
		ts := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(c.debugOut, "\n[%s] --- RESPONSE (HTTP %d) ---\n%s\n--- END RESPONSE ---\n\n", ts, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, errors.New("no response from LLM")
	}

	content := chatResp.Choices[0].Message.Content
	return parseResponse(content)
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
