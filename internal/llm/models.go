package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type ModelInfo struct {
	ID      string
	Name    string
	Company string
}

type ModelGroup struct {
	Company string
	Models  []ModelInfo
}

type openRouterModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type openRouterResponse struct {
	Data []openRouterModel `json:"data"`
}

type openAIModelEntry struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
}

type openAIModelsResponse struct {
	Data []openAIModelEntry `json:"data"`
}

func FetchOpenRouterModels(baseURL, apiKey string) ([]ModelInfo, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", strings.TrimRight(baseURL, "/")+"/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create OpenRouter models request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenRouter models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter models API error (%d): %s", resp.StatusCode, string(body))
	}

	var or openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return nil, fmt.Errorf("decode OpenRouter models: %w", err)
	}

	const otherCompany = "Other"
	models := make([]ModelInfo, 0, len(or.Data))
	for _, m := range or.Data {
		company := otherCompany
		if idx := strings.Index(m.ID, "/"); idx >= 0 {
			prefix := m.ID[:idx]
			if prefix != "" {
				company = strings.ToUpper(prefix[:1]) + prefix[1:]
			}
		}
		models = append(models, ModelInfo{ID: m.ID, Name: m.Name, Company: company})
	}
	return models, nil
}

func FetchOpenAIModels(baseURL, apiKey string) ([]ModelInfo, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), "GET", strings.TrimRight(baseURL, "/")+"/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create OpenAI models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenAI models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI models API error (%d): %s", resp.StatusCode, string(body))
	}

	var ai openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&ai); err != nil {
		return nil, fmt.Errorf("decode OpenAI models: %w", err)
	}

	var gptModels []openAIModelEntry
	for _, m := range ai.Data {
		if strings.HasPrefix(m.ID, "gpt-") {
			gptModels = append(gptModels, m)
		}
	}
	sort.Slice(gptModels, func(i, j int) bool {
		return gptModels[i].Created > gptModels[j].Created
	})

	models := make([]ModelInfo, 0, len(gptModels))
	for _, m := range gptModels {
		models = append(models, ModelInfo{ID: m.ID, Name: m.ID, Company: "OpenAI"})
	}
	return models, nil
}

func GroupByCompany(models []ModelInfo) []ModelGroup {
	if len(models) == 0 {
		return nil
	}

	groupMap := make(map[string][]ModelInfo)
	for _, m := range models {
		groupMap[m.Company] = append(groupMap[m.Company], m)
	}

	groups := make([]ModelGroup, 0, len(groupMap))
	for company, ms := range groupMap {
		groups = append(groups, ModelGroup{Company: company, Models: ms})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Company < groups[j].Company
	})

	return groups
}
