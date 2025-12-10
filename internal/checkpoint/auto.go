package checkpoint

import (
	"fmt"
	"strings"
	"time"
)

const (
	// AutoCheckpointPrefix is the prefix for auto-generated checkpoint names
	AutoCheckpointPrefix = "auto"
)

// AutoCheckpointReason describes why an auto-checkpoint was triggered
type AutoCheckpointReason string

const (
	ReasonBroadcast  AutoCheckpointReason = "broadcast"   // Before sending to all agents
	ReasonAddAgents  AutoCheckpointReason = "add_agents"  // Before adding many agents
	ReasonSpawn      AutoCheckpointReason = "spawn"       // After spawning session
	ReasonRiskyOp    AutoCheckpointReason = "risky_op"    // Before other risky operation
)

// AutoCheckpointOptions configures auto-checkpoint creation
type AutoCheckpointOptions struct {
	SessionName     string
	Reason          AutoCheckpointReason
	Description     string // Additional context
	ScrollbackLines int
	IncludeGit      bool
	MaxCheckpoints  int // Max auto-checkpoints to keep (rotation)
}

// AutoCheckpointer handles automatic checkpoint creation with rotation
type AutoCheckpointer struct {
	capturer *Capturer
	storage  *Storage
}

// NewAutoCheckpointer creates a new auto-checkpointer
func NewAutoCheckpointer() *AutoCheckpointer {
	return &AutoCheckpointer{
		capturer: NewCapturer(),
		storage:  NewStorage(),
	}
}

// Create creates an auto-checkpoint with the given options
// It returns the created checkpoint and any error encountered
func (a *AutoCheckpointer) Create(opts AutoCheckpointOptions) (*Checkpoint, error) {
	// Build checkpoint name from reason
	name := fmt.Sprintf("%s-%s", AutoCheckpointPrefix, opts.Reason)

	// Build description
	desc := fmt.Sprintf("Auto-checkpoint: %s", opts.Reason)
	if opts.Description != "" {
		desc = fmt.Sprintf("%s (%s)", desc, opts.Description)
	}

	// Build checkpoint options
	cpOpts := []CheckpointOption{
		WithDescription(desc),
		WithGitCapture(opts.IncludeGit),
	}
	if opts.ScrollbackLines > 0 {
		cpOpts = append(cpOpts, WithScrollbackLines(opts.ScrollbackLines))
	}

	// Create the checkpoint
	cp, err := a.capturer.Create(opts.SessionName, name, cpOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating auto-checkpoint: %w", err)
	}

	// Apply rotation policy
	if opts.MaxCheckpoints > 0 {
		if err := a.rotateAutoCheckpoints(opts.SessionName, opts.MaxCheckpoints); err != nil {
			// Log but don't fail - checkpoint was created successfully
			fmt.Printf("Warning: failed to rotate auto-checkpoints: %v\n", err)
		}
	}

	return cp, nil
}

// rotateAutoCheckpoints ensures we don't exceed the max auto-checkpoints
// by deleting the oldest auto-checkpoints
func (a *AutoCheckpointer) rotateAutoCheckpoints(sessionName string, maxCount int) error {
	// List all checkpoints for the session
	checkpoints, err := a.storage.List(sessionName)
	if err != nil {
		return err
	}

	// Filter to auto-checkpoints only
	var autoCheckpoints []*Checkpoint
	for _, cp := range checkpoints {
		if isAutoCheckpoint(cp) {
			autoCheckpoints = append(autoCheckpoints, cp)
		}
	}

	// If under limit, nothing to do
	if len(autoCheckpoints) <= maxCount {
		return nil
	}

	// Delete oldest auto-checkpoints (list is sorted newest first)
	toDelete := autoCheckpoints[maxCount:]
	for _, cp := range toDelete {
		if err := a.storage.Delete(sessionName, cp.ID); err != nil {
			// Log but continue
			fmt.Printf("Warning: failed to delete old auto-checkpoint %s: %v\n", cp.ID, err)
		}
	}

	return nil
}

// isAutoCheckpoint checks if a checkpoint was auto-generated
func isAutoCheckpoint(cp *Checkpoint) bool {
	// Check by name prefix
	if strings.HasPrefix(cp.Name, AutoCheckpointPrefix) {
		return true
	}
	// Also check description as fallback
	if strings.Contains(cp.Description, "Auto-checkpoint:") {
		return true
	}
	return false
}

// GetLastAutoCheckpoint returns the most recent auto-checkpoint for a session
func (a *AutoCheckpointer) GetLastAutoCheckpoint(sessionName string) (*Checkpoint, error) {
	checkpoints, err := a.storage.List(sessionName)
	if err != nil {
		return nil, err
	}

	for _, cp := range checkpoints {
		if isAutoCheckpoint(cp) {
			return cp, nil
		}
	}

	return nil, fmt.Errorf("no auto-checkpoints found for session: %s", sessionName)
}

// ListAutoCheckpoints returns all auto-checkpoints for a session
func (a *AutoCheckpointer) ListAutoCheckpoints(sessionName string) ([]*Checkpoint, error) {
	checkpoints, err := a.storage.List(sessionName)
	if err != nil {
		return nil, err
	}

	var autoCheckpoints []*Checkpoint
	for _, cp := range checkpoints {
		if isAutoCheckpoint(cp) {
			autoCheckpoints = append(autoCheckpoints, cp)
		}
	}

	return autoCheckpoints, nil
}

// TimeSinceLastAutoCheckpoint returns the duration since the last auto-checkpoint
// Returns 0 if no auto-checkpoint exists
func (a *AutoCheckpointer) TimeSinceLastAutoCheckpoint(sessionName string) time.Duration {
	cp, err := a.GetLastAutoCheckpoint(sessionName)
	if err != nil {
		return 0
	}
	return time.Since(cp.CreatedAt)
}
