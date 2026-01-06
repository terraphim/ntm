package coordinator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/agentmail"
	"github.com/Dicklesworthstone/ntm/internal/bv"
)

// WorkAssignment represents a work assignment to an agent.
type WorkAssignment struct {
	BeadID        string    `json:"bead_id"`
	BeadTitle     string    `json:"bead_title"`
	AgentPaneID   string    `json:"agent_pane_id"`
	AgentMailName string    `json:"agent_mail_name,omitempty"`
	AgentType     string    `json:"agent_type"`
	AssignedAt    time.Time `json:"assigned_at"`
	Priority      int       `json:"priority"`
	Score         float64   `json:"score"`
	FilesToReserve []string `json:"files_to_reserve,omitempty"`
}

// AssignmentResult contains the result of an assignment attempt.
type AssignmentResult struct {
	Success      bool            `json:"success"`
	Assignment   *WorkAssignment `json:"assignment,omitempty"`
	Error        string          `json:"error,omitempty"`
	Reservations []string        `json:"reservations,omitempty"`
	MessageSent  bool            `json:"message_sent"`
}

// AssignWork assigns work to idle agents based on bv triage.
func (c *SessionCoordinator) AssignWork(ctx context.Context) ([]AssignmentResult, error) {
	if !c.config.AutoAssign {
		return nil, nil
	}

	// Get idle agents
	idleAgents := c.GetIdleAgents()
	if len(idleAgents) == 0 {
		return nil, nil
	}

	// Get triage recommendations
	triage, err := bv.GetTriage(c.projectKey)
	if err != nil {
		return nil, fmt.Errorf("getting triage: %w", err)
	}

	if triage == nil || len(triage.Triage.Recommendations) == 0 {
		return nil, nil
	}

	var results []AssignmentResult

	// Match agents to recommendations
	for _, agent := range idleAgents {
		if len(triage.Triage.Recommendations) == 0 {
			break // No more work to assign
		}

		// Find best match for this agent
		assignment, rec := c.findBestMatch(agent, triage.Triage.Recommendations)
		if assignment == nil {
			continue
		}

		// Attempt the assignment
		result := c.attemptAssignment(ctx, assignment, rec)
		results = append(results, result)

		if result.Success {
			// Remove this recommendation from the list
			triage.Triage.Recommendations = removeRecommendation(triage.Triage.Recommendations, rec.ID)

			// Emit event
			select {
			case c.events <- CoordinatorEvent{
				Type:      EventWorkAssigned,
				Timestamp: time.Now(),
				AgentID:   agent.PaneID,
				Details: map[string]any{
					"bead_id":    assignment.BeadID,
					"bead_title": assignment.BeadTitle,
					"agent_type": agent.AgentType,
					"score":      assignment.Score,
				},
			}:
			default:
			}
		}
	}

	return results, nil
}

// findBestMatch finds the best work recommendation for an agent.
func (c *SessionCoordinator) findBestMatch(agent *AgentState, recommendations []bv.TriageRecommendation) (*WorkAssignment, *bv.TriageRecommendation) {
	for _, rec := range recommendations {
		// Skip if blocked (status indicates blocked state)
		if rec.Status == "blocked" {
			continue
		}

		// Create assignment
		assignment := &WorkAssignment{
			BeadID:      rec.ID,
			BeadTitle:   rec.Title,
			AgentPaneID: agent.PaneID,
			AgentType:   agent.AgentType,
			AssignedAt:  time.Now(),
			Priority:    rec.Priority,
			Score:       rec.Score,
		}

		// Check agent mail name mapping
		if agent.AgentMailName != "" {
			assignment.AgentMailName = agent.AgentMailName
		}

		return assignment, &rec
	}

	return nil, nil
}

