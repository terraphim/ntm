package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Dicklesworthstone/ntm/internal/tmux"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var ccCount, codCount, gmiCount int

	cmd := &cobra.Command{
		Use:   "add <session-name>",
		Short: "Add more agents to an existing session",
		Long: `Add additional AI agents to an existing tmux session.

Examples:
  ntm add myproject --cc=2           # Add 2 Claude agents
  ntm add myproject --cod=1 --gmi=1  # Add 1 Codex, 1 Gemini`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(args[0], ccCount, codCount, gmiCount)
		},
	}

	cmd.Flags().IntVar(&ccCount, "cc", 0, "number of Claude agents to add")
	cmd.Flags().IntVar(&codCount, "cod", 0, "number of Codex agents to add")
	cmd.Flags().IntVar(&gmiCount, "gmi", 0, "number of Gemini agents to add")

	return cmd
}

func runAdd(session string, ccCount, codCount, gmiCount int) error {
	if err := tmux.EnsureInstalled(); err != nil {
		return err
	}

	if !tmux.SessionExists(session) {
		return fmt.Errorf("session '%s' does not exist (use 'ntm spawn' to create)", session)
	}

	totalAgents := ccCount + codCount + gmiCount
	if totalAgents == 0 {
		return fmt.Errorf("no agents specified (use --cc, --cod, or --gmi)")
	}

	dir := cfg.GetProjectDir(session)
	fmt.Printf("Adding %d agent(s) to session '%s'...\n", totalAgents, session)

	// Get existing panes to determine next indices
	panes, err := tmux.GetPanes(session)
	if err != nil {
		return err
	}

	maxCC, maxCod, maxGmi := 0, 0, 0

	// Helper to parse index from title (e.g., "myproject__cc_2" -> 2)
	parseIndex := func(title, suffix string) int {
		if idx := strings.LastIndex(title, suffix); idx != -1 {
			numPart := title[idx+len(suffix):]
			if val, err := strconv.Atoi(numPart); err == nil {
				return val
			}
		}
		return 0
	}

	for _, p := range panes {
		if idx := parseIndex(p.Title, "__cc_"); idx > maxCC {
			maxCC = idx
		}
		if idx := parseIndex(p.Title, "__cod_"); idx > maxCod {
			maxCod = idx
		}
		if idx := parseIndex(p.Title, "__gmi_"); idx > maxGmi {
			maxGmi = idx
		}
	}

	// Add Claude agents
	for i := 0; i < ccCount; i++ {
		paneID, err := tmux.SplitWindow(session, dir)
		if err != nil {
			return fmt.Errorf("creating pane: %w", err)
		}

		num := maxCC + i + 1
		title := fmt.Sprintf("%s__cc_%d", session, num)
		if err := tmux.SetPaneTitle(paneID, title); err != nil {
			return fmt.Errorf("setting pane title: %w", err)
		}

		cmd := fmt.Sprintf("cd %q && %s", dir, cfg.Agents.Claude)
		if err := tmux.SendKeys(paneID, cmd, true); err != nil {
			return fmt.Errorf("launching agent: %w", err)
		}
	}

	// Add Codex agents
	for i := 0; i < codCount; i++ {
		paneID, err := tmux.SplitWindow(session, dir)
		if err != nil {
			return fmt.Errorf("creating pane: %w", err)
		}

		num := maxCod + i + 1
		title := fmt.Sprintf("%s__cod_%d", session, num)
		if err := tmux.SetPaneTitle(paneID, title); err != nil {
			return fmt.Errorf("setting pane title: %w", err)
		}

		cmd := fmt.Sprintf("cd %q && %s", dir, cfg.Agents.Codex)
		if err := tmux.SendKeys(paneID, cmd, true); err != nil {
			return fmt.Errorf("launching agent: %w", err)
		}
	}

	// Add Gemini agents
	for i := 0; i < gmiCount; i++ {
		paneID, err := tmux.SplitWindow(session, dir)
		if err != nil {
			return fmt.Errorf("creating pane: %w", err)
		}

		num := maxGmi + i + 1
		title := fmt.Sprintf("%s__gmi_%d", session, num)
		if err := tmux.SetPaneTitle(paneID, title); err != nil {
			return fmt.Errorf("setting pane title: %w", err)
		}

		cmd := fmt.Sprintf("cd %q && %s", dir, cfg.Agents.Gemini)
		if err := tmux.SendKeys(paneID, cmd, true); err != nil {
			return fmt.Errorf("launching agent: %w", err)
		}
	}

	fmt.Printf("✓ Added %dx cc, %dx cod, %dx gmi\n", ccCount, codCount, gmiCount)
	return nil
}