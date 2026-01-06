package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Dicklesworthstone/ntm/internal/cm"
	"github.com/Dicklesworthstone/ntm/internal/output"
)

func newMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Interact with CASS Memory (cm) system",
	}

	cmd.AddCommand(
		newMemoryServeCmd(),
		newMemoryContextCmd(),
		newMemoryOutcomeCmd(),
	)

	return cmd
}

func newMemoryServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start CM HTTP server (manual)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Use 'ntm spawn' to auto-start the memory daemon via supervisor.")
			fmt.Println("To run manually: cm serve --port", port)
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 8200, "Port to listen on")
	return cmd
}

func newMemoryContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "context <task>",
		Short: "Get relevant context for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := args[0]
			
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			
			sessionID, err := findSessionID(dir)
			if err != nil {
				return err
			}
			
			client, err := cm.NewClient(dir, sessionID)
			if err != nil {
				return err
			}
			
			ctxResult, err := client.GetContext(context.Background(), task)
			if err != nil {
				return err
			}
			
			return output.PrintJSON(ctxResult)
		},
	}
}

func newMemoryOutcomeCmd() *cobra.Command {
	var rules []string
	cmd := &cobra.Command{
		Use:   "outcome <success|failure|partial>",
		Short: "Record task outcome feedback",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			statusStr := args[0]
			var status cm.OutcomeStatus
			switch statusStr {
			case "success":
				status = cm.OutcomeSuccess
			case "failure":
				status = cm.OutcomeFailure
			case "partial":
				status = cm.OutcomePartial
			default:
				return fmt.Errorf("invalid status: %s", statusStr)
			}
			
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			
			sessionID, err := findSessionID(dir)
			if err != nil {
				return err
			}
			
			client, err := cm.NewClient(dir, sessionID)
			if err != nil {
				return err
			}
			
			report := cm.OutcomeReport{
				Status:  status,
				RuleIDs: rules,
			}
			
			return client.RecordOutcome(context.Background(), report)
		},
	}
	cmd.Flags().StringSliceVar(&rules, "rules", nil, "Comma-separated list of rule IDs applied")
	return cmd
}

func findSessionID(dir string) (string, error) {
	pidsDir := ".ntm/pids"
	entries, err := os.ReadDir(pidsDir)
	if err != nil {
		return "", fmt.Errorf("could not find .ntm/pids in current directory (run from project root): %w", err)
	}
	
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 3 && name[:3] == "cm-" && name[len(name)-4:] == ".pid" {
			return name[3 : len(name)-4], nil
		}
	}
	return "", fmt.Errorf("no running memory daemon found in current directory")
}
