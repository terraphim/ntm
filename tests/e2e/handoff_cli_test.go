package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

func TestHandoffCLI_CreateListShow(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)

	logger := testutil.NewTestLoggerStdout(t)

	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer os.Chdir(oldWd)

	session := "handoff-e2e"
	goal := "Completed handoff e2e create"
	now := "Verify list and show"

	logger.LogSection("handoff create")
	createOut := testutil.AssertCommandSuccess(t, logger, "ntm", "handoff", "create", session, "--goal", goal, "--now", now, "--format", "json")

	var createResp struct {
		Success bool   `json:"success"`
		Path    string `json:"path"`
		Session string `json:"session"`
		Goal    string `json:"goal"`
		Now     string `json:"now"`
	}
	if err := json.Unmarshal(createOut, &createResp); err != nil {
		t.Fatalf("create output invalid JSON: %v", err)
	}
	if !createResp.Success {
		t.Fatalf("create output success=false")
	}
	if createResp.Path == "" {
		t.Fatalf("create output missing path")
	}
	if createResp.Session != session {
		t.Fatalf("create output session=%q, want %q", createResp.Session, session)
	}
	if createResp.Goal != goal || createResp.Now != now {
		t.Fatalf("create output goal/now mismatch")
	}
	if _, err := os.Stat(createResp.Path); err != nil {
		t.Fatalf("handoff file not found at %s: %v", createResp.Path, err)
	}

	logger.LogSection("handoff list")
	listOut := testutil.AssertCommandSuccess(t, logger, "ntm", "handoff", "list", session, "--json")

	var listResp struct {
		Session  string `json:"session"`
		Count    int    `json:"count"`
		Handoffs []struct {
			Path    string `json:"path"`
			Session string `json:"session"`
		} `json:"handoffs"`
	}
	if err := json.Unmarshal(listOut, &listResp); err != nil {
		t.Fatalf("list output invalid JSON: %v", err)
	}
	if listResp.Session != session {
		t.Fatalf("list output session=%q, want %q", listResp.Session, session)
	}
	if listResp.Count < 1 || len(listResp.Handoffs) < 1 {
		t.Fatalf("list output missing handoffs (count=%d)", listResp.Count)
	}

	handoffPath := listResp.Handoffs[0].Path
	if !filepath.IsAbs(handoffPath) {
		handoffPath = filepath.Join(tmpDir, handoffPath)
	}

	logger.LogSection("handoff show")
	showOut := testutil.AssertCommandSuccess(t, logger, "ntm", "handoff", "show", handoffPath, "--json")

	var showResp struct {
		Session string `json:"session"`
		Goal    string `json:"goal"`
		Now     string `json:"now"`
	}
	if err := json.Unmarshal(showOut, &showResp); err != nil {
		t.Fatalf("show output invalid JSON: %v", err)
	}
	if showResp.Session != session {
		t.Fatalf("show output session=%q, want %q", showResp.Session, session)
	}
	if showResp.Goal != goal || showResp.Now != now {
		t.Fatalf("show output goal/now mismatch")
	}

	logger.LogSection("handoff resume")
	resumeOut := testutil.AssertCommandSuccess(t, logger, "ntm", "resume", session, "--json")

	var resumeResp struct {
		Success bool   `json:"success"`
		Action  string `json:"action"`
		Handoff struct {
			Path    string `json:"path"`
			Session string `json:"session"`
			Goal    string `json:"goal"`
			Now     string `json:"now"`
		} `json:"handoff"`
	}
	if err := json.Unmarshal(resumeOut, &resumeResp); err != nil {
		t.Fatalf("resume output invalid JSON: %v", err)
	}
	if !resumeResp.Success {
		t.Fatalf("resume output success=false")
	}
	if resumeResp.Action != "display" {
		t.Fatalf("resume output action=%q, want %q", resumeResp.Action, "display")
	}
	if resumeResp.Handoff.Session != session {
		t.Fatalf("resume output session=%q, want %q", resumeResp.Handoff.Session, session)
	}
	if resumeResp.Handoff.Goal != goal || resumeResp.Handoff.Now != now {
		t.Fatalf("resume output goal/now mismatch")
	}
}