// attemptAssignment attempts to assign work to an agent.
func (c *SessionCoordinator) attemptAssignment(ctx context.Context, assignment *WorkAssignment, rec *bv.TriageRecommendation) AssignmentResult {
	result := AssignmentResult{
		Assignment: assignment,
	}

	// Reserve files if we know what files will be touched
	// For now, we don't pre-reserve since we don't know the files yet
	// The agent should reserve files when it starts working

	// Send assignment message if mail client available
	if c.mailClient != nil && assignment.AgentMailName != "" {
		body := c.formatAssignmentMessage(assignment, rec)
		_, err := c.mailClient.SendMessage(ctx, agentmail.SendMessageOptions{
			ProjectKey:  c.projectKey,
			SenderName:  c.agentName,
			To:          []string{assignment.AgentMailName},
			Subject:     fmt.Sprintf("Work Assignment: %s", assignment.BeadTitle),
			BodyMD:      body,
			Importance:  "normal",
			AckRequired: true,
		})

		if err != nil {
			result.Error = fmt.Sprintf("sending message: %v", err)
			return result
		}
		result.MessageSent = true
	}

	result.Success = true
	return result
}

// formatAssignmentMessage formats a work assignment message.
func (c *SessionCoordinator) formatAssignmentMessage(assignment *WorkAssignment, rec *bv.TriageRecommendation) string {
	var sb strings.Builder

	sb.WriteString("# Work Assignment\n\n")
	sb.WriteString(fmt.Sprintf("**Bead:** %s\n", assignment.BeadID))
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", assignment.BeadTitle))
	sb.WriteString(fmt.Sprintf("**Priority:** P%d\n", assignment.Priority))
	sb.WriteString(fmt.Sprintf("**Score:** %.2f\n\n", assignment.Score))

	if len(rec.Reasons) > 0 {
		sb.WriteString("## Why This Task\n\n")
		for _, reason := range rec.Reasons {
			sb.WriteString(fmt.Sprintf("- %s\n", reason))
		}
		sb.WriteString("\n")
	}

	if len(rec.UnblocksIDs) > 0 {
		sb.WriteString("## Impact\n\n")
		sb.WriteString(fmt.Sprintf("Completing this will unblock %d other tasks:\n", len(rec.UnblocksIDs)))
		for _, id := range rec.UnblocksIDs {
			if len(sb.String()) > 1500 {
				sb.WriteString("- ...\n")
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", id))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. Review the bead with `bd show " + assignment.BeadID + "`\n")
	sb.WriteString("2. Claim the work with `bd update " + assignment.BeadID + " --status in_progress`\n")
	sb.WriteString("3. Reserve any files you'll modify\n")
	sb.WriteString("4. Implement and test\n")
	sb.WriteString("5. Close with `bd close " + assignment.BeadID + "`\n")
	sb.WriteString("6. Commit with `.beads/` changes\n\n")

	sb.WriteString("Please acknowledge this message when you begin work.\n")

	return sb.String()
}

// removeRecommendation removes a recommendation by ID from the list.
func removeRecommendation(recs []bv.TriageRecommendation, id string) []bv.TriageRecommendation {
	result := make([]bv.TriageRecommendation, 0, len(recs)-1)
	for _, r := range recs {
		if r.ID != id {
			result = append(result, r)
		}
	}
	return result
}

// GetAssignableWork returns work items that could be assigned to idle agents.
func (c *SessionCoordinator) GetAssignableWork(ctx context.Context) ([]bv.TriageRecommendation, error) {
	triage, err := bv.GetTriage(c.projectKey)
	if err != nil {
		return nil, err
	}

	if triage == nil {
		return nil, nil
	}

	// Filter to unblocked items
	var assignable []bv.TriageRecommendation
	for _, rec := range triage.Triage.Recommendations {
		if rec.Status != "blocked" {
			assignable = append(assignable, rec)
		}
	}

	return assignable, nil
}

// SuggestAssignment suggests the best work for a specific agent without assigning.
func (c *SessionCoordinator) SuggestAssignment(ctx context.Context, paneID string) (*WorkAssignment, error) {
	agent := c.GetAgentByPaneID(paneID)
	if agent == nil {
		return nil, fmt.Errorf("agent not found: %s", paneID)
	}

	triage, err := bv.GetTriage(c.projectKey)
	if err != nil {
		return nil, err
	}

	if triage == nil || len(triage.Triage.Recommendations) == 0 {
		return nil, nil
	}

	assignment, _ := c.findBestMatch(agent, triage.Triage.Recommendations)
	return assignment, nil
}
