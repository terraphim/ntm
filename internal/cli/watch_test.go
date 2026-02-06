package cli

import (
	"testing"
	"time"
)

func TestParseWatchInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "default", input: "", want: 250 * time.Millisecond},
		{name: "duration", input: "2s", want: 2 * time.Second},
		{name: "milliseconds integer", input: "500", want: 500 * time.Millisecond},
		{name: "invalid", input: "abc", wantErr: true},
		{name: "zero invalid", input: "0", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseWatchInterval(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWatchInterval returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("duration = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractBeadMentions(t *testing.T) {
	t.Parallel()

	re, err := beadMentionRegexp("bd-123")
	if err != nil {
		t.Fatalf("beadMentionRegexp error: %v", err)
	}

	input := "working on bd-123 now\nnoise line\nbd-1234 should not match\nDone with BD-123"
	got := extractBeadMentions(input, re)

	if len(got) != 2 {
		t.Fatalf("mentions count = %d, want 2", len(got))
	}
	if got[0] != "working on bd-123 now" {
		t.Fatalf("first mention = %q", got[0])
	}
	if got[1] != "Done with BD-123" {
		t.Fatalf("second mention = %q", got[1])
	}
}
