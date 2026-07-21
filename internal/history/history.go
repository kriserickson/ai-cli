package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kriserickson/ai-cli/internal/config"
	"github.com/kriserickson/ai-cli/internal/executor"
	"github.com/kriserickson/ai-cli/internal/llm"
	"github.com/kriserickson/ai-cli/internal/shell"
)

const (
	maxLogBytes = 1 << 20
	maxBackups  = 5
)

type Session struct {
	ID               string           `json:"id"`
	Instruction      string           `json:"instruction"`
	Provider         string           `json:"provider"`
	Model            string           `json:"model"`
	OS               string           `json:"os"`
	Shell            string           `json:"shell"`
	ShellVersion     string           `json:"shell_version"`
	WorkingDirectory string           `json:"working_directory,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	RetryCount       int              `json:"retry_count"`
	Status           string           `json:"status"`
	Exchanges        []Exchange       `json:"exchanges,omitempty"`
	Executions       []CommandAttempt `json:"executions,omitempty"`
}

type Exchange struct {
	Attempt      int            `json:"attempt"`
	Kind         string         `json:"kind"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	UserMessage  string         `json:"user_message"`
	Response     *llm.Response  `json:"response,omitempty"`
	Trace        *llm.ChatTrace `json:"trace,omitempty"`
	Error        string         `json:"error,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type CommandAttempt struct {
	Attempt     int       `json:"attempt"`
	Index       int       `json:"index"`
	Command     string    `json:"command"`
	Description string    `json:"description"`
	Risk        string    `json:"risk"`
	Certainty   int       `json:"certainty"`
	Confirmed   bool      `json:"confirmed"`
	Skipped     bool      `json:"skipped"`
	ExitCode    int       `json:"exit_code"`
	Stdout      string    `json:"stdout,omitempty"`
	Stderr      string    `json:"stderr,omitempty"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	DurationMS  int64     `json:"duration_ms"`
}

func NewSession(instruction, cwd string, cfg *config.Config, shellInfo shell.Info) *Session {
	now := time.Now().UTC()
	provider := cfg.Provider.Default
	model := cfg.Provider.Model
	if selection, err := cfg.ActiveSelection(); err == nil {
		provider = selection.Provider
		model = selection.Model
	}
	return &Session{
		ID:               strconv.FormatInt(now.UnixNano(), 10),
		Instruction:      instruction,
		Provider:         provider,
		Model:            model,
		OS:               shellInfo.OS,
		Shell:            shellInfo.Shell,
		ShellVersion:     shellInfo.Version,
		WorkingDirectory: cwd,
		CreatedAt:        now,
		UpdatedAt:        now,
		Status:           "pending",
	}
}

func (s *Session) RecordExchange(kind string, attempt int, systemPrompt, userMessage string, result *llm.ChatResult, err error, cfg config.HistoryConfig) {
	exchange := Exchange{
		Attempt:     attempt,
		Kind:        kind,
		UserMessage: userMessage,
		CreatedAt:   time.Now().UTC(),
	}
	if cfg.IncludeLLMOutput {
		exchange.SystemPrompt = systemPrompt
	}
	if result != nil {
		if cfg.IncludeLLMOutput {
			exchange.Response = result.Response
		}
		if cfg.IncludeDebug {
			trace := result.Trace
			exchange.Trace = &trace
		}
	}
	if err != nil {
		exchange.Error = err.Error()
		s.Status = "failed"
	}
	s.Exchanges = append(s.Exchanges, exchange)
	s.touch()
}

func (s *Session) RecordExecutions(attempt int, results []executor.CommandResult) {
	for i := range results {
		result := &results[i]
		s.Executions = append(s.Executions, CommandAttempt{
			Attempt:     attempt,
			Index:       result.Index,
			Command:     result.Command.Command,
			Description: result.Command.Description,
			Risk:        result.Command.Risk,
			Certainty:   result.Command.Certainty,
			Confirmed:   result.Confirmed,
			Skipped:     result.Skipped,
			ExitCode:    result.ExitCode,
			Stdout:      result.Stdout,
			Stderr:      result.Stderr,
			Error:       result.Error,
			StartedAt:   result.StartedAt.UTC(),
			DurationMS:  result.Duration.Milliseconds(),
		})
	}
	s.touch()
}

func (s *Session) LastFailedExecution() *CommandAttempt {
	for i := len(s.Executions) - 1; i >= 0; i-- {
		if s.Executions[i].ExitCode != 0 && !s.Executions[i].Skipped {
			return &s.Executions[i]
		}
	}
	return nil
}

func (s *Session) MarkStatus(status string) {
	s.Status = status
	s.touch()
}

func Save(session *Session) error {
	path, err := logPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := rotateIfNeeded(path, int64(len(data))); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Chmod(0o600)
}

