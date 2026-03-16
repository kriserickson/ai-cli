package interactive

import (
	"testing"

	"github.com/kriserickson/ai-cli/internal/runner"
)

func TestPrintHelp(_ *testing.T) {
	printHelp()
}

func TestParseRetryDepth(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{input: "retry", want: 0},
		{input: "retry 3", want: 3},
		{input: "retry nope", wantErr: true},
		{input: "retry 0", wantErr: true},
	}

	for _, tt := range tests {
		got, err := runner.ParseRetryDepth(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ParseRetryDepth(%q) error = nil, want error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseRetryDepth(%q) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("ParseRetryDepth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
