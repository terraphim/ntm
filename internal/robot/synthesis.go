// Package robot provides machine-readable output for AI agents and automation.
// synthesis.go implements file conflict detection across multiple agents.
package robot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/agentmail"
)

// ConflictReason describes why a file conflict was detected.
type ConflictReason string

const (
	// ReasonConcurrentActivity indicates multiple panes had activity while file was modified.
	ReasonConcurrentActivity ConflictReason = "concurrent_activity"
	// ReasonReservationViolation indicates a file was modified without holding a reservation.
	ReasonReservationViolation ConflictReason = "reservation_violation"
	// ReasonOverlappingReservations indicates multiple agents have reservations for same file.
	ReasonOverlappingReservations ConflictReason = "overlapping_reservations"
	// ReasonUnclaimedModification indicates a modified file with no known modifier.
	ReasonUnclaimedModification ConflictReason = "unclaimed_modification"
)

// DetectedConflict represents a detected or potential file conflict from synthesis analysis.
// This extends the simpler FileConflict in tui_parity.go with more detailed conflict analysis.
type DetectedConflict struct {
	// Path is the file path relative to the repository root.
	Path string `json:"path"`

	// LikelyModifiers are pane IDs that may have modified this file.
	LikelyModifiers []string `json:"likely_modifiers"`

	// GitStatus is the git status code (M=modified, A=added, D=deleted, ??=untracked).
	GitStatus string `json:"git_status"`

	// Confidence is a score from 0.0-1.0 indicating conflict likelihood.
	// 0.9+ = high, 0.7-0.9 = medium, 0.5-0.7 = low
	Confidence float64 `json:"confidence"`

	// Reason explains why this conflict was detected.
	Reason ConflictReason `json:"reason"`

	// ReservationHolders are agents with active reservations for this file.
	ReservationHolders []string `json:"reservation_holders,omitempty"`

	// ModifiedAt is when the file was last modified (from filesystem).
	ModifiedAt time.Time `json:"modified_at,omitempty"`

	// Details provides additional context for the conflict.
	Details string `json:"details,omitempty"`
}

// ConflictConfidence categorizes confidence levels.
type ConflictConfidence string

const (
	// ConfidenceHigh indicates strong evidence of conflict (0.9+).
	ConfidenceHigh ConflictConfidence = "high"
	// ConfidenceMedium indicates moderate evidence (0.7-0.9).
	ConfidenceMedium ConflictConfidence = "medium"
	// ConfidenceLow indicates weak evidence (0.5-0.7).
	ConfidenceLow ConflictConfidence = "low"
	// ConfidenceNone indicates no significant conflict evidence (<0.5).
	ConfidenceNone ConflictConfidence = "none"
)

// ConfidenceLevel returns the categorical confidence level.
func (dc *DetectedConflict) ConfidenceLevel() ConflictConfidence {
	switch {
	case dc.Confidence >= 0.9:
		return ConfidenceHigh
	case dc.Confidence >= 0.7:
		return ConfidenceMedium
	case dc.Confidence >= 0.5:
		return ConfidenceLow
	default:
		return ConfidenceNone
	}
}

