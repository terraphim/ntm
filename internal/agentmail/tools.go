package agentmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// EnsureProject ensures a project exists for the given path.
func (c *Client) EnsureProject(ctx context.Context, projectKey string) (*Project, error) {
	args := map[string]interface{}{
		"human_key": projectKey,
	}

	result, err := c.callTool(ctx, "ensure_project", args)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := json.Unmarshal(result, &project); err != nil {
		return nil, NewAPIError("ensure_project", 0, err)
	}

	return &project, nil
}

// RegisterAgent registers an agent in a project.
func (c *Client) RegisterAgent(ctx context.Context, opts RegisterAgentOptions) (*Agent, error) {
	args := map[string]interface{}{
		"project_key": opts.ProjectKey,
		"program":     opts.Program,
		"model":       opts.Model,
	}
	if opts.Name != "" {
		args["name"] = opts.Name
	}
	if opts.TaskDescription != "" {
		args["task_description"] = opts.TaskDescription
	}

	result, err := c.callTool(ctx, "register_agent", args)
	if err != nil {
		return nil, err
	}

	var agent Agent
	if err := json.Unmarshal(result, &agent); err != nil {
		return nil, NewAPIError("register_agent", 0, err)
	}

	return &agent, nil
}

// CreateAgentIdentity creates a new unique agent identity.
func (c *Client) CreateAgentIdentity(ctx context.Context, opts RegisterAgentOptions) (*Agent, error) {
	args := map[string]interface{}{
		"project_key": opts.ProjectKey,
		"program":     opts.Program,
		"model":       opts.Model,
	}
	if opts.Name != "" {
		args["name_hint"] = opts.Name
	}
	if opts.TaskDescription != "" {
		args["task_description"] = opts.TaskDescription
	}

	result, err := c.callTool(ctx, "create_agent_identity", args)
	if err != nil {
		return nil, err
	}

	var agent Agent
	if err := json.Unmarshal(result, &agent); err != nil {
		return nil, NewAPIError("create_agent_identity", 0, err)
	}

	return &agent, nil
}

// Whois retrieves agent profile details.
func (c *Client) Whois(ctx context.Context, projectKey, agentName string, includeRecentCommits bool) (*Agent, error) {
	args := map[string]interface{}{
		"project_key":            projectKey,
		"agent_name":             agentName,
		"include_recent_commits": includeRecentCommits,
	}

	result, err := c.callTool(ctx, "whois", args)
	if err != nil {
		return nil, err
	}

	var agent Agent
	if err := json.Unmarshal(result, &agent); err != nil {
		return nil, NewAPIError("whois", 0, err)
	}

	return &agent, nil
}

// SendMessage sends a message to one or more agents.
func (c *Client) SendMessage(ctx context.Context, opts SendMessageOptions) (*SendResult, error) {
	args := map[string]interface{}{
		"project_key": opts.ProjectKey,
		"sender_name": opts.SenderName,
		"to":          opts.To,
		"subject":     opts.Subject,
		"body_md":     opts.BodyMD,
	}
	if len(opts.CC) > 0 {
		args["cc"] = opts.CC
	}
	if len(opts.BCC) > 0 {
		args["bcc"] = opts.BCC
	}
	if opts.Importance != "" {
		args["importance"] = opts.Importance
	}
	if opts.AckRequired {
		args["ack_required"] = true
	}
	if opts.ThreadID != "" {
		args["thread_id"] = opts.ThreadID
	}
	if opts.ConvertImages != nil {
		args["convert_images"] = *opts.ConvertImages
	}

	result, err := c.callTool(ctx, "send_message", args)
	if err != nil {
		return nil, err
	}

	var sendResult SendResult
	if err := json.Unmarshal(result, &sendResult); err != nil {
		return nil, NewAPIError("send_message", 0, err)
	}

	return &sendResult, nil
}

