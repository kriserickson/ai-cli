package llm

import "testing"

func TestParseResponse_PlainJSON(t *testing.T) {
	input := `{"type":"commands","commands":[{"command":"ls -la","description":"list files","risk":"safe","certainty":95}]}`
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want %q", resp.Type, "commands")
	}
	if len(resp.Commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(resp.Commands))
	}
	if resp.Commands[0].Command != "ls -la" {
		t.Errorf("command = %q, want %q", resp.Commands[0].Command, "ls -la")
	}
	if resp.Commands[0].Certainty != 95 {
		t.Errorf("certainty = %d, want 95", resp.Commands[0].Certainty)
	}
}

func TestParseResponse_MarkdownFencedJSON(t *testing.T) {
	input := "```json\n{\"type\":\"commands\",\"commands\":[{\"command\":\"pwd\",\"description\":\"print dir\",\"risk\":\"safe\",\"certainty\":99}]}\n```"
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Type != "commands" {
		t.Errorf("type = %q, want %q", resp.Type, "commands")
	}
	if resp.Commands[0].Command != "pwd" {
		t.Errorf("command = %q, want %q", resp.Commands[0].Command, "pwd")
	}
}

func TestParseResponse_MarkdownFenceNoLanguage(t *testing.T) {
	input := "```\n{\"type\":\"commands\",\"commands\":[{\"command\":\"date\",\"description\":\"show date\",\"risk\":\"safe\",\"certainty\":90}]}\n```"
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Commands[0].Command != "date" {
		t.Errorf("command = %q, want %q", resp.Commands[0].Command, "date")
	}
}

func TestParseResponse_WithWhitespace(t *testing.T) {
	input := "  \n{\"type\":\"commands\",\"commands\":[{\"command\":\"whoami\",\"description\":\"current user\",\"risk\":\"safe\",\"certainty\":99}]}\n  "
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Commands[0].Command != "whoami" {
		t.Errorf("command = %q, want %q", resp.Commands[0].Command, "whoami")
	}
}

func TestParseResponse_ConfigType(t *testing.T) {
	input := `{"type":"config","action":"set_model","key":"model","value":"gpt-4o"}`
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Type != "config" {
		t.Errorf("type = %q, want %q", resp.Type, "config")
	}
	if resp.Action != "set_model" {
		t.Errorf("action = %q, want %q", resp.Action, "set_model")
	}
	if resp.Value != "gpt-4o" {
		t.Errorf("value = %q, want %q", resp.Value, "gpt-4o")
	}
}

func TestParseResponse_MultipleCommands(t *testing.T) {
	input := `{
		"type": "commands",
		"commands": [
			{"command": "lsof -ti :8080", "description": "find pid", "risk": "safe", "certainty": 95},
			{"command": "kill -9 $(lsof -ti :8080)", "description": "kill it", "risk": "risky", "certainty": 90}
		]
	}`
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(resp.Commands))
	}
	if resp.Commands[0].Risk != "safe" {
		t.Errorf("first command risk = %q, want %q", resp.Commands[0].Risk, "safe")
	}
	if resp.Commands[1].Risk != "risky" {
		t.Errorf("second command risk = %q, want %q", resp.Commands[1].Risk, "risky")
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	input := "this is not json at all"
	_, err := parseResponse(input)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseResponse_EmptyString(t *testing.T) {
	_, err := parseResponse("")
	if err == nil {
		t.Error("expected error for empty string, got nil")
	}
}

func TestParseResponse_WithExplanation(t *testing.T) {
	input := `{"type":"commands","commands":[{"command":"ls -la","description":"list files","risk":"safe","certainty":95,"explanation":"Lists all files including hidden ones in long format. -l enables long listing, -a includes dotfiles."}]}`
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Commands[0].Explanation != "Lists all files including hidden ones in long format. -l enables long listing, -a includes dotfiles." {
		t.Errorf("explanation = %q, want detailed explanation", resp.Commands[0].Explanation)
	}
}

func TestParseResponse_WithoutExplanation(t *testing.T) {
	input := `{"type":"commands","commands":[{"command":"ls","description":"list files","risk":"safe","certainty":99}]}`
	resp, err := parseResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Commands[0].Explanation != "" {
		t.Errorf("explanation = %q, want empty string", resp.Commands[0].Explanation)
	}
}
