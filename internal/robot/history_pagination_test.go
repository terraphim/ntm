package robot

import (
	"testing"

	"github.com/Dicklesworthstone/ntm/internal/history"
)

func TestGetHistoryPagination(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	session := "history-pagination-session"
	entries := []*history.HistoryEntry{
		history.NewEntry(session, []string{"1"}, "first", history.SourceCLI),
		history.NewEntry(session, []string{"1"}, "second", history.SourceCLI),
		history.NewEntry(session, []string{"1"}, "third", history.SourceCLI),
	}
	for _, entry := range entries {
		entry.SetSuccess()
	}

	if err := history.BatchAppend(entries); err != nil {
		t.Fatalf("failed to write history: %v", err)
	}

	output, err := GetHistory(HistoryOptions{
		Session: session,
		Limit:   1,
		Offset:  1,
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if output.Pagination == nil {
		t.Fatal("expected pagination info, got nil")
	}
	if output.Pagination.Total != 3 || output.Pagination.Count != 1 {
		t.Fatalf("unexpected pagination info: %+v", output.Pagination)
	}
	if !output.Pagination.HasMore || output.Pagination.NextCursor == nil || *output.Pagination.NextCursor != 2 {
		t.Fatalf("expected next_cursor=2 and has_more=true, got %+v", output.Pagination)
	}
	if output.AgentHints == nil || output.AgentHints.NextOffset == nil || *output.AgentHints.NextOffset != 2 {
		t.Fatalf("expected _agent_hints.next_offset=2, got %+v", output.AgentHints)
	}
	if output.AgentHints.PagesRemaining == nil || *output.AgentHints.PagesRemaining != 1 {
		t.Fatalf("expected _agent_hints.pages_remaining=1, got %+v", output.AgentHints)
	}
	if len(output.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(output.Entries))
	}
	if output.Entries[0].Prompt != "second" {
		t.Fatalf("expected second entry, got %q", output.Entries[0].Prompt)
	}
}
