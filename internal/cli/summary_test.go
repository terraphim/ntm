package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSummaryFilename(t *testing.T) {
	session, ts, ok := parseSummaryFilename("my-session-20260128-101112.json")
	if !ok {
		t.Fatalf("expected filename to parse")
	}
	if session != "my-session" {
		t.Fatalf("expected session my-session, got %q", session)
	}
	if ts.IsZero() {
		t.Fatalf("expected timestamp to parse")
	}

	if _, _, ok := parseSummaryFilename("badname.json"); ok {
		t.Fatalf("expected bad filename to fail")
	}
}

func TestParseArchiveFilename(t *testing.T) {
	session, ts, ok := parseArchiveFilename("my_session_2026-01-28.jsonl")
	if !ok {
		t.Fatalf("expected archive filename to parse")
	}
	if session != "my_session" {
		t.Fatalf("expected session my_session, got %q", session)
	}
	if ts.IsZero() {
		t.Fatalf("expected timestamp to parse")
	}

	if _, _, ok := parseArchiveFilename("bad.jsonl"); ok {
		t.Fatalf("expected invalid archive filename to fail")
	}
}

func TestListSummaryFilesSortsByTime(t *testing.T) {
	dir := t.TempDir()
	summaryDir := filepath.Join(dir, ".ntm", "summaries")
	if err := os.MkdirAll(summaryDir, 0755); err != nil {
		t.Fatalf("failed to create summary dir: %v", err)
	}

	files := []string{
		filepath.Join(summaryDir, "alpha-20260128-101112.json"),
		filepath.Join(summaryDir, "alpha-20260129-091011.json"),
		filepath.Join(summaryDir, "beta-20260127-090000.json"),
	}
	for _, path := range files {
		if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
			t.Fatalf("failed to write summary file: %v", err)
		}
	}

	list, err := listSummaryFiles(dir)
	if err != nil {
		t.Fatalf("listSummaryFiles: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(list))
	}

	if list[0].Timestamp.Before(list[1].Timestamp) {
		t.Fatalf("expected summaries sorted descending by timestamp")
	}

	if list[0].Session != "alpha" {
		t.Fatalf("expected latest session alpha, got %q", list[0].Session)
	}
}

func TestResolveSummarySessionName(t *testing.T) {
	now := time.Now()
	files := []summaryFileInfo{
		{Session: "alpha", Timestamp: now},
		{Session: "beta", Timestamp: now},
		{Session: "alphonse", Timestamp: now},
	}

	resolved, ok, err := resolveSummarySessionName("beta", files)
	if err != nil || !ok || resolved != "beta" {
		t.Fatalf("expected exact match beta, got %q (ok=%v, err=%v)", resolved, ok, err)
	}

	resolved, ok, err = resolveSummarySessionName("alph", files)
	if err == nil || ok {
		t.Fatalf("expected ambiguous prefix error, got %q (ok=%v, err=%v)", resolved, ok, err)
	}

	resolved, ok, err = resolveSummarySessionName("alp", []summaryFileInfo{{Session: "alpha", Timestamp: now}})
	if err != nil || !ok || resolved != "alpha" {
		t.Fatalf("expected prefix match alpha, got %q (ok=%v, err=%v)", resolved, ok, err)
	}
}