// ReplyMessage replies to an existing message.
func (c *Client) ReplyMessage(ctx context.Context, opts ReplyMessageOptions) (*Message, error) {
	args := map[string]interface{}{
		"project_key": opts.ProjectKey,
		"message_id":  opts.MessageID,
		"sender_name": opts.SenderName,
		"body_md":     opts.BodyMD,
	}
	if len(opts.To) > 0 {
		args["to"] = opts.To
	}
	if len(opts.CC) > 0 {
		args["cc"] = opts.CC
	}
	if len(opts.BCC) > 0 {
		args["bcc"] = opts.BCC
	}
	if opts.SubjectPrefix != "" {
		args["subject_prefix"] = opts.SubjectPrefix
	}

	result, err := c.callTool(ctx, "reply_message", args)
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(result, &msg); err != nil {
		return nil, NewAPIError("reply_message", 0, err)
	}

	return &msg, nil
}

// FetchInbox retrieves inbox messages for an agent.
func (c *Client) FetchInbox(ctx context.Context, opts FetchInboxOptions) ([]InboxMessage, error) {
	args := map[string]interface{}{
		"project_key": opts.ProjectKey,
		"agent_name":  opts.AgentName,
	}
	if opts.UrgentOnly {
		args["urgent_only"] = true
	}
	if opts.SinceTS != nil {
		args["since_ts"] = opts.SinceTS.Format("2006-01-02T15:04:05Z07:00")
	}
	if opts.Limit > 0 {
		args["limit"] = opts.Limit
	}
	if opts.IncludeBodies {
		args["include_bodies"] = true
	}

	result, err := c.callTool(ctx, "fetch_inbox", args)
	if err != nil {
		return nil, err
	}

	// The result is wrapped in a "result" field
	var wrapper struct {
		Result []InboxMessage `json:"result"`
	}
	if err := json.Unmarshal(result, &wrapper); err != nil {
		// Try direct unmarshal
		var messages []InboxMessage
		if err := json.Unmarshal(result, &messages); err != nil {
			return nil, NewAPIError("fetch_inbox", 0, err)
		}
		return messages, nil
	}

	return wrapper.Result, nil
}

// MarkMessageRead marks a message as read for an agent.
func (c *Client) MarkMessageRead(ctx context.Context, projectKey, agentName string, messageID int) error {
	args := map[string]interface{}{
		"project_key": projectKey,
		"agent_name":  agentName,
		"message_id":  messageID,
	}

	_, err := c.callTool(ctx, "mark_message_read", args)
	return err
}

// AcknowledgeMessage acknowledges a message for an agent.
func (c *Client) AcknowledgeMessage(ctx context.Context, projectKey, agentName string, messageID int) error {
	args := map[string]interface{}{
		"project_key": projectKey,
		"agent_name":  agentName,
		"message_id":  messageID,
	}

	_, err := c.callTool(ctx, "acknowledge_message", args)
	return err
}

// RequestContact requests contact approval from another agent.
func (c *Client) RequestContact(ctx context.Context, projectKey, fromAgent, toAgent, reason string) error {
	args := map[string]interface{}{
		"project_key": projectKey,
		"from_agent":  fromAgent,
		"to_agent":    toAgent,
	}
	if reason != "" {
		args["reason"] = reason
	}

	_, err := c.callTool(ctx, "request_contact", args)
	return err
}

// RespondContact approves or denies a contact request.
func (c *Client) RespondContact(ctx context.Context, projectKey, toAgent, fromAgent string, accept bool) error {
	args := map[string]interface{}{
		"project_key": projectKey,
		"to_agent":    toAgent,
		"from_agent":  fromAgent,
		"accept":      accept,
	}

	_, err := c.callTool(ctx, "respond_contact", args)
	return err
}

// ListContacts lists contact links for an agent.
func (c *Client) ListContacts(ctx context.Context, projectKey, agentName string) ([]ContactLink, error) {
	args := map[string]interface{}{
		"project_key": projectKey,
		"agent_name":  agentName,
	}

	result, err := c.callTool(ctx, "list_contacts", args)
	if err != nil {
		return nil, err
	}

	var contacts []ContactLink
	if err := json.Unmarshal(result, &contacts); err != nil {
		return nil, NewAPIError("list_contacts", 0, err)
	}

	return contacts, nil
}

