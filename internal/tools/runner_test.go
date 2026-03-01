package tools

import (
	"strings"
	"testing"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

// mockClient implements llm.Client for testing.
type mockClient struct {
	responses []*llm.Response
	errors    []error
	calls     int
	messages  [][]llm.Message // captured messages from each call
}

func (m *mockClient) Chat(systemPrompt, userMessage string) (*llm.Response, error) {
	return m.ChatMessages([]llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	})
}

func (m *mockClient) ChatMessages(messages []llm.Message) (*llm.Response, error) {
	idx := m.calls
	m.calls++
	m.messages = append(m.messages, messages)
	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return &llm.Response{Type: "commands", Commands: []llm.Command{{Command: "echo fallback", Description: "fallback", Risk: "safe", Certainty: 100}}}, nil
}

func toolCallingCfg(mode string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Safety.ToolCalling = mode
	return cfg
}

func TestRunWithTools_NeverMode_SkipsToolLoop(t *testing.T) {
	client := &mockClient{
		responses: []*llm.Response{
			{Type: "commands", Commands: []llm.Command{{Command: "ls", Description: "list", Risk: "safe", Certainty: 95}}},
		},
	}

	resp, err := RunWithTools(client, "system", "list files", toolCallingCfg(config.ToolCallingNever), shell.Info{}, 3)
	if err != nil {
		t.Fatalf("RunWithTools error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want commands", resp.Type)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want 1", client.calls)
	}
}

func TestRunWithTools_NeverMode_IgnoresToolRequest(t *testing.T) {
	// Even if the LLM returns a tool_request, "never" mode uses Chat() which
	// returns whatever the LLM says. The tool_request won't be acted on.
	client := &mockClient{
		responses: []*llm.Response{
			// Chat() is used, so this is the one response
			{Type: "tool_request", Tool: "disk_usage", ToolArgs: map[string]string{}},
		},
	}

	resp, err := RunWithTools(client, "system", "test", toolCallingCfg(config.ToolCallingNever), shell.Info{}, 3)
	if err != nil {
		t.Fatalf("RunWithTools error: %v", err)
	}
	// In never mode, the raw response is returned without entering the tool loop
	if resp.Type != "tool_request" {
		t.Errorf("type = %q, want tool_request (passed through without processing)", resp.Type)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want 1 (no loop)", client.calls)
	}
}

func TestRunWithTools_AlwaysAllowMode_NoPrompt(t *testing.T) {
	client := &mockClient{
		responses: []*llm.Response{
			{Type: "tool_request", Tool: "check_command", ToolArgs: map[string]string{"command": "ls"}},
			{Type: "commands", Commands: []llm.Command{{Command: "ls -la", Description: "list", Risk: "safe", Certainty: 95}}},
		},
	}

	resp, err := RunWithTools(client, "system", "list files", toolCallingCfg(config.ToolCallingAlwaysAllow), shell.Info{}, 3)
	if err != nil {
		t.Fatalf("RunWithTools error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want commands", resp.Type)
	}
	if client.calls != 2 {
		t.Errorf("calls = %d, want 2", client.calls)
	}

	// Verify tool result was injected
	lastMessages := client.messages[1]
	found := false
	for _, m := range lastMessages {
		if strings.Contains(m.Content, "Tool result for check_command") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool result in follow-up messages")
	}
}

func TestRunWithTools_AlwaysPromptMode_Approved(t *testing.T) {
	origConfirm := ConfirmFunc
	defer func() { ConfirmFunc = origConfirm }()
	ConfirmFunc = func(string, map[string]string, string) bool { return true }

	client := &mockClient{
		responses: []*llm.Response{
			{Type: "tool_request", Tool: "disk_usage", ToolArgs: map[string]string{}},
			{Type: "commands", Commands: []llm.Command{{Command: "df -h", Description: "disk", Risk: "safe", Certainty: 90}}},
		},
	}

	resp, err := RunWithTools(client, "system", "show disk", toolCallingCfg(config.ToolCallingAlwaysPrompt), shell.Info{}, 3)
	if err != nil {
		t.Fatalf("RunWithTools error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want commands", resp.Type)
	}
	if client.calls != 2 {
		t.Errorf("calls = %d, want 2", client.calls)
	}
}

