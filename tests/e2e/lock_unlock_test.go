package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

type lockResult struct {
	Success bool   `json:"success"`
	Session string `json:"session"`
	Agent   string `json:"agent"`
	Granted []struct {
		PathPattern string `json:"path_pattern"`
		AgentName   string `json:"agent_name"`
	} `json:"granted,omitempty"`
	Conflicts []struct {
		Path    string   `json:"path"`
		Holders []string `json:"holders"`
	} `json:"conflicts,omitempty"`
	Error string `json:"error,omitempty"`
}

type unlockResult struct {
	Success  bool   `json:"success"`
	Session  string `json:"session"`
	Agent    string `json:"agent"`
	Released int    `json:"released"`
	Error    string `json:"error,omitempty"`
}

func TestE2ELockUnlockFileReservations(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)
	client := requireAgentMail(t)

	logger := testutil.NewTestLoggerStdout(t)
	logger.LogSection("setup")

	// Isolate session agent files from the developer machine.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	projectDir := t.TempDir()
	lockPath := filepath.Join(projectDir, "internal", "cli", "send.go")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		t.Fatalf("mkdir lock path dir: %v", err)
	}
	if err := os.WriteFile(lockPath, []byte("package cli\n"), 0644); err != nil {
		t.Fatalf("write lock path: %v", err)
	}
	pattern := filepath.ToSlash("internal/cli/send.go")

	sessionA := "lock_unlock_a"
	sessionB := "lock_unlock_b"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	infoA, err := client.RegisterSessionAgent(ctx, sessionA, projectDir)
	cancel()
	if err != nil {
		t.Fatalf("register session A agent: %v", err)
	}
	if infoA == nil || infoA.AgentName == "" {
		t.Fatalf("register session A agent: missing agent name")
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	infoB, err := client.RegisterSessionAgent(ctx, sessionB, projectDir)
	cancel()
	if err != nil {
		t.Fatalf("register session B agent: %v", err)
	}
	if infoB == nil || infoB.AgentName == "" {
		t.Fatalf("register session B agent: missing agent name")
	}

	logger.Log("sessionA=%s agentA=%s", sessionA, infoA.AgentName)
	logger.Log("sessionB=%s agentB=%s", sessionB, infoB.AgentName)

	t.Cleanup(func() {
		// Best-effort cleanup so reservations don't leak into later tests/runs.
		_, _ = runCmdAllowFail(t, projectDir, "ntm", "--json", "unlock", sessionA, "--all")
		_, _ = runCmdAllowFail(t, projectDir, "ntm", "--json", "unlock", sessionB, "--all")
	})

	logger.LogSection("lock session A")
	out := runCmd(t, projectDir, "ntm", "--json", "lock", sessionA, pattern, "--ttl", "1h", "--reason", "e2e lock/unlock")
	logger.Log("expected: lock succeeds for sessionA")
	logger.Log("actual: %s", strings.TrimSpace(string(out)))

	var lockA lockResult
	if err := json.Unmarshal(out, &lockA); err != nil {
		t.Fatalf("unmarshal lockA: %v\nout=%s", err, string(out))
	}
	if !lockA.Success {
		t.Fatalf("expected lockA.success=true, got false (error=%q)", lockA.Error)
	}
	if lockA.Agent == "" {
		t.Fatalf("expected lockA.agent to be set")
	}
	if lockA.Agent != infoA.AgentName {
		t.Fatalf("expected lockA.agent=%q, got %q", infoA.AgentName, lockA.Agent)
	}

	logger.LogSection("verify reservation exists")
	reservations := listReservations(t, client, projectDir)
	logger.Log("expected: reservation present for %s", pattern)
	logger.Log("actual: count=%d", len(reservations))
	if !hasReservation(reservations, pattern) {
		t.Fatalf("expected reservation for %s to exist", pattern)
	}

	logger.LogSection("lock session B (conflict expected)")
	out = runCmd(t, projectDir, "ntm", "--json", "lock", sessionB, pattern, "--ttl", "1h", "--reason", "e2e conflict")
	logger.Log("expected: lock conflicts for sessionB; holders include %s", infoA.AgentName)
	logger.Log("actual: %s", strings.TrimSpace(string(out)))

	var lockB lockResult
	if err := json.Unmarshal(out, &lockB); err != nil {
		t.Fatalf("unmarshal lockB: %v\nout=%s", err, string(out))
	}
	if lockB.Success {
		t.Fatalf("expected lockB.success=false due to conflict, got true")
	}
	if len(lockB.Conflicts) == 0 {
		t.Fatalf("expected lockB.conflicts to be non-empty")
	}
	conflictFound := false
	holderFound := false
	for _, c := range lockB.Conflicts {
		if c.Path == pattern {
			conflictFound = true
			for _, holder := range c.Holders {
				if holder == infoA.AgentName {
					holderFound = true
				}
			}
		}
	}
	if !conflictFound {
		t.Fatalf("expected conflict entry for %s, got %+v", pattern, lockB.Conflicts)
	}
	if !holderFound {
		t.Fatalf("expected conflict holders to include %q, got %+v", infoA.AgentName, lockB.Conflicts)
	}

	logger.LogSection("verify both reservations exist")
	reservations = listReservations(t, client, projectDir)
	logger.Log("expected: 2 active reservations for %s (agentA + agentB)", pattern)
	logger.Log("actual: count=%d", len(reservations))

	activeCount := 0
	hasA := false
	hasB := false
	for _, r := range reservations {
		if r.PathPattern != pattern || r.ReleasedTS != nil {
			continue
		}
		activeCount++
		if r.AgentName == infoA.AgentName {
			hasA = true
		}
		if r.AgentName == infoB.AgentName {
			hasB = true
		}
	}
	if activeCount != 2 || !hasA || !hasB {
		t.Fatalf("expected active reservations for both agents (count=%d hasA=%v hasB=%v)", activeCount, hasA, hasB)
	}

	logger.LogSection("unlock session A")
	out = runCmd(t, projectDir, "ntm", "--json", "unlock", sessionA, pattern)
	logger.Log("expected: unlock succeeds for sessionA")
	logger.Log("actual: %s", strings.TrimSpace(string(out)))

	var unlockA unlockResult
	if err := json.Unmarshal(out, &unlockA); err != nil {
		t.Fatalf("unmarshal unlockA: %v\nout=%s", err, string(out))
	}
	if !unlockA.Success {
		t.Fatalf("expected unlockA.success=true, got false (error=%q)", unlockA.Error)
	}
	if unlockA.Agent != infoA.AgentName {
		t.Fatalf("expected unlockA.agent=%q, got %q", infoA.AgentName, unlockA.Agent)
	}
	if unlockA.Released != 1 {
		t.Fatalf("expected unlockA.released=1, got %d", unlockA.Released)
	}

	logger.LogSection("verify reservation released")
	reservations = listReservations(t, client, projectDir)
	logger.Log("expected: reservation held only by agentB for %s", pattern)
	logger.Log("actual: count=%d", len(reservations))

	activeCount = 0
	hasA = false
	hasB = false
	for _, r := range reservations {
		if r.PathPattern != pattern || r.ReleasedTS != nil {
			continue
		}
		activeCount++
		if r.AgentName == infoA.AgentName {
			hasA = true
		}
		if r.AgentName == infoB.AgentName {
			hasB = true
		}
	}
	if activeCount != 1 || hasA || !hasB {
		t.Fatalf("expected active reservation only for agentB (count=%d hasA=%v hasB=%v)", activeCount, hasA, hasB)
	}

	logger.LogSection("unlock session B (cleanup)")
	out = runCmd(t, projectDir, "ntm", "--json", "unlock", sessionB, "--all")
	logger.Log("expected: unlock succeeds for sessionB")
	logger.Log("actual: %s", strings.TrimSpace(string(out)))

	var unlockB unlockResult
	if err := json.Unmarshal(out, &unlockB); err != nil {
		t.Fatalf("unmarshal unlockB: %v\nout=%s", err, string(out))
	}
	if !unlockB.Success {
		t.Fatalf("expected unlockB.success=true, got false (error=%q)", unlockB.Error)
	}
}

func TestE2ELockRejectsTTLBelowOneMinute(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireNTMBinary(t)
	client := requireAgentMail(t)

	// Isolate session agent files from the developer machine.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	projectDir := t.TempDir()
	lockPath := filepath.Join(projectDir, "file.txt")
	if err := os.WriteFile(lockPath, []byte("ok\n"), 0644); err != nil {
		t.Fatalf("write lock path: %v", err)
	}

	session := "lock_ttl_invalid"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	info, err := client.RegisterSessionAgent(ctx, session, projectDir)
	cancel()
	if err != nil {
		t.Fatalf("register session agent: %v", err)
	}
	if info == nil || info.AgentName == "" {
		t.Fatalf("register session agent: missing agent name")
	}

	out, err := runCmdAllowFail(t, projectDir, "ntm", "lock", session, "file.txt", "--ttl", "30s")
	if err == nil {
		t.Fatalf("expected ntm lock to fail for TTL < 1m, got success; out=%s", string(out))
	}
	if !strings.Contains(string(out), "TTL must be at least 1 minute") {
		t.Fatalf("expected error mentioning min TTL, got:\n%s", string(out))
	}
}