// SearchMessages searches messages by query.
func (c *Client) SearchMessages(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	args := map[string]interface{}{
		"project_key": opts.ProjectKey,
		"query":       opts.Query,
	}
	if opts.Limit > 0 {
		args["limit"] = opts.Limit
	}

	result, err := c.callToolWithTimeout(ctx, "search_messages", args, LongTimeout)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	if err := json.Unmarshal(result, &results); err != nil {
		return nil, NewAPIError("search_messages", 0, err)
	}

	return results, nil
}

// SummarizeThread summarizes a message thread.
func (c *Client) SummarizeThread(ctx context.Context, projectKey, threadID string, includeExamples bool) (*ThreadSummary, error) {
	args := map[string]interface{}{
		"project_key":      projectKey,
		"thread_id":        threadID,
		"include_examples": includeExamples,
	}

	result, err := c.callToolWithTimeout(ctx, "summarize_thread", args, LongTimeout)
	if err != nil {
		return nil, err
	}

	var summary ThreadSummary
	if err := json.Unmarshal(result, &summary); err != nil {
		return nil, NewAPIError("summarize_thread", 0, err)
	}

	return &summary, nil
}

// ReservePaths requests file path reservations.
func (c *Client) ReservePaths(ctx context.Context, opts FileReservationOptions) (*ReservationResult, error) {
	args := map[string]interface{}{
		"project_key": opts.ProjectKey,
		"agent_name":  opts.AgentName,
		"paths":       opts.Paths,
	}
	if opts.TTLSeconds > 0 {
		args["ttl_seconds"] = opts.TTLSeconds
	}
	if opts.Exclusive {
		args["exclusive"] = true
	}
	if opts.Reason != "" {
		args["reason"] = opts.Reason
	}

	result, err := c.callTool(ctx, "file_reservation_paths", args)
	if err != nil {
		return nil, err
	}

	var reservationResult ReservationResult
	if err := json.Unmarshal(result, &reservationResult); err != nil {
		return nil, NewAPIError("file_reservation_paths", 0, err)
	}

	// Check for conflicts
	if len(reservationResult.Conflicts) > 0 {
		return &reservationResult, fmt.Errorf("%w: %d conflicts", ErrReservationConflict, len(reservationResult.Conflicts))
	}

	return &reservationResult, nil
}

// ReleaseReservations releases file path reservations.
func (c *Client) ReleaseReservations(ctx context.Context, projectKey, agentName string, paths []string, ids []int) error {
	args := map[string]interface{}{
		"project_key": projectKey,
		"agent_name":  agentName,
	}
	if len(paths) > 0 {
		args["paths"] = paths
	}
	if len(ids) > 0 {
		args["file_reservation_ids"] = ids
	}

	_, err := c.callTool(ctx, "release_file_reservations", args)
	return err
}

// RenewReservations extends the TTL of existing reservations.
func (c *Client) RenewReservations(ctx context.Context, projectKey, agentName string, extendSeconds int) error {
	args := map[string]interface{}{
		"project_key":    projectKey,
		"agent_name":     agentName,
		"extend_seconds": extendSeconds,
	}

	_, err := c.callTool(ctx, "renew_file_reservations", args)
	return err
}

// ListReservations lists active file reservations for a project (optionally filtered by agent).
// If the Agent Mail server does not support this tool, callers will receive an error rather
// than an empty slice so the CLI can surface the limitation instead of misreporting "no locks".
func (c *Client) ListReservations(ctx context.Context, projectKey, agentName string, allAgents bool) ([]FileReservation, error) {
	args := map[string]interface{}{
		"project_key": projectKey,
	}
	if agentName != "" {
		args["agent_name"] = agentName
	}
	if allAgents {
		args["all_agents"] = true
	}

	// Primary tool name
	result, err := c.callTool(ctx, "list_file_reservations", args)
	if err != nil {
		// Some deployments may expose legacy name; try a fallback once.
		fallbackResult, fallbackErr := c.callTool(ctx, "list_reservations", args)
		if fallbackErr != nil {
			return nil, err // return original error to make diagnosis clear
		}
		result = fallbackResult
	}

	var reservations []FileReservation
	if err := json.Unmarshal(result, &reservations); err != nil {
		return nil, NewAPIError("list_file_reservations", 0, err)
	}
	return reservations, nil
}

