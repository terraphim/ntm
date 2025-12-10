package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/config"
	"github.com/Dicklesworthstone/ntm/internal/output"
	"github.com/Dicklesworthstone/ntm/internal/robot"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config

	// Global JSON output flag - inherited by all subcommands
	jsonOutput bool

	// Build information - set by goreleaser via ldflags
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "ntm",
	Short: "Named Tmux Manager - orchestrate AI coding agents in tmux sessions",
	Long: `NTM (Named Tmux Manager) helps you create and manage tmux sessions
with multiple AI coding agents (Claude, Codex, Gemini) in separate panes.

Quick Start:
  ntm spawn myproject --cc=2 --cod=2    # Create session with 4 agents
  ntm attach myproject                   # Attach to session
  ntm palette                            # Open command palette (TUI)
  ntm send myproject --all "fix bugs"   # Broadcast prompt to all agents

Shell Integration:
  Add to your .zshrc:  eval "$(ntm init zsh)"
  Add to your .bashrc: eval "$(ntm init bash)"`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Enable profiling if requested
		EnableProfilingIfRequested()

		// Skip config loading for init, completion, version, and upgrade commands
		if cmd.Name() == "init" || cmd.Name() == "completion" || cmd.Name() == "version" || cmd.Name() == "upgrade" {
			return nil
		}

		// Profile config loading
		endProfile := ProfileConfigLoad()
		defer endProfile()

		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			// Use defaults if config doesn't exist
			cfg = config.Default()
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Print profiling output if enabled
		PrintProfilingIfEnabled()
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Handle robot flags for AI agent integration
		if robotHelp {
			robot.PrintHelp()
			return
		}
		if robotStatus {
			if err := robot.PrintStatus(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotVersion {
			if err := robot.PrintVersion(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotPlan {
			if err := robot.PrintPlan(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotSnapshot {
			// Set bead limit from flag
			if robotBeadLimit > 0 {
				robot.BeadLimit = robotBeadLimit
			}
			var err error
			if robotSince != "" {
				// Parse the since timestamp
				sinceTime, parseErr := time.Parse(time.RFC3339, robotSince)
				if parseErr != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid --since timestamp (expected ISO8601/RFC3339 format): %v\n", parseErr)
					os.Exit(1)
				}
				err = robot.PrintSnapshotDelta(sinceTime)
			} else {
				err = robot.PrintSnapshot()
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotGraph {
			if err := robot.PrintGraph(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotTail != "" {
			// Parse pane filter
			var paneFilter []string
			if robotPanes != "" {
				paneFilter = strings.Split(robotPanes, ",")
			}
			if err := robot.PrintTail(robotTail, robotLines, paneFilter); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotSend != "" {
			// Validate message is provided
			if robotSendMsg == "" {
				fmt.Fprintf(os.Stderr, "Error: --msg is required with --robot-send\n")
				os.Exit(1)
			}
			// Parse pane filter
			var paneFilter []string
			if robotPanes != "" {
				paneFilter = strings.Split(robotPanes, ",")
			}
			// Parse exclude list
			var excludeList []string
			if robotSendExclude != "" {
				excludeList = strings.Split(robotSendExclude, ",")
			}
			// Parse agent types
			var agentTypes []string
			if robotSendType != "" {
				agentTypes = strings.Split(robotSendType, ",")
			}

			// Check if --track flag is set for combined send+ack mode
			if robotAckTrack {
				opts := robot.SendAndAckOptions{
					SendOptions: robot.SendOptions{
						Session:    robotSend,
						Message:    robotSendMsg,
						All:        robotSendAll,
						Panes:      paneFilter,
						AgentTypes: agentTypes,
						Exclude:    excludeList,
						DelayMs:    robotSendDelay,
					},
					AckTimeoutMs: robotAckTimeout,
					AckPollMs:    robotAckPoll,
				}
				if err := robot.PrintSendAndAck(opts); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				return
			}

			opts := robot.SendOptions{
				Session:    robotSend,
				Message:    robotSendMsg,
				All:        robotSendAll,
				Panes:      paneFilter,
				AgentTypes: agentTypes,
				Exclude:    excludeList,
				DelayMs:    robotSendDelay,
			}
			if err := robot.PrintSend(opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotHealth {
			if err := robot.PrintHealth(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotAck != "" {
			// Parse pane filter
			var paneFilter []string
			if robotPanes != "" {
				paneFilter = strings.Split(robotPanes, ",")
			}
			opts := robot.AckOptions{
				Session:   robotAck,
				Message:   robotSendMsg, // Reuse --msg flag for echo detection
				Panes:     paneFilter,
				TimeoutMs: robotAckTimeout,
				PollMs:    robotAckPoll,
			}
			if err := robot.PrintAck(opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		// TODO(ntm-20n): --robot-assign is in development
		if robotAssign != "" {
			fmt.Fprintf(os.Stderr, "Error: --robot-assign is not yet implemented\n")
			os.Exit(1)
		}
		if robotSpawn != "" {
			opts := robot.SpawnOptions{
				Session:      robotSpawn,
				CCCount:      robotSpawnCC,
				CodCount:     robotSpawnCod,
				GmiCount:     robotSpawnGmi,
				Preset:       robotSpawnPreset,
				NoUserPane:   robotSpawnNoUser,
				WaitReady:    robotSpawnWait,
				ReadyTimeout: robotSpawnTimeout,
			}
			if err := robot.PrintSpawn(opts, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if robotInterrupt != "" {
			// Parse pane filter (reuse --panes flag)
			var paneFilter []string
			if robotPanes != "" {
				paneFilter = strings.Split(robotPanes, ",")
			}
			opts := robot.InterruptOptions{
				Session:   robotInterrupt,
				Message:   robotInterruptMsg,
				Panes:     paneFilter,
				All:       robotInterruptAll,
				Force:     robotInterruptForce,
				NoWait:    robotInterruptNoWait,
				TimeoutMs: robotInterruptTimeout,
			}
			if err := robot.PrintInterrupt(opts); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		// Show stunning help with gradients when run without subcommand
		PrintStunningHelp()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

// Robot output flags for AI agent integration
var (
	robotHelp     bool
	robotStatus   bool
	robotVersion  bool
	robotPlan     bool
	robotSnapshot bool   // unified state query
	robotSince    string // ISO8601 timestamp for delta snapshot
	robotTail      string // session name for tail
	robotLines     int    // number of lines to capture
	robotPanes     string // comma-separated pane filter
	robotGraph     bool   // bv insights passthrough
	robotBeadLimit int    // limit for ready/in-progress beads in snapshot

	// Robot-send flags
	robotSend        string // session name for send
	robotSendMsg     string // message to send
	robotSendAll     bool   // send to all panes
	robotSendType    string // filter by agent type (e.g., "claude")
	robotSendExclude string // comma-separated panes to exclude
	robotSendDelay   int    // delay between sends in ms

	// Robot-assign flags for work distribution
	robotAssign         string // session name for work assignment
	robotAssignBeads    string // comma-separated bead IDs to assign
	robotAssignStrategy string // assignment strategy: balanced, speed, quality, dependency

	// Robot-health flag
	robotHealth bool // project health summary

	// Robot-recipes flag
	robotRecipes bool // list available recipes as JSON

	// Robot-ack flags for send confirmation tracking
	robotAck        string // session name for ack
	robotAckTimeout int    // timeout in milliseconds
	robotAckPoll    int    // poll interval in milliseconds
	robotAckTrack   bool   // combined send+ack mode

	// Robot-spawn flags for structured session creation
	robotSpawn        string // session name for spawn
	robotSpawnCC      int    // number of Claude agents
	robotSpawnCod     int    // number of Codex agents
	robotSpawnGmi     int    // number of Gemini agents
	robotSpawnPreset  string // recipe/preset name
	robotSpawnNoUser  bool   // don't create user pane
	robotSpawnWait    bool   // wait for agents to be ready
	robotSpawnTimeout int    // timeout for ready detection in seconds

	// Robot-interrupt flags for priority course correction
	robotInterrupt        string // session name for interrupt
	robotInterruptMsg     string // message to send after interrupt
	robotInterruptAll     bool   // include all panes (including user)
	robotInterruptForce   bool   // send Ctrl+C even if agent appears idle
	robotInterruptNoWait  bool   // don't wait for ready state
	robotInterruptTimeout int    // timeout for ready state in milliseconds
)

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/ntm/config.toml)")

	// Global JSON output flag - applies to all commands
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (machine-readable)")

	// Profiling flag for startup timing analysis
	rootCmd.PersistentFlags().BoolVar(&profileStartup, "profile-startup", false, "Enable startup profiling (outputs timing data)")

	// Robot flags for AI agents
	rootCmd.Flags().BoolVar(&robotHelp, "robot-help", false, "Show AI agent help documentation (JSON)")
	rootCmd.Flags().BoolVar(&robotStatus, "robot-status", false, "Output session status as JSON for AI agents")
	rootCmd.Flags().BoolVar(&robotVersion, "robot-version", false, "Output version info as JSON")
	rootCmd.Flags().BoolVar(&robotPlan, "robot-plan", false, "Output execution plan as JSON for AI agents")
	rootCmd.Flags().BoolVar(&robotSnapshot, "robot-snapshot", false, "Output unified system state snapshot (JSON)")
	rootCmd.Flags().StringVar(&robotSince, "since", "", "ISO8601 timestamp for delta snapshot (used with --robot-snapshot)")
	rootCmd.Flags().StringVar(&robotTail, "robot-tail", "", "Tail pane output for session (JSON)")
	rootCmd.Flags().IntVar(&robotLines, "lines", 20, "Number of lines to capture (used with --robot-tail)")
	rootCmd.Flags().StringVar(&robotPanes, "panes", "", "Comma-separated pane indices to filter (used with --robot-tail/--robot-send)")
	rootCmd.Flags().BoolVar(&robotGraph, "robot-graph", false, "Output bv graph insights as JSON for AI agents")
	rootCmd.Flags().IntVar(&robotBeadLimit, "bead-limit", 5, "Limit for ready/in-progress beads in snapshot (used with --robot-snapshot)")

	// Robot-send flags for batch messaging
	rootCmd.Flags().StringVar(&robotSend, "robot-send", "", "Send prompt to panes atomically (JSON output)")
	rootCmd.Flags().StringVar(&robotSendMsg, "msg", "", "Message to send (used with --robot-send)")
	rootCmd.Flags().BoolVar(&robotSendAll, "all", false, "Send to all panes including user (used with --robot-send)")
	rootCmd.Flags().StringVar(&robotSendType, "type", "", "Filter by agent type: claude, codex, gemini (used with --robot-send)")
	rootCmd.Flags().StringVar(&robotSendExclude, "exclude", "", "Comma-separated pane indices to exclude (used with --robot-send)")
	rootCmd.Flags().IntVar(&robotSendDelay, "delay-ms", 0, "Delay between sends in milliseconds (used with --robot-send)")

	// Robot-assign flags for work distribution
	rootCmd.Flags().StringVar(&robotAssign, "robot-assign", "", "Get work distribution recommendations for session (JSON)")
	rootCmd.Flags().StringVar(&robotAssignBeads, "beads", "", "Comma-separated bead IDs to assign (used with --robot-assign)")
	rootCmd.Flags().StringVar(&robotAssignStrategy, "strategy", "balanced", "Assignment strategy: balanced, speed, quality, dependency (used with --robot-assign)")

	// Robot-health flag for project health summary
	rootCmd.Flags().BoolVar(&robotHealth, "robot-health", false, "Output project health summary as JSON for AI agents")

	// Robot-recipes flag for recipe listing
	rootCmd.Flags().BoolVar(&robotRecipes, "robot-recipes", false, "List available recipes as JSON for AI agents")

	// Robot-ack flags for send confirmation tracking
	rootCmd.Flags().StringVar(&robotAck, "robot-ack", "", "Watch panes for acknowledgment after send (JSON output)")
	rootCmd.Flags().IntVar(&robotAckTimeout, "ack-timeout", 30000, "Timeout in milliseconds for acknowledgment (used with --robot-ack)")
	rootCmd.Flags().IntVar(&robotAckPoll, "ack-poll", 500, "Poll interval in milliseconds (used with --robot-ack)")
	rootCmd.Flags().BoolVar(&robotAckTrack, "track", false, "Combined send+ack mode: send message and wait for acknowledgment (used with --robot-send)")

	// Robot-spawn flags for structured session creation
	rootCmd.Flags().StringVar(&robotSpawn, "robot-spawn", "", "Create session and spawn agents (JSON output)")
	rootCmd.Flags().IntVar(&robotSpawnCC, "spawn-cc", 0, "Number of Claude agents (used with --robot-spawn)")
	rootCmd.Flags().IntVar(&robotSpawnCod, "spawn-cod", 0, "Number of Codex agents (used with --robot-spawn)")
	rootCmd.Flags().IntVar(&robotSpawnGmi, "spawn-gmi", 0, "Number of Gemini agents (used with --robot-spawn)")
	rootCmd.Flags().StringVar(&robotSpawnPreset, "spawn-preset", "", "Recipe/preset name (used with --robot-spawn)")
	rootCmd.Flags().BoolVar(&robotSpawnNoUser, "spawn-no-user", false, "Don't create user pane (used with --robot-spawn)")
	rootCmd.Flags().BoolVar(&robotSpawnWait, "spawn-wait", false, "Wait for agents to be ready (used with --robot-spawn)")
	rootCmd.Flags().IntVar(&robotSpawnTimeout, "spawn-timeout", 30, "Timeout in seconds for ready detection (used with --robot-spawn)")

	// Robot-interrupt flags for priority course correction
	rootCmd.Flags().StringVar(&robotInterrupt, "robot-interrupt", "", "Interrupt agents with Ctrl+C and optionally send message (JSON output)")
	rootCmd.Flags().StringVar(&robotInterruptMsg, "interrupt-msg", "", "Message to send after interrupt (used with --robot-interrupt)")
	rootCmd.Flags().BoolVar(&robotInterruptAll, "interrupt-all", false, "Include all panes including user (used with --robot-interrupt)")
	rootCmd.Flags().BoolVar(&robotInterruptForce, "interrupt-force", false, "Send Ctrl+C even if agent appears idle (used with --robot-interrupt)")
	rootCmd.Flags().BoolVar(&robotInterruptNoWait, "interrupt-no-wait", false, "Don't wait for ready state (used with --robot-interrupt)")
	rootCmd.Flags().IntVar(&robotInterruptTimeout, "interrupt-timeout", 10000, "Timeout in ms for ready state (used with --robot-interrupt)")

	// Sync version info with robot package
	robot.Version = Version
	robot.Commit = Commit
	robot.Date = Date
	robot.BuiltBy = BuiltBy

	// Add all subcommands
	rootCmd.AddCommand(
		// Session creation
		newCreateCmd(),
		newSpawnCmd(),
		newQuickCmd(),

		// Agent management
		newAddCmd(),
		newSendCmd(),
		newReplayCmd(),
		newInterruptCmd(),
		newMailCmd(),

		// Session navigation
		newAttachCmd(),
		newListCmd(),
		newStatusCmd(),
		newViewCmd(),
		newZoomCmd(),
		newDashboardCmd(),
		newWatchCmd(),

		// Output management
		newCopyCmd(),
		newSaveCmd(),
		newGrepCmd(),

		// Session persistence
		newCheckpointCmd(),
		newRollbackCmd(),
		newSessionPersistCmd(),

		// Utilities
		newPaletteCmd(),
		newBindCmd(),
		newDepsCmd(),
		newKillCmd(),
		newScanCmd(),
		newHooksCmd(),
		newHealthCmd(),
		newHistoryCmd(),

		// Shell integration
		newInitCmd(),
		newCompletionCmd(),
		newVersionCmd(),
		newConfigCmd(),
		newUpgradeCmd(),

		// Tutorial
		newTutorialCmd(),

		// Agent Mail & File Reservations
		newLockCmd(),
		newUnlockCmd(),
		newLocksCmd(),

		// Git coordination
		newGitCmd(),

		// Configuration management
		newRecipesCmd(),
		newPersonasCmd(),
		newTemplateCmd(),
	)
}

func newVersionCmd() *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			if short {
				fmt.Println(Version)
				return
			}
			fmt.Printf("ntm version %s\n", Version)
			fmt.Printf("  commit:  %s\n", Commit)
			fmt.Printf("  built:   %s\n", Date)
			fmt.Printf("  builder: %s\n", BuiltBy)
		},
	}
	cmd.Flags().BoolVarP(&short, "short", "s", false, "Print only version number")
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.CreateDefault()
			if err != nil {
				return err
			}
			fmt.Printf("Created config file: %s\n", path)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print configuration file path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.DefaultPath())
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("")
			if err != nil {
				cfg = config.Default()
				fmt.Println("# Using default configuration (no config file found)")
				fmt.Println()
			}
			return config.Print(cfg, os.Stdout)
		},
	})

	return cmd
}

// IsJSONOutput returns true if JSON output is enabled
func IsJSONOutput() bool {
	return jsonOutput
}

// GetOutputFormat returns the current output format
func GetOutputFormat() output.Format {
	return output.DetectFormat(jsonOutput)
}

// GetFormatter returns a formatter configured for the current output mode
func GetFormatter() *output.Formatter {
	return output.New(output.WithJSON(jsonOutput))
}

