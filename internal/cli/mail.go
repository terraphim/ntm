package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/agentmail"
	"github.com/Dicklesworthstone/ntm/internal/tmux"
	"github.com/spf13/cobra"
)

func newMailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mail",
		Short: "Agent Mail operations",
		Long: `Interact with the MCP Agent Mail system for multi-agent coordination.

The mail command provides subcommands for sending and reading messages
between agents in a session.

Examples:
  ntm mail send myproject --to cc_1 "Please review the API changes"
  ntm mail send myproject --all "Checkpoint: sync and report status"`,
	}

	cmd.AddCommand(newMailSendCmd())

	return cmd
}

func newMailSendCmd() *cobra.Command {
	var (
		to          []string
		ccAddr      []string
		subject     string
		urgent      bool
		important   bool
		ackRequired bool
		threadID    string
		all         bool
		targetCC    bool // all Claude agents
		targetCod   bool // all Codex agents
		targetGmi   bool // all Gemini agents
		fromFile    string
	)

	cmd := &cobra.Command{
		Use:   "send <session> <message>",
		Short: "Send a message to agents",
		Long: `Send a message to agents in a session via Agent Mail.

Recipients can be specified by:
  - Pane identifiers: cc_1, cod_2 (resolved to agent names)
  - Agent names: GreenCastle (used directly)
  - Type filters: --cc, --cod, --gmi (all agents of that type)
  - All agents: --all

Examples:
  ntm mail send myproject --to cc_1 "Please review the API changes"
  ntm mail send myproject --to GreenCastle --subject "API Review" "Review needed"
  ntm mail send myproject --cc "All Claude agents: focus on tests"
  ntm mail send myproject --all --urgent "Stop and checkpoint now"
  ntm mail send myproject --to cc_1 --file ./message.md`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session := args[0]

			// Get message body
			var body string
			if fromFile != "" {
				content, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				body = string(content)
			} else if len(args) > 1 {
				body = strings.Join(args[1:], " ")
			} else {
				return fmt.Errorf("message body required (provide as argument or use --file)")
			}

			// Determine importance
			importance := "normal"
			if urgent {
				importance = "urgent"
			} else if important {
				importance = "high"
			}

			return runMailSend(session, to, ccAddr, subject, body, importance, ackRequired, threadID, all, targetCC, targetCod, targetGmi)
		},
	}

	cmd.Flags().StringArrayVar(&to, "to", nil, "recipient agent (pane ID or name)")
	cmd.Flags().StringArrayVar(&ccAddr, "cc-addr", nil, "CC recipient agent")
	cmd.Flags().StringVarP(&subject, "subject", "s", "", "message subject")
	cmd.Flags().BoolVar(&urgent, "urgent", false, "mark message as urgent")
	cmd.Flags().BoolVar(&important, "important", false, "mark message as high importance")
	cmd.Flags().BoolVar(&ackRequired, "ack-required", false, "require acknowledgment")
	cmd.Flags().StringVar(&threadID, "thread", "", "thread ID for conversation")
	cmd.Flags().BoolVar(&all, "all", false, "send to all agents in session")
	cmd.Flags().BoolVar(&targetCC, "cc", false, "send to all Claude agents")
	cmd.Flags().BoolVar(&targetCod, "cod", false, "send to all Codex agents")
	cmd.Flags().BoolVar(&targetGmi, "gmi", false, "send to all Gemini agents")
	cmd.Flags().StringVarP(&fromFile, "file", "f", "", "read message body from file")

	return cmd
}

func runMailSend(session string, to, ccAddr []string, subject, body, importance string, ackRequired bool, threadID string, all, targetCC, targetCod, targetGmi bool) error {
	if err := tmux.EnsureInstalled(); err != nil {
		return err
	}

	if !tmux.SessionExists(session) {
		return fmt.Errorf("session '%s' not found", session)
	}

	// Get project key (current working directory)
	projectKey, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Create Agent Mail client
	client := agentmail.NewClient(agentmail.WithProjectKey(projectKey))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if Agent Mail is available
	if !client.IsAvailable() {
		return fmt.Errorf("Agent Mail server not available at %s", agentmail.DefaultBaseURL)
	}

	// Resolve recipients
	recipients, err := resolveRecipients(session, to, all, targetCC, targetCod, targetGmi)
	if err != nil {
		return err
	}

	if len(recipients) == 0 {
		return fmt.Errorf("no recipients specified (use --to, --all, --cc, --cod, or --gmi)")
	}

	// Get sender name from environment or generate
	senderName := os.Getenv("AGENT_NAME")
	if senderName == "" {
		senderName = fmt.Sprintf("NtmCli%s", strings.Title(session))
	}

	// Ensure project exists
	_, err = client.EnsureProject(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("ensuring project: %w", err)
	}

	// Register sender if needed
	_, err = client.RegisterAgent(ctx, agentmail.RegisterAgentOptions{
		ProjectKey:      projectKey,
		Name:            senderName,
		Program:         "ntm-cli",
		Model:           "user",
		TaskDescription: "CLI message sender",
	})
	if err != nil {
		// Ignore registration errors (may already exist)
	}

	// Auto-generate subject if not provided
	if subject == "" {
		subject = truncateSubject(body, 60)
	}

	// Send message
	result, err := client.SendMessage(ctx, agentmail.SendMessageOptions{
		ProjectKey:  projectKey,
		SenderName:  senderName,
		To:          recipients,
		CC:          ccAddr,
		Subject:     subject,
		BodyMD:      body,
		Importance:  importance,
		AckRequired: ackRequired,
		ThreadID:    threadID,
	})
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	// Output result
	var messageID int
	if len(result.Deliveries) > 0 && result.Deliveries[0].Payload != nil {
		messageID = result.Deliveries[0].Payload.ID
	}

	if IsJSONOutput() {
		return encodeJSONResult(map[string]interface{}{
			"success":    true,
			"recipients": recipients,
			"subject":    subject,
			"message_id": messageID,
			"count":      result.Count,
		})
	}

	fmt.Printf("Message sent to %d recipient(s)\n", result.Count)
	for _, r := range recipients {
		fmt.Printf("  → %s\n", r)
	}

	return nil
}

// resolveRecipients resolves recipient specifiers to agent names.
func resolveRecipients(session string, to []string, all, targetCC, targetCod, targetGmi bool) ([]string, error) {
	var recipients []string

	// Get panes for resolution
	panes, err := tmux.GetPanes(session)
	if err != nil {
		return nil, fmt.Errorf("getting panes: %w", err)
	}

	// Build pane map for resolution
	paneMap := make(map[string]tmux.Pane) // e.g., "cc_1" -> pane
	for _, p := range panes {
		// Generate pane identifier based on type
		var prefix string
		switch p.Type {
		case tmux.AgentClaude:
			prefix = "cc"
		case tmux.AgentCodex:
			prefix = "cod"
		case tmux.AgentGemini:
			prefix = "gmi"
		default:
			prefix = "user"
		}
		id := fmt.Sprintf("%s_%d", prefix, p.Index)
		paneMap[id] = p
	}

	// Process --all flag
	if all {
		for _, p := range panes {
			if p.Type != tmux.AgentUser {
				name := resolveAgentName(p)
				if name != "" {
					recipients = append(recipients, name)
				}
			}
		}
	}

	// Process type filters
	if targetCC {
		for _, p := range panes {
			if p.Type == tmux.AgentClaude {
				name := resolveAgentName(p)
				if name != "" {
					recipients = append(recipients, name)
				}
			}
		}
	}
	if targetCod {
		for _, p := range panes {
			if p.Type == tmux.AgentCodex {
				name := resolveAgentName(p)
				if name != "" {
					recipients = append(recipients, name)
				}
			}
		}
	}
	if targetGmi {
		for _, p := range panes {
			if p.Type == tmux.AgentGemini {
				name := resolveAgentName(p)
				if name != "" {
					recipients = append(recipients, name)
				}
			}
		}
	}

	// Process --to recipients
	for _, recipient := range to {
		// Check if it's a pane identifier
		if p, ok := paneMap[recipient]; ok {
			name := resolveAgentName(p)
			if name != "" {
				recipients = append(recipients, name)
			}
		} else {
			// Assume it's an agent name
			recipients = append(recipients, recipient)
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	unique := make([]string, 0, len(recipients))
	for _, r := range recipients {
		if !seen[r] {
			seen[r] = true
			unique = append(unique, r)
		}
	}

	return unique, nil
}

// resolveAgentName tries to get the agent name from a pane.
func resolveAgentName(p tmux.Pane) string {
	// Try pane title first (may contain agent name)
	if p.Title != "" && !strings.HasPrefix(p.Title, "pane") {
		if looksLikeAgentName(p.Title) {
			return p.Title
		}
	}

	// Fall back to generated name based on type and index
	var prefix string
	switch p.Type {
	case tmux.AgentClaude:
		prefix = "Claude"
	case tmux.AgentCodex:
		prefix = "Codex"
	case tmux.AgentGemini:
		prefix = "Gemini"
	default:
		return ""
	}
	return fmt.Sprintf("%sAgent%d", prefix, p.Index)
}

// looksLikeAgentName checks if a string looks like an AdjectiveNoun agent name.
func looksLikeAgentName(s string) bool {
	if strings.Contains(s, " ") || strings.Contains(s, "_") || strings.Contains(s, "-") {
		return false
	}
	if len(s) == 0 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			return true
		}
	}
	return false
}

// truncateSubject creates a subject from message body.
func truncateSubject(body string, maxLen int) string {
	lines := strings.SplitN(body, "\n", 2)
	subject := strings.TrimSpace(lines[0])

	// Remove markdown heading prefix
	subject = strings.TrimPrefix(subject, "# ")
	subject = strings.TrimPrefix(subject, "## ")
	subject = strings.TrimPrefix(subject, "### ")

	if len(subject) > maxLen {
		return subject[:maxLen-3] + "..."
	}
	return subject
}

// encodeJSONResult is a helper to output JSON.
func encodeJSONResult(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
