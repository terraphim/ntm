package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/agentmail"
	"github.com/Dicklesworthstone/ntm/internal/tmux"
	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

func TestE2EAgentMailCommunicationFlow(t *testing.T) {
	testutil.RequireE2E(t)
	testutil.RequireTmuxThrottled(t)
	testutil.RequireNTMBinary(t)

	client := requireAgentMail(t)
	logger := testutil.NewTestLoggerStdout(t)

	session := fmt.Sprintf("am_comm_%d", time.Now().UnixNano())
	projectsBase := t.TempDir()
	projectDir := filepath.Join(projectsBase, session)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	// Override XDG_CONFIG_HOME to isolate session data (agent registry).
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	configPath := writeAgentMailTestConfig(t, projectsBase)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := client.EnsureProject(ctx, projectDir)
	cancel()
	if err != nil {
		t.Fatalf("ensure Agent Mail project: %v", err)
	}

	t.Cleanup(func() {
		_ = exec.Command(tmux.BinaryPath(), "kill-session", "-t", session).Run()
	})

	logger.Log("[E2E-COMM] Spawning test session: session=%s project=%s", session, projectDir)
	runCmd(t, projectDir, "ntm", "--config", configPath, "spawn", session, "--cc=3")
	time.Sleep(750 * time.Millisecond)

	agentNames := waitForAgentNames(t, logger, session, projectDir, 3)
	if len(agentNames) < 3 {
		t.Fatalf("expected at least 3 registered agents, got %d", len(agentNames))
	}

	agentA := agentNames[0]
	agentB := agentNames[1]
	agentC := agentNames[2]
	logger.Log("[E2E-COMM] Agents: A=%s B=%s C=%s", agentA, agentB, agentC)

	t.Run("basic_send_receive_threaded_reply", func(t *testing.T) {
		threadID := "bd-3q3u-basic"
		subject := fmt.Sprintf("basic send %d", time.Now().UnixNano())
		body := fmt.Sprintf("hello from %s to %s (thread=%s)", agentA, agentB, threadID)

		logger.Log("[E2E-COMM] Sending: from=%s to=%s subject=%q thread=%s", agentA, agentB, subject, threadID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := client.SendMessage(ctx, agentmail.SendMessageOptions{
			ProjectKey:  projectDir,
			SenderName:  agentA,
			To:          []string{agentB},
			Subject:     subject,
			BodyMD:      body,
			ThreadID:    threadID,
			Importance:  "normal",
			AckRequired: false,
		})
		cancel()
		if err != nil {
			t.Fatalf("send message: %v", err)
		}

		msgToB := waitForInboxMessage(t, logger, client, projectDir, agentB, threadID, subject, agentA, false)
		if !strings.Contains(msgToB.BodyMD, body) {
			t.Fatalf("unexpected body_md (want contains %q): %q", body, msgToB.BodyMD)
		}

		replyBody := fmt.Sprintf("reply from %s to %s", agentB, agentA)
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		reply, err := client.ReplyMessage(ctx, agentmail.ReplyMessageOptions{
			ProjectKey:    projectDir,
			MessageID:     msgToB.ID,
			SenderName:    agentB,
			BodyMD:        replyBody,
			To:            []string{agentA},
			SubjectPrefix: "Re:",
		})
		cancel()
		if err != nil {
			t.Fatalf("reply message: %v", err)
		}
		if reply.ThreadID == nil || *reply.ThreadID != threadID {
			t.Fatalf("reply thread mismatch: got %v, want %q", reply.ThreadID, threadID)
		}

		expectedReplySubject := reply.Subject
		if !strings.HasPrefix(strings.ToLower(expectedReplySubject), "re:") {
			t.Fatalf("expected reply subject to be prefixed with Re:, got %q", expectedReplySubject)
		}

		msgToA := waitForInboxMessage(t, logger, client, projectDir, agentA, threadID, expectedReplySubject, agentB, false)
		if !strings.Contains(msgToA.BodyMD, replyBody) {
			t.Fatalf("unexpected reply body_md (want contains %q): %q", replyBody, msgToA.BodyMD)
		}
	})

	t.Run("broadcast_messaging_no_duplicates", func(t *testing.T) {
		threadID := "bd-3q3u-broadcast"
		subject := fmt.Sprintf("broadcast %d", time.Now().UnixNano())
		body := fmt.Sprintf("broadcast from %s to %s,%s", agentA, agentB, agentC)

		logger.Log("[E2E-COMM] Broadcasting: from=%s to=[%s %s] subject=%q thread=%s", agentA, agentB, agentC, subject, threadID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := client.SendMessage(ctx, agentmail.SendMessageOptions{
			ProjectKey:  projectDir,
			SenderName:  agentA,
			To:          []string{agentB, agentC},
			Subject:     subject,
			BodyMD:      body,
			ThreadID:    threadID,
			Importance:  "normal",
			AckRequired: false,
		})
		cancel()
		if err != nil {
			t.Fatalf("send broadcast: %v", err)
		}

		waitForInboxMessage(t, logger, client, projectDir, agentB, threadID, subject, agentA, false)
		waitForInboxMessage(t, logger, client, projectDir, agentC, threadID, subject, agentA, false)

		assertInboxMessageCount(t, logger, client, projectDir, agentB, threadID, subject, agentA, 1)
		assertInboxMessageCount(t, logger, client, projectDir, agentC, threadID, subject, agentA, 1)
	})

	t.Run("urgent_message_ack_flow", func(t *testing.T) {
		threadID := "bd-3q3u-urgent"
		subject := fmt.Sprintf("urgent %d", time.Now().UnixNano())
		body := fmt.Sprintf("URGENT: please ack (from=%s)", agentA)

		logger.Log("[E2E-COMM] Urgent send: from=%s to=%s subject=%q thread=%s", agentA, agentB, subject, threadID)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := client.SendMessage(ctx, agentmail.SendMessageOptions{
			ProjectKey:  projectDir,
			SenderName:  agentA,
			To:          []string{agentB},
			Subject:     subject,
			BodyMD:      body,
			ThreadID:    threadID,
			Importance:  "urgent",
			AckRequired: true,
		})
		cancel()
		if err != nil {
			t.Fatalf("send urgent: %v", err)
		}

		msg := waitForInboxMessage(t, logger, client, projectDir, agentB, threadID, subject, agentA, true)
		if !msg.AckRequired {
			t.Fatalf("expected ack_required=true for urgent message")
		}
		if !strings.EqualFold(msg.Importance, "urgent") && !strings.EqualFold(msg.Importance, "high") {
			t.Fatalf("expected importance urgent/high, got %q", msg.Importance)
		}

		// Acknowledge via `ntm mail ack` (exercises CLI integration as well).
		out := runCmd(t, projectDir, "ntm", "--config", configPath, "mail", "ack", session, fmt.Sprintf("%d", msg.ID), "--agent", agentB, "--json")
		testutil.AssertJSONOutput(t, logger, out)

		var summary struct {
			Action    string `json:"action"`
			Agent     string `json:"agent"`
			Processed int    `json:"processed"`
			Errors    int    `json:"errors"`
			IDs       []int  `json:"ids"`
		}
		if err := json.Unmarshal(out, &summary); err != nil {
			t.Fatalf("parse ack json: %v\n%s", err, string(out))
		}
		if summary.Action != "ack" || summary.Agent != agentB || summary.Processed != 1 || summary.Errors != 0 {
			t.Fatalf("unexpected ack summary: %+v", summary)
		}
		if len(summary.IDs) != 1 || summary.IDs[0] != msg.ID {
			t.Fatalf("unexpected ack ids: %+v", summary.IDs)
		}
	})

	t.Run("ntm_mail_cli_smoke_send_inbox_read", func(t *testing.T) {
		threadID := "bd-3q3u-mailcli"
		subject := fmt.Sprintf("overseer ping %d", time.Now().UnixNano())
		body := "Stop current work and checkpoint"

		logger.Log("[E2E-COMM] ntm mail send: to=%s subject=%q thread=%s", agentC, subject, threadID)
		runCmd(t, projectDir, "ntm", "--config", configPath, "mail", "send", session, "--to", agentC, "--subject", subject, "--thread", threadID, body)

		inboxJSON := runCmd(t, projectDir, "ntm", "--config", configPath, "mail", "inbox", session, "--agent", agentC, "--json")
		testutil.AssertJSONOutput(t, logger, inboxJSON)

		var msgs []struct {
			ID      int      `json:"id"`
			Subject string   `json:"subject"`
			From    string   `json:"from"`
			Targets []string `json:"recipients"`
		}
		if err := json.Unmarshal(inboxJSON, &msgs); err != nil {
			t.Fatalf("parse inbox json: %v\n%s", err, string(inboxJSON))
		}

		found := false
		var messageID int
		var messageFrom string
		for _, m := range msgs {
			if m.Subject != subject {
				continue
			}
			messageID = m.ID
			messageFrom = m.From
			found = true
			if m.From == "" {
				t.Fatalf("expected non-empty sender in inbox entry: %+v", m)
			}
			break
		}
		if !found {
			t.Fatalf("expected message %q to appear in inbox JSON for %s", subject, agentC)
		}

		// Verify thread continuity via direct inbox fetch (CLI inbox JSON does not include thread_id).
		waitForInboxMessage(t, logger, client, projectDir, agentC, threadID, subject, messageFrom, false)

		// Mark as read via CLI (smoke).
		out := runCmd(t, projectDir, "ntm", "--config", configPath, "mail", "read", session, fmt.Sprintf("%d", messageID), "--agent", agentC, "--json")
		testutil.AssertJSONOutput(t, logger, out)
	})
}

func waitForAgentNames(t *testing.T, logger *testutil.TestLogger, session, projectDir string, expectedCount int) []string {
	t.Helper()

	var registry *agentmail.SessionAgentRegistry
	ok := testutil.AssertEventually(t, logger, 15*time.Second, 300*time.Millisecond, "session agent registry populated", func() bool {
		r, err := agentmail.LoadSessionAgentRegistry(session, projectDir)
		if err != nil {
			logger.Log("[E2E-COMM] registry load error: %v", err)
			return false
		}
		if r == nil || r.Count() < expectedCount {
			return false
		}
		registry = r
		return true
	})
	if !ok {
		t.Fatalf("timed out waiting for agent registry")
	}

	titles := make([]string, 0, len(registry.Agents))
	for title := range registry.Agents {
		titles = append(titles, title)
	}
	sort.Strings(titles)

	out := make([]string, 0, len(titles))
	for _, title := range titles {
		name := strings.TrimSpace(registry.Agents[title])
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func waitForInboxMessage(t *testing.T, logger *testutil.TestLogger, client *agentmail.Client, projectDir, agentName, threadID, subject, from string, urgentOnly bool) agentmail.InboxMessage {
	t.Helper()

	var found agentmail.InboxMessage
	desc := fmt.Sprintf("inbox message delivered: agent=%s subject=%q", agentName, subject)
	ok := testutil.AssertEventually(t, logger, 15*time.Second, 300*time.Millisecond, desc, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		inbox, err := client.FetchInbox(ctx, agentmail.FetchInboxOptions{
			ProjectKey:    projectDir,
			AgentName:     agentName,
			UrgentOnly:    urgentOnly,
			IncludeBodies: true,
			Limit:         50,
		})
		if err != nil {
			logger.Log("[E2E-COMM] fetch inbox error: %v", err)
			return false
		}
		for _, msg := range inbox {
			if msg.Subject != subject {
				continue
			}
			if from != "" && !strings.EqualFold(msg.From, from) {
				continue
			}
			if threadID != "" {
				if msg.ThreadID == nil || *msg.ThreadID != threadID {
					continue
				}
			}
			found = msg
			return true
		}
		return false
	})
	if !ok {
		t.Fatalf("expected message not found: agent=%s subject=%q", agentName, subject)
	}
	return found
}

func assertInboxMessageCount(t *testing.T, logger *testutil.TestLogger, client *agentmail.Client, projectDir, agentName, threadID, subject, from string, want int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	inbox, err := client.FetchInbox(ctx, agentmail.FetchInboxOptions{
		ProjectKey:    projectDir,
		AgentName:     agentName,
		UrgentOnly:    false,
		IncludeBodies: false,
		Limit:         200,
	})
	if err != nil {
		t.Fatalf("fetch inbox for count: %v", err)
	}

	got := 0
	for _, msg := range inbox {
		if msg.Subject != subject {
			continue
		}
		if from != "" && !strings.EqualFold(msg.From, from) {
			continue
		}
		if threadID != "" {
			if msg.ThreadID == nil || *msg.ThreadID != threadID {
				continue
			}
		}
		got++
	}
	logger.Log("[E2E-COMM] Inbox count: agent=%s subject=%q got=%d want=%d", agentName, subject, got, want)
	if got != want {
		t.Fatalf("unexpected message count: agent=%s subject=%q got=%d want=%d", agentName, subject, got, want)
	}
}