// ActivityWindow represents a time window of agent activity.
type ActivityWindow struct {
	PaneID    string    `json:"pane_id"`
	AgentType string    `json:"agent_type"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	HasOutput bool      `json:"has_output"` // Whether output was detected during window
}

// Overlaps returns true if this window overlaps with another.
func (aw *ActivityWindow) Overlaps(other *ActivityWindow) bool {
	return aw.Start.Before(other.End) && other.Start.Before(aw.End)
}

// Contains returns true if the given time falls within this window.
func (aw *ActivityWindow) Contains(t time.Time) bool {
	return !t.Before(aw.Start) && !t.After(aw.End)
}

// GitFileStatus represents a file's status from git.
type GitFileStatus struct {
	Path       string    `json:"path"`
	Status     string    `json:"status"` // M, A, D, ??, etc.
	Staged     bool      `json:"staged"`
	ModifiedAt time.Time `json:"modified_at,omitempty"`
}

// ConflictDetector detects potential file conflicts across agents.
type ConflictDetector struct {
	repoPath        string
	activityWindows map[string][]ActivityWindow // paneID -> windows
	amClient        *agentmail.Client
	projectKey      string

	mu sync.RWMutex
}

// ConflictDetectorConfig holds configuration for conflict detection.
type ConflictDetectorConfig struct {
	RepoPath   string
	ProjectKey string
	AMClient   *agentmail.Client
}

// NewConflictDetector creates a new conflict detector.
func NewConflictDetector(cfg *ConflictDetectorConfig) *ConflictDetector {
	if cfg == nil {
		cfg = &ConflictDetectorConfig{}
	}

	repoPath := cfg.RepoPath
	if repoPath == "" {
		repoPath, _ = os.Getwd()
	}

	return &ConflictDetector{
		repoPath:        repoPath,
		activityWindows: make(map[string][]ActivityWindow),
		amClient:        cfg.AMClient,
		projectKey:      cfg.ProjectKey,
	}
}

// RecordActivity records an activity window for a pane.
func (cd *ConflictDetector) RecordActivity(paneID, agentType string, start, end time.Time, hasOutput bool) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	window := ActivityWindow{
		PaneID:    paneID,
		AgentType: agentType,
		Start:     start,
		End:       end,
		HasOutput: hasOutput,
	}

	cd.activityWindows[paneID] = append(cd.activityWindows[paneID], window)

	// Keep only windows from the last hour to prevent unbounded growth
	cutoff := time.Now().Add(-1 * time.Hour)
	cd.pruneWindowsLocked(cutoff)
}

// pruneWindowsLocked removes activity windows older than cutoff.
// Must be called with mu held.
func (cd *ConflictDetector) pruneWindowsLocked(cutoff time.Time) {
	for paneID, windows := range cd.activityWindows {
		var kept []ActivityWindow
		for _, w := range windows {
			if w.End.After(cutoff) {
				kept = append(kept, w)
			}
		}
		if len(kept) > 0 {
			cd.activityWindows[paneID] = kept
		} else {
			delete(cd.activityWindows, paneID)
		}
	}
}

// GetGitStatus returns the current git status of modified files.
func (cd *ConflictDetector) GetGitStatus() ([]GitFileStatus, error) {
	cmd := exec.Command("git", "-C", cd.repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseGitStatusPorcelain(string(output), cd.repoPath)
}

// parseGitStatusPorcelain parses `git status --porcelain` output.
func parseGitStatusPorcelain(output, repoPath string) ([]GitFileStatus, error) {
	var results []GitFileStatus

	// Don't TrimSpace the whole output - it would remove leading spaces from status codes
	// like " M file.go" where space means "not staged"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r") // Handle CRLF
		if len(line) < 3 {
			continue
		}

		// Format: XY path
		// X = index status, Y = work tree status
		xy := line[:2]
		path := strings.TrimSpace(line[3:])

		// Handle renamed files (path contains " -> ")
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}

		status := GitFileStatus{
			Path:   path,
			Status: strings.TrimSpace(xy),
			Staged: xy[0] != ' ' && xy[0] != '?',
		}

		// Get file modification time
		fullPath := filepath.Join(repoPath, path)
		if info, err := os.Stat(fullPath); err == nil {
			status.ModifiedAt = info.ModTime()
		}

		results = append(results, status)
	}

	return results, nil
}

// DetectConflicts analyzes git status and activity windows to detect conflicts.
func (cd *ConflictDetector) DetectConflicts(ctx context.Context) ([]DetectedConflict, error) {
	// Get current git status
	gitStatus, err := cd.GetGitStatus()
	if err != nil {
		return nil, err
	}

	if len(gitStatus) == 0 {
		return nil, nil // No modified files
	}

	// Get file reservations from Agent Mail if available
	var reservations []agentmail.FileReservation
	if cd.amClient != nil && cd.projectKey != "" {
		// List all reservations (not filtered by agent)
		reservations, _ = cd.amClient.ListReservations(ctx, cd.projectKey, "", true)
	}

	cd.mu.RLock()
	defer cd.mu.RUnlock()

	var conflicts []DetectedConflict

	for _, file := range gitStatus {
		conflict := cd.analyzeFileConflict(file, reservations)
		if conflict != nil && conflict.Confidence >= 0.5 {
			conflicts = append(conflicts, *conflict)
		}
	}

	return conflicts, nil
}

// analyzeFileConflict analyzes a single file for conflicts.
func (cd *ConflictDetector) analyzeFileConflict(file GitFileStatus, reservations []agentmail.FileReservation) *DetectedConflict {
	conflict := &DetectedConflict{
		Path:       file.Path,
		GitStatus:  file.Status,
		ModifiedAt: file.ModifiedAt,
		Confidence: 0.0,
	}

	// Find reservation holders for this file
	holders := cd.findReservationHolders(file.Path, reservations)
	conflict.ReservationHolders = holders

	// Find panes with activity during file modification window
	modifiers := cd.findLikelyModifiers(file)
	conflict.LikelyModifiers = modifiers

	// Score the conflict
	cd.scoreConflict(conflict, len(modifiers), len(holders))

	return conflict
}

// findReservationHolders returns agents with reservations matching the file path.
func (cd *ConflictDetector) findReservationHolders(filePath string, reservations []agentmail.FileReservation) []string {
	var holders []string
	seen := make(map[string]bool)

	for _, r := range reservations {
		// Skip released reservations
		if r.ReleasedTS != nil {
			continue
		}
		// Skip expired reservations
		if r.ExpiresTS.Before(time.Now()) {
			continue
		}

		if matchesPattern(filePath, r.PathPattern) && !seen[r.AgentName] {
			holders = append(holders, r.AgentName)
			seen[r.AgentName] = true
		}
	}

	return holders
}

// matchesPattern checks if a file path matches a glob pattern.
// Supports:
// - Exact match: "src/main.go"
// - Prefix match: "src/" matches "src/main.go"
// - Single * wildcard: "src/*.go" matches "src/main.go"
// - Double ** wildcard: "src/**" matches any path under src/
// - Combined: "src/**/test.go" matches "src/foo/bar/test.go"
func matchesPattern(filePath, pattern string) bool {
	// Exact match
	if filePath == pattern {
		return true
	}

	// Handle ** patterns (match any number of path segments)
	if strings.Contains(pattern, "**") {
		parts := strings.SplitN(pattern, "**", 2)
		prefix := parts[0]
		suffix := ""
		if len(parts) > 1 {
			suffix = strings.TrimPrefix(parts[1], "/")
		}

		// Path must start with prefix
		if !strings.HasPrefix(filePath, prefix) {
			return false
		}

		// If no suffix, just prefix match is enough
		if suffix == "" {
			return true
		}

		// Path must end with suffix (after stripping prefix)
		remaining := strings.TrimPrefix(filePath, prefix)
		return strings.HasSuffix(remaining, suffix)
	}

	// Handle single * patterns (match single path segment)
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")

		// Must start with first part and end with last part
		if !strings.HasPrefix(filePath, parts[0]) {
			return false
		}
		if !strings.HasSuffix(filePath, parts[len(parts)-1]) {
			return false
		}

		// For multiple wildcards, check that all parts appear in order
		remaining := filePath
		for _, part := range parts {
			if part == "" {
				continue
			}
			idx := strings.Index(remaining, part)
			if idx == -1 {
				return false
			}
			remaining = remaining[idx+len(part):]
		}
		return true
	}

	// Prefix match (pattern is a directory)
	return strings.HasPrefix(filePath, pattern+"/")
}

// findLikelyModifiers returns pane IDs with activity around the file modification time.
func (cd *ConflictDetector) findLikelyModifiers(file GitFileStatus) []string {
	if file.ModifiedAt.IsZero() {
		return nil
	}

	var modifiers []string
	seen := make(map[string]bool)

	// Look for activity windows that contain the file modification time
	// Use a tolerance window of 60 seconds before and after
	tolerance := 60 * time.Second
	checkStart := file.ModifiedAt.Add(-tolerance)
	checkEnd := file.ModifiedAt.Add(tolerance)

	for paneID, windows := range cd.activityWindows {
		for _, w := range windows {
			// Check if window overlaps with modification time window
			if w.Start.Before(checkEnd) && w.End.After(checkStart) {
				if !seen[paneID] {
					modifiers = append(modifiers, paneID)
					seen[paneID] = true
				}
				break
			}
		}
	}

	return modifiers
}

// scoreConflict calculates the conflict confidence score.
func (cd *ConflictDetector) scoreConflict(conflict *DetectedConflict, modifierCount, holderCount int) {
	// Base confidence based on situation
	switch {
	case modifierCount > 1:
		// Multiple modifiers - high confidence of conflict
		conflict.Confidence = 0.9
		conflict.Reason = ReasonConcurrentActivity
		conflict.Details = "Multiple agents had activity when this file was modified"

	case modifierCount == 1 && holderCount > 0:
		// Single modifier with reservation holders
		if !containsAny(conflict.LikelyModifiers, conflict.ReservationHolders) {
			// Modifier doesn't hold the reservation
			conflict.Confidence = 0.85
			conflict.Reason = ReasonReservationViolation
			conflict.Details = "File modified by agent without active reservation"
		} else {
			// Modifier holds reservation - likely OK
			conflict.Confidence = 0.3
			conflict.Reason = ReasonConcurrentActivity
			conflict.Details = "File modified by reservation holder"
		}

	case modifierCount == 0 && holderCount > 1:
		// No detected modifier but multiple reservation holders
		conflict.Confidence = 0.75
		conflict.Reason = ReasonOverlappingReservations
		conflict.Details = "Multiple agents have reservations for this file"

	case modifierCount == 0 && holderCount == 0:
		// Unknown modifier, no reservations
		conflict.Confidence = 0.6
		conflict.Reason = ReasonUnclaimedModification
		conflict.Details = "File modified with no tracked activity or reservations"

	case modifierCount == 1 && holderCount == 0:
		// Single modifier, no reservations (normal case)
		conflict.Confidence = 0.4
		conflict.Reason = ReasonConcurrentActivity
		conflict.Details = "File modified by single agent without reservation"

	default:
		conflict.Confidence = 0.5
		conflict.Reason = ReasonUnclaimedModification
	}
}

// containsAny returns true if any element of a is in b.
func containsAny(a, b []string) bool {
	bSet := make(map[string]bool, len(b))
	for _, s := range b {
		bSet[s] = true
	}
	for _, s := range a {
		if bSet[s] {
			return true
		}
	}
	return false
}

// GetActivityWindows returns all tracked activity windows.
func (cd *ConflictDetector) GetActivityWindows() map[string][]ActivityWindow {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	// Return a copy
	result := make(map[string][]ActivityWindow, len(cd.activityWindows))
	for paneID, windows := range cd.activityWindows {
		windowsCopy := make([]ActivityWindow, len(windows))
		copy(windowsCopy, windows)
		result[paneID] = windowsCopy
	}
	return result
}

// ClearActivityWindows removes all tracked activity windows.
func (cd *ConflictDetector) ClearActivityWindows() {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	cd.activityWindows = make(map[string][]ActivityWindow)
}

// ConflictSummary provides a summary of detected conflicts.
type ConflictSummary struct {
	TotalConflicts int                `json:"total_conflicts"`
	HighConfidence int                `json:"high_confidence"` // 0.9+
	MedConfidence  int                `json:"med_confidence"`  // 0.7-0.9
	LowConfidence  int                `json:"low_confidence"`  // 0.5-0.7
	ByReason       map[string]int     `json:"by_reason"`
	Conflicts      []DetectedConflict `json:"conflicts"`
	Timestamp      string             `json:"timestamp"`
}

// SummarizeConflicts generates a summary from a list of conflicts.
func SummarizeConflicts(conflicts []DetectedConflict) *ConflictSummary {
	summary := &ConflictSummary{
		TotalConflicts: len(conflicts),
		ByReason:       make(map[string]int),
		Conflicts:      conflicts,
		Timestamp:      FormatTimestamp(time.Now()),
	}

	for _, c := range conflicts {
		switch c.ConfidenceLevel() {
		case ConfidenceHigh:
			summary.HighConfidence++
		case ConfidenceMedium:
			summary.MedConfidence++
		case ConfidenceLow:
			summary.LowConfidence++
		}
		summary.ByReason[string(c.Reason)]++
	}

	return summary
}

// ConflictDetectionResponse is the robot command response for conflict detection.
type ConflictDetectionResponse struct {
	RobotResponse
	Summary *ConflictSummary `json:"summary,omitempty"`
}

// NewConflictDetectionResponse creates a new conflict detection response.
func NewConflictDetectionResponse(conflicts []DetectedConflict) *ConflictDetectionResponse {
	resp := &ConflictDetectionResponse{
		RobotResponse: NewRobotResponse(true),
	}
	if len(conflicts) > 0 {
		resp.Summary = SummarizeConflicts(conflicts)
	}
	return resp
}
