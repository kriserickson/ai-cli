package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/kriserickson/ai-cli/internal/config"
)

// CopilotToken returns the GitHub token used to call the Copilot API.
// An explicitly configured key wins; otherwise the token of the GitHub CLI
// login is used, so `gh auth login` is all the setup Copilot needs.
func CopilotToken(cfg *config.Config) (string, error) {
	if cfg.Provider.Copilot.APIKey != "" {
		return cfg.Provider.Copilot.APIKey, nil
	}
	out, err := exec.CommandContext(context.Background(), "gh", "auth", "token").Output()
	token := strings.TrimSpace(string(out))
	if err != nil || token == "" {
		return "", errors.New("no GitHub login found for copilot. Run: gh auth login")
	}
	return token, nil
}

type copilotModelsResponse struct {
	Data []struct {
		ID                 string `json:"id"`
		Name               string `json:"name"`
		Vendor             string `json:"vendor"`
		ModelPickerEnabled bool   `json:"model_picker_enabled"`
	} `json:"data"`
}

// FetchCopilotModels lists the chat models available to the Copilot account.
func FetchCopilotModels(baseURL, token string) ([]ModelInfo, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", strings.TrimRight(baseURL, "/")+"/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create Copilot models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch Copilot models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Copilot models API error (%d): %s", resp.StatusCode, string(body))
	}

	var cr copilotModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode Copilot models: %w", err)
	}

	models := make([]ModelInfo, 0, len(cr.Data))
	for _, m := range cr.Data {
		if m.ModelPickerEnabled {
			models = append(models, ModelInfo{ID: m.ID, Name: m.Name, Company: m.Vendor})
		}
	}
	return models, nil
}