// StartSession is a macro that starts a project session (ensure project, register agent, fetch inbox).
func (c *Client) StartSession(ctx context.Context, projectKey, program, model, taskDescription string) (*SessionStartResult, error) {
	args := map[string]interface{}{
		"human_key": projectKey,
		"program":   program,
		"model":     model,
	}
	if taskDescription != "" {
		args["task_description"] = taskDescription
	}

	result, err := c.callTool(ctx, "macro_start_session", args)
	if err != nil {
		return nil, err
	}

	var sessionResult SessionStartResult
	if err := json.Unmarshal(result, &sessionResult); err != nil {
		return nil, NewAPIError("macro_start_session", 0, err)
	}

	return &sessionResult, nil
}

// SendOverseerMessage sends a Human Overseer message via the HTTP REST API.
// This bypasses contact policies and auto-injects a preamble telling agents
// to prioritize the human's instructions. Messages are automatically marked
// as high importance.
//
// Note: This uses the HTTP REST API, not the MCP tools API, because the
// overseer functionality is specifically designed for human operators.
func (c *Client) SendOverseerMessage(ctx context.Context, opts OverseerMessageOptions) (*OverseerSendResult, error) {
	// Build request body
	reqBody := map[string]interface{}{
		"recipients": opts.Recipients,
		"subject":    opts.Subject,
		"body_md":    opts.BodyMD,
	}
	if opts.ThreadID != "" {
		reqBody["thread_id"] = opts.ThreadID
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, NewAPIError("overseer_send", 0, err)
	}

	// Build URL: /mail/{project_slug}/overseer/send
	httpBaseURL := c.httpBaseURL()
	url := fmt.Sprintf("%s/mail/%s/overseer/send", httpBaseURL, opts.ProjectSlug)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, NewAPIError("overseer_send", 0, err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, NewAPIError("overseer_send", 0, ErrTimeout)
		}
		return nil, NewAPIError("overseer_send", 0, ErrServerUnavailable)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAPIError("overseer_send", 0, err)
	}

	// Check HTTP status
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, NewAPIError("overseer_send", resp.StatusCode, ErrUnauthorized)
	}
	if resp.StatusCode == http.StatusBadRequest {
		// Try to extract error message from response
		var errResp struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Detail != "" {
			return nil, NewAPIError("overseer_send", resp.StatusCode, fmt.Errorf("%s", errResp.Detail))
		}
		return nil, NewAPIError("overseer_send", resp.StatusCode, fmt.Errorf("bad request"))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, NewAPIError("overseer_send", resp.StatusCode, fmt.Errorf("unexpected status: %s", resp.Status))
	}

	// Parse response
	var result OverseerSendResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, NewAPIError("overseer_send", 0, err)
	}

	return &result, nil
}

// ListProjectAgents lists all agents registered in a project.
// This is useful for discovering recipients for overseer messages.
func (c *Client) ListProjectAgents(ctx context.Context, projectKey string) ([]Agent, error) {
	// Use the MCP resource to list agents
	// Resource URI: resource://agents/{project_key}
	args := map[string]interface{}{
		"project_key": projectKey,
	}

	result, err := c.callTool(ctx, "list_agents", args)
	if err != nil {
		return nil, err
	}

	var agents []Agent
	if err := json.Unmarshal(result, &agents); err != nil {
		return nil, NewAPIError("list_agents", 0, err)
	}

	return agents, nil
}

// InstallPrecommitGuard installs the Agent Mail pre-commit guard for a repo.
func (c *Client) InstallPrecommitGuard(ctx context.Context, projectKey, repoPath string) error {
	args := map[string]interface{}{
		"project_key":    projectKey,
		"code_repo_path": repoPath,
	}

	_, err := c.callTool(ctx, "install_precommit_guard", args)
	return err
}

// UninstallPrecommitGuard removes the Agent Mail pre-commit guard from a repo.
func (c *Client) UninstallPrecommitGuard(ctx context.Context, repoPath string) error {
	args := map[string]interface{}{
		"code_repo_path": repoPath,
	}

	_, err := c.callTool(ctx, "uninstall_precommit_guard", args)
	return err
}