func Load(id string) (*Session, error) {
	sessions, err := latestSessions()
	if err != nil {
		return nil, err
	}

	matches := make([]Session, 0, 1)
	for i := range sessions {
		session := sessions[i]
		if session.ID == id || strings.HasPrefix(session.ID, id) {
			matches = append(matches, session)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("history session %q not found", id)
	case 1:
		return &matches[0], nil
	default:
		return nil, errors.New("history session id is ambiguous")
	}
}

func List() ([]Session, error) {
	sessions, err := latestSessions()
	if err != nil {
		return nil, err
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

func Remove(id string) error {
	sessions, err := latestSessions()
	if err != nil {
		return err
	}

	kept := make([]Session, 0, len(sessions))
	removed := false
	for i := range sessions {
		session := sessions[i]
		if session.ID == id || strings.HasPrefix(session.ID, id) {
			if removed {
				return errors.New("history session id is ambiguous")
			}
			removed = true
			continue
		}
		kept = append(kept, session)
	}
	if !removed {
		return fmt.Errorf("history session %q not found", id)
	}
	return rewriteLogs(kept)
}

func Clear() error {
	paths, err := logPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func BuildRetryMessage(session *Session, depth int) string {
	if depth <= 0 {
		depth = 1
	}
	start := len(session.Executions) - depth
	if start < 0 {
		start = 0
	}

	var b strings.Builder
	b.WriteString("The previous AI-generated command sequence failed. Analyze what happened, explain the fix briefly, and return corrected JSON commands.\n\n")
	b.WriteString("Original user request:\n")
	b.WriteString(session.Instruction)
	b.WriteString("\n\n")

	if len(session.Exchanges) > 0 {
		lastExchange := session.Exchanges[len(session.Exchanges)-1]
		b.WriteString("Most recent AI response:\n")
		if lastExchange.Response != nil {
			data, _ := json.MarshalIndent(lastExchange.Response, "", "  ")
			b.Write(data)
		} else if lastExchange.Error != "" {
			b.WriteString(lastExchange.Error)
		}
		b.WriteString("\n\n")
	}

	b.WriteString("Recent command execution history:\n")
	for i := start; i < len(session.Executions); i++ {
		execution := &session.Executions[i]
		fmt.Fprintf(&b, "- Step %d: %s\n", execution.Index+1, execution.Command)
		if execution.Description != "" {
			fmt.Fprintf(&b, "  Description: %s\n", execution.Description)
		}
		fmt.Fprintf(&b, "  Exit code: %d\n", execution.ExitCode)
		if execution.Stdout != "" {
			fmt.Fprintf(&b, "  Stdout:\n%s\n", indentBlock(execution.Stdout, "    "))
		}
		if execution.Stderr != "" {
			fmt.Fprintf(&b, "  Stderr:\n%s\n", indentBlock(execution.Stderr, "    "))
		}
		if execution.Error != "" {
			fmt.Fprintf(&b, "  Error: %s\n", execution.Error)
		}
	}

	return strings.TrimSpace(b.String())
}

func logPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.log"), nil
}

func logPaths() ([]string, error) {
	base, err := logPath()
	if err != nil {
		return nil, err
	}
	paths := []string{base}
	for i := 1; i <= maxBackups; i++ {
		paths = append(paths, fmt.Sprintf("%s.%d", base, i))
	}
	return paths, nil
}

func rotateIfNeeded(path string, incomingBytes int64) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size()+incomingBytes <= maxLogBytes {
		return nil
	}

	for i := maxBackups; i >= 1; i-- {
		src := path
		if i > 1 {
			src = fmt.Sprintf("%s.%d", path, i-1)
		}
		dst := fmt.Sprintf("%s.%d", path, i)
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func latestSessions() ([]Session, error) {
	paths, err := logPaths()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]Session)
	for _, path := range paths {
		sessions, err := readLog(path)
		if err != nil {
			return nil, err
		}
		for i := range sessions {
			session := sessions[i]
			current, ok := seen[session.ID]
			if !ok || session.UpdatedAt.After(current.UpdatedAt) {
				seen[session.ID] = session
			}
		}
	}

	out := make([]Session, 0, len(seen))
	for id := range seen {
		session := seen[id]
		out = append(out, session)
	}
	return out, nil
}

func readLog(path string) ([]Session, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var sessions []Session
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = []byte(strings.TrimSpace(string(line)))
			if len(line) > 0 {
				var session Session
				if unmarshalErr := json.Unmarshal(line, &session); unmarshalErr != nil {
					return nil, unmarshalErr
				}
				sessions = append(sessions, session)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

func rewriteLogs(sessions []Session) error {
	base, err := logPath()
	if err != nil {
		return err
	}
	if err := Clear(); err != nil {
		return err
	}
	if len(sessions) == 0 {
		return nil
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.Before(sessions[j].UpdatedAt)
	})
	for i := range sessions {
		if err := Save(&sessions[i]); err != nil {
			return err
		}
	}

	_, err = os.Stat(base)
	if err != nil {
		return err
	}
	return nil
}

func indentBlock(input, prefix string) string {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func (s *Session) touch() {
	s.UpdatedAt = time.Now().UTC()
}
