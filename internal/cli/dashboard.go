package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Dicklesworthstone/ntm/internal/agentmail"
	"github.com/Dicklesworthstone/ntm/internal/config"
	"github.com/Dicklesworthstone/ntm/internal/tmux"
	"github.com/Dicklesworthstone/ntm/internal/tui/dashboard"
	"github.com/Dicklesworthstone/ntm/internal/watcher"
)

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "dashboard [session-name]",
		Aliases: []string{"dash", "d"},
		Short:   "Open interactive session dashboard",
		Long: `Open a stunning interactive dashboard for a tmux session.

The dashboard shows:
- Visual grid of all panes with agent types
- Agent counts (Claude, Codex, Gemini)
- Quick actions for zooming and sending commands

If no session is specified:
- Inside tmux: uses the current session
- Outside tmux: shows a session selector

Examples:
  ntm dashboard myproject
  ntm dash                  # Auto-detect session`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var session string
			if len(args) > 0 {
				session = args[0]
			}
			return runDashboard(cmd.OutOrStdout(), cmd.ErrOrStderr(), session)
		},
	}
}

func runDashboard(w io.Writer, errW io.Writer, session string) error {
	if err := tmux.EnsureInstalled(); err != nil {
		return err
	}

	res, err := ResolveSession(session, w)
	if err != nil {
		return err
	}
	if res.Session == "" {
		return nil
	}
	res.ExplainIfInferred(errW)
	session = res.Session

	if !tmux.SessionExists(session) {
		return fmt.Errorf("session '%s' not found", session)
	}

	projectDir := ""
	if cfg != nil {
		projectDir = cfg.GetProjectDir(session)
	} else {
		// Fallback to default if config not loaded
		projectDir = config.Default().GetProjectDir(session)
	}

	// Validate project directory exists, warn if not
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		fmt.Fprintf(errW, "Warning: project directory does not exist: %s\n", projectDir)
		fmt.Fprintf(errW, "Some features (beads, file tracking) may not work correctly.\n")
		fmt.Fprintf(errW, "Check your projects_base setting in config: ntm config show\n\n")
	}

	// Start FileReservationWatcher if enabled and Agent Mail is available
	var reservationWatcher *watcher.FileReservationWatcher
	if cfg != nil && cfg.FileReservation.Enabled && cfg.AgentMail.Enabled {
		// Create Agent Mail client with config options
		amOpts := []agentmail.Option{
			agentmail.WithBaseURL(cfg.AgentMail.URL),
			agentmail.WithProjectKey(projectDir),
		}
		if cfg.AgentMail.Token != "" {
			amOpts = append(amOpts, agentmail.WithToken(cfg.AgentMail.Token))
		}
		amClient := agentmail.NewClient(amOpts...)

		// Check if Agent Mail is reachable
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if _, err := amClient.HealthCheck(ctx); err == nil {
			// Convert config to watcher config values
			cfgValues := watcher.FileReservationConfigValues{
				Enabled:               cfg.FileReservation.Enabled,
				AutoReserve:           cfg.FileReservation.AutoReserve,
				AutoReleaseIdleMin:    cfg.FileReservation.AutoReleaseIdleMin,
				NotifyOnConflict:      cfg.FileReservation.NotifyOnConflict,
				ExtendOnActivity:      cfg.FileReservation.ExtendOnActivity,
				DefaultTTLMin:         cfg.FileReservation.DefaultTTLMin,
				PollIntervalSec:       cfg.FileReservation.PollIntervalSec,
				CaptureLinesForDetect: cfg.FileReservation.CaptureLinesForDetect,
				Debug:                 cfg.FileReservation.Debug,
			}

			// Create conflict callback for notifications
			conflictCallback := func(conflict watcher.FileConflict) {
				if cfg.FileReservation.Debug {
					log.Printf("[FileReservation] Conflict: %s requested by %s, held by %v",
						conflict.Path, conflict.RequestorAgent, conflict.Holders)
				}
				// TODO: Integrate with dashboard notification system
			}

			reservationWatcher = watcher.NewFileReservationWatcherFromConfig(
				cfgValues,
				amClient,
				projectDir,
				session, // Use session name as agent name
				conflictCallback,
			)

			if reservationWatcher != nil {
				reservationWatcher.Start(context.Background())
				defer reservationWatcher.Stop()
				if cfg.FileReservation.Debug {
					log.Printf("[FileReservation] Watcher started for session %s", session)
				}
			}
		}
		cancel()
	}

	return dashboard.Run(session, projectDir)
}
