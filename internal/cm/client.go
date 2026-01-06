package cm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Client struct {
	baseURL string
	client  *http.Client
}

// PIDFileInfo matches supervisor.PIDFileInfo
type PIDFileInfo struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	OwnerID   string    `json:"owner_id"`
	Command   string    `json:"command"`
	StartedAt time.Time `json:"started_at"`
}

// NewClient creates a new CM client by discovering the daemon port from the PID file.
func NewClient(projectDir, sessionID string) (*Client, error) {
	// Read PID file to find port
	pidPath := filepath.Join(projectDir, ".ntm", "pids", fmt.Sprintf("cm-%s.pid", sessionID))
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return nil, fmt.Errorf("reading cm pid file: %w", err)
	}

	var info PIDFileInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parsing cm pid file: %w", err)
	}

	return &Client{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", info.Port),
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

type ContextResult struct {
	RelevantBullets  []Rule    `json:"relevantBullets"`
	AntiPatterns     []Rule    `json:"antiPatterns"`
	HistorySnippets  []Snippet `json:"historySnippets"`
	SuggestedQueries []string  `json:"suggestedCassQueries"`
}

type Rule struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Category string `json:"category"`
}

type Snippet struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// GetContext queries CM for task-relevant rules via HTTP.
func (c *Client) GetContext(ctx context.Context, task string) (*ContextResult, error) {
	reqBody := map[string]string{"task": task}
	data, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/context", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("cm context failed: %s", resp.Status)
	}

	var result ContextResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

type OutcomeStatus string

const (
	OutcomeSuccess OutcomeStatus = "success"
	OutcomeFailure OutcomeStatus = "failure"
	OutcomePartial OutcomeStatus = "partial"
)

type OutcomeReport struct {
	Status    OutcomeStatus `json:"status"`
	RuleIDs   []string      `json:"rule_ids"`
	Sentiment string        `json:"sentiment"`
	Notes     string        `json:"notes,omitempty"`
}

// RecordOutcome sends feedback about rule effectiveness.
func (c *Client) RecordOutcome(ctx context.Context, report OutcomeReport) error {
	data, _ := json.Marshal(report)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/outcome", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("cm outcome failed: %s", resp.Status)
	}
	return nil
}