func TestRunWithTools_AlwaysPromptMode_Denied(t *testing.T) {
	origConfirm := ConfirmFunc
	defer func() { ConfirmFunc = origConfirm }()
	ConfirmFunc = func(string, map[string]string, string) bool { return false }

	client := &mockClient{
		responses: []*llm.Response{
			{Type: "tool_request", Tool: "disk_usage", ToolArgs: map[string]string{}},
			{Type: "commands", Commands: []llm.Command{{Command: "df -h", Description: "disk", Risk: "safe", Certainty: 90}}},
		},
	}

	resp, err := RunWithTools(client, "system", "show disk", toolCallingCfg(config.ToolCallingAlwaysPrompt), shell.Info{}, 3)
	if err != nil {
		t.Fatalf("RunWithTools error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want commands", resp.Type)
	}

	// Check that denial message was sent
	lastMessages := client.messages[1]
	found := false
	for _, m := range lastMessages {
		if strings.Contains(m.Content, "Tool request denied") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected denial message in follow-up")
	}
}

func TestRunWithTools_DangerousPromptMode_SafeToolNoPrompt(t *testing.T) {
	// In dangerous_prompt mode, a safe tool (no safety issue) should NOT prompt
	promptCalled := false
	origConfirm := ConfirmFunc
	defer func() { ConfirmFunc = origConfirm }()
	ConfirmFunc = func(string, map[string]string, string) bool {
		promptCalled = true
		return true
	}

	client := &mockClient{
		responses: []*llm.Response{
			{Type: "tool_request", Tool: "disk_usage", ToolArgs: map[string]string{}},
			{Type: "commands", Commands: []llm.Command{{Command: "df -h", Description: "disk", Risk: "safe", Certainty: 90}}},
		},
	}

	resp, err := RunWithTools(client, "system", "show disk", toolCallingCfg(config.ToolCallingDangerousPrompt), shell.Info{}, 3)
	if err != nil {
		t.Fatalf("RunWithTools error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want commands", resp.Type)
	}
	if promptCalled {
		t.Error("should not have prompted for a safe tool in dangerous_prompt mode")
	}
}

func TestRunWithTools_UnknownTool(t *testing.T) {
	client := &mockClient{
		responses: []*llm.Response{
			{Type: "tool_request", Tool: "nonexistent_tool", ToolArgs: map[string]string{}},
		},
	}

	_, err := RunWithTools(client, "system", "test", toolCallingCfg(config.ToolCallingAlwaysAllow), shell.Info{}, 3)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("error = %q, want 'unknown tool'", err.Error())
	}
}

func TestRunWithTools_MaxIterations(t *testing.T) {
	client := &mockClient{
		responses: []*llm.Response{
			{Type: "tool_request", Tool: "disk_usage", ToolArgs: map[string]string{}},
			{Type: "tool_request", Tool: "disk_usage", ToolArgs: map[string]string{}},
			{Type: "tool_request", Tool: "disk_usage", ToolArgs: map[string]string{}},
			// Response to "max iterations" follow-up
			{Type: "commands", Commands: []llm.Command{{Command: "df -h", Description: "disk usage", Risk: "safe", Certainty: 90}}},
		},
	}

	resp, err := RunWithTools(client, "system", "show disk", toolCallingCfg(config.ToolCallingAlwaysAllow), shell.Info{}, 3)
	if err != nil {
		t.Fatalf("RunWithTools error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want commands", resp.Type)
	}
	// 3 tool iterations + 1 final call = 4
	if client.calls != 4 {
		t.Errorf("calls = %d, want 4", client.calls)
	}
}

func TestRunWithTools_ClientError(t *testing.T) {
	client := &mockClient{
		responses: []*llm.Response{nil},
		errors:    []error{errTest},
	}

	_, err := RunWithTools(client, "system", "test", toolCallingCfg(config.ToolCallingAlwaysAllow), shell.Info{}, 3)
	if err == nil {
		t.Fatal("expected error")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }
