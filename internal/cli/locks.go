package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/agentmail"
	"github.com/spf13/cobra"
)

func newLocksCmd() *cobra.Command {
	var allAgents bool

	cmd := &cobra.Command{
		Use:   "locks <session>",
		Short: "Show current file reservations",
		Long: `Display file path reservations for this session or all agents in the project.

Examples:
  ntm locks myproject               # Show session's reservations
  ntm locks myproject --all-agents  # Show all project reservations
  ntm locks myproject --json        # JSON output for scripts`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session := args[0]
			return runLocks(session, allAgents)
		},
	}

	cmd.Flags().BoolVar(&allAgents, "all-agents", false, "Show reservations for all agents")

	return cmd
}

// LocksResult contains the list of active file reservations.
type LocksResult struct {
	Success      bool                        `json:"success"`
	Session      string                      `json:"session"`
	Agent        string                      `json:"agent,omitempty"`
	ProjectKey   string                      `json:"project_key"`
	Reservations []agentmail.FileReservation `json:"reservations"`
	Count        int                         `json:"count"`
	Error        string                      `json:"error,omitempty"`
}

func runLocks(session string, allAgents bool) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	sessionAgent, err := agentmail.LoadSessionAgent(session)
	if err != nil {
		return fmt.Errorf("loading session agent: %w", err)
	}

	agentName := ""
	if sessionAgent != nil {
		agentName = sessionAgent.AgentName
	}

	client := agentmail.NewClient(agentmail.WithProjectKey(wd))
	if !client.IsAvailable() {
		if IsJSONOutput() {
			result := LocksResult{Success: false, Session: session, Agent: agentName, ProjectKey: wd, Error: "Agent Mail server unavailable"}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		return fmt.Errorf("Agent Mail server unavailable")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reservations, err := fetchActiveReservations(ctx, client, wd, agentName, allAgents)

	result := LocksResult{
		Session:      session,
		Agent:        agentName,
		ProjectKey:   wd,
		Reservations: reservations,
		Count:        len(reservations),
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	if IsJSONOutput() {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	return printLocksResult(result, allAgents)
}

func fetchActiveReservations(ctx context.Context, client *agentmail.Client, projectKey, agentName string, allAgents bool) ([]agentmail.FileReservation, error) {
	// Note: Needs ListReservations API endpoint - currently returns empty
	return []agentmail.FileReservation{}, nil
}

func printLocksResult(result LocksResult, allAgents bool) error {
	if !result.Success && result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}

	scope := "session"
	if allAgents {
		scope = "project"
	}

	if result.Count == 0 {
		fmt.Printf("No active file reservations (%s scope)\n", scope)
		if result.Agent != "" {
			fmt.Printf("   Agent: %s\n", result.Agent)
		}
		fmt.Printf("   Project: %s\n", result.ProjectKey)
		fmt.Println("\nTip: Use 'ntm lock <session> <pattern>' to reserve files")
		return nil
	}

	fmt.Printf("File Reservations: %d active (%s scope)\n", result.Count, scope)
	fmt.Println(strings.Repeat("-", 60))

	for _, r := range result.Reservations {
		lockType := "Exclusive"
		if !r.Exclusive {
			lockType = "Shared"
		}

		remaining := time.Until(r.ExpiresTS)
		expiresStr := formatLockDuration(remaining)

		fmt.Printf("[X] %s\n", r.PathPattern)
		fmt.Printf("   Agent: %s | %s | Expires in %s\n", r.AgentName, lockType, expiresStr)
		if r.Reason != "" {
			fmt.Printf("   Reason: %s\n", r.Reason)
		}
		fmt.Println(strings.Repeat("-", 60))
	}

	return nil
}

func formatLockDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}

	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return fmt.Sprintf("%ds", int(d.Seconds()))
}
