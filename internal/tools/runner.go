package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

// ConfirmFunc prompts the user for tool confirmation. Replaceable in tests.
var ConfirmFunc = defaultConfirm

func defaultConfirm(toolName string, args map[string]string, reason string) bool {
	argsStr := ""
	if len(args) > 0 {
		parts := make([]string, 0, len(args))
		for k, v := range args {
			parts = append(parts, k+"="+v)
		}
		argsStr = " (" + strings.Join(parts, ", ") + ")"
	}
	if reason != "" {
		color.Yellow("Warning: %s", reason)
	}
	fmt.Printf("AI wants to use tool %q%s. Allow? [Y/n] ", toolName, argsStr)
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

// RunWithTools calls the LLM and handles tool_request responses in a loop
// (up to maxIter iterations). Returns the final non-tool response.
// If tool_calling is "never", it calls the LLM once with Chat() (no tool loop).
func RunWithTools(client llm.Client, systemPrompt, userMessage string, cfg *config.Config, shellInfo shell.Info, maxIter int) (*llm.Response, error) {
	// If tools are disabled, just do a simple chat call
	if cfg.Safety.ToolCalling == config.ToolCallingNever {
		return client.Chat(systemPrompt, userMessage)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	for i := 0; i < maxIter; i++ {
		resp, err := client.ChatMessages(messages)
		if err != nil {
			return nil, err
		}

		if resp.Type != "tool_request" {
			return resp, nil
		}

		// Validate tool exists
		if !ValidTool(resp.Tool) {
			return nil, fmt.Errorf("AI requested unknown tool: %s", resp.Tool)
		}

		// Check if this tool call would trigger a safety issue (for dangerous_prompt mode)
		safetyIssue := checkToolSafety(resp.Tool, resp.ToolArgs)

		// Determine whether to prompt based on mode
		switch cfg.Safety.ToolCalling {
		case config.ToolCallingAlwaysPrompt:
			if !ConfirmFunc(resp.Tool, resp.ToolArgs, "") {
				messages = appendDenial(messages, resp)
				continue
			}
		case config.ToolCallingDangerousPrompt:
			if safetyIssue != "" {
				if !ConfirmFunc(resp.Tool, resp.ToolArgs, safetyIssue) {
					messages = appendDenial(messages, resp)
					continue
				}
			}
		case config.ToolCallingAlwaysAllow:
			// No prompting
		}

		// Execute tool
		color.Cyan("Using tool: %s", resp.Tool)
		output, err := Execute(resp.Tool, resp.ToolArgs, shellInfo)
		if err != nil {
			output = fmt.Sprintf("Tool error: %s", err.Error())
		}

		// Build follow-up messages
		toolReqJSON, _ := json.Marshal(resp)
		messages = append(messages,
			llm.Message{Role: "assistant", Content: string(toolReqJSON)},
			llm.Message{Role: "user", Content: fmt.Sprintf("Tool result for %s:\n%s", resp.Tool, output)},
		)
	}

	// Max iterations reached — make one final call
	messages = append(messages,
		llm.Message{Role: "user", Content: "You have used the maximum number of tools. Please provide your final response now."},
	)
	return client.ChatMessages(messages)
}

// appendDenial adds a denial message to the conversation for the LLM to continue without the tool.
func appendDenial(messages []llm.Message, resp *llm.Response) []llm.Message {
	color.Yellow("Tool request denied. Asking AI to proceed without it.")
	toolReqJSON, _ := json.Marshal(resp)
	return append(messages,
		llm.Message{Role: "assistant", Content: string(toolReqJSON)},
		llm.Message{Role: "user", Content: "Tool request denied by user. Please generate your best response without using tools."},
	)
}

// checkToolSafety checks if a tool call would trigger a safety concern.
// Returns a description of the issue, or "" if the call is safe.
func checkToolSafety(toolName string, args map[string]string) string {
	switch toolName {
	case "read_file", "list_directory":
		path := args["path"]
		if path == "" || path == "." {
			return ""
		}
		cwd, err := os.Getwd()
		if err != nil {
			return ""
		}
		_, err = ValidatePath(path, cwd)
		if err != nil {
			return fmt.Sprintf("path %q triggers safety rule: %s", path, err.Error())
		}
	}
	return ""
}
