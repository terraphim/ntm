package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CAAMAdapter provides integration with the CAAM (Coding Agent Account Manager) tool.
// CAAM manages multiple accounts for AI coding agents and handles automatic
// account rotation when rate limits are hit.
type CAAMAdapter struct {
	*BaseAdapter
}

// NewCAAMAdapter creates a new CAAM adapter
func NewCAAMAdapter() *CAAMAdapter {
	return &CAAMAdapter{
		BaseAdapter: NewBaseAdapter(ToolCAAM, "caam"),
	}
}

// Detect checks if caam is installed
func (a *CAAMAdapter) Detect() (string, bool) {
	path, err := exec.LookPath(a.BinaryName())
	if err != nil {
		return "", false
	}
	return path, true
}

// Version returns the installed caam version
func (a *CAAMAdapter) Version(ctx context.Context) (Version, error) {
	ctx, cancel := context.WithTimeout(ctx, a.Timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, a.BinaryName(), "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return Version{}, fmt.Errorf("failed to get caam version: %w", err)
	}

	return parseVersion(stdout.String())
}

// Capabilities returns the list of caam capabilities
func (a *CAAMAdapter) Capabilities(ctx context.Context) ([]Capability, error) {
	caps := []Capability{}

	path, installed := a.Detect()
	if !installed {
		return caps, nil
	}

	ctx, cancel := context.WithTimeout(ctx, a.Timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "help")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // Ignore error, just check output

	output := stdout.String()

	// Check for known capabilities
	if strings.Contains(output, "--json") || strings.Contains(output, "robot") {
		caps = append(caps, CapRobotMode)
	}

	return caps, nil
}

// Health checks if caam is functioning correctly
func (a *CAAMAdapter) Health(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()

	path, installed := a.Detect()
	if !installed {
		return &HealthStatus{
			Healthy:     false,
			Message:     "caam not installed",
			LastChecked: time.Now(),
		}, nil
	}

	// Try to get version as a basic health check
	_, err := a.Version(ctx)
	latency := time.Since(start)

	if err != nil {
		return &HealthStatus{
			Healthy:     false,
			Message:     fmt.Sprintf("caam at %s not responding", path),
			Error:       err.Error(),
			LastChecked: time.Now(),
			Latency:     latency,
		}, nil
	}

	return &HealthStatus{
		Healthy:     true,
		Message:     "caam is healthy",
		LastChecked: time.Now(),
		Latency:     latency,
	}, nil
}

// HasCapability checks if caam has a specific capability
func (a *CAAMAdapter) HasCapability(ctx context.Context, cap Capability) bool {
	caps, err := a.Capabilities(ctx)
	if err != nil {
		return false
	}
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}

// Info returns complete caam tool information
func (a *CAAMAdapter) Info(ctx context.Context) (*ToolInfo, error) {
	return a.BaseAdapter.Info(ctx, a)
}

// CAAM-specific types and methods

// CAAMAccount represents an account managed by CAAM
type CAAMAccount struct {
	ID            string    `json:"id"`
	Provider      string    `json:"provider"`
	Email         string    `json:"email,omitempty"`
	Name          string    `json:"name,omitempty"`
	Active        bool      `json:"active"`
	RateLimited   bool      `json:"rate_limited,omitempty"`
	CooldownUntil time.Time `json:"cooldown_until,omitempty"`
}

// CAAMStatus represents the current CAAM status
type CAAMStatus struct {
	Available     bool          `json:"available"`
	Version       string        `json:"version,omitempty"`
	AccountsCount int           `json:"accounts_count"`
	Providers     []string      `json:"providers,omitempty"`
	ActiveAccount *CAAMAccount  `json:"active_account,omitempty"`
	Accounts      []CAAMAccount `json:"accounts,omitempty"`
}

// Cached status to avoid repeated lookups
var (
	caamStatusOnce   sync.Once
	caamStatusCache  CAAMStatus
	caamStatusExpiry time.Time
	caamStatusMutex  sync.RWMutex
	caamCacheTTL     = 5 * time.Minute
)

// GetStatus returns the current CAAM status with caching
func (a *CAAMAdapter) GetStatus(ctx context.Context) (*CAAMStatus, error) {
	// Check cache first
	caamStatusMutex.RLock()
	if time.Now().Before(caamStatusExpiry) {
		status := caamStatusCache
		caamStatusMutex.RUnlock()
		return &status, nil
	}
	caamStatusMutex.RUnlock()

	// Fetch fresh status
	status, err := a.fetchStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	caamStatusMutex.Lock()
	caamStatusCache = *status
	caamStatusExpiry = time.Now().Add(caamCacheTTL)
	caamStatusMutex.Unlock()

	return status, nil
}

// fetchStatus fetches fresh status from caam
func (a *CAAMAdapter) fetchStatus(ctx context.Context) (*CAAMStatus, error) {
	status := &CAAMStatus{}

	// Check if caam is installed
	path, installed := a.Detect()
	if !installed {
		status.Available = false
		return status, nil
	}
	status.Available = true

	// Get version
	version, err := a.Version(ctx)
	if err == nil {
		status.Version = version.String()
	}

	// Get accounts list
	ctx, cancel := context.WithTimeout(ctx, a.Timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "list", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// caam might not have accounts configured - that's ok
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrTimeout
		}
		// Return status without accounts
		return status, nil
	}

	output := stdout.Bytes()
	if len(output) == 0 || !json.Valid(output) {
		return status, nil
	}

	// Parse accounts list
	var accounts []CAAMAccount
	if err := json.Unmarshal(output, &accounts); err != nil {
		// Try parsing as a status object instead
		var statusResp struct {
			Accounts []CAAMAccount `json:"accounts"`
		}
		if err := json.Unmarshal(output, &statusResp); err == nil {
			accounts = statusResp.Accounts
		}
	}

	status.Accounts = accounts
	status.AccountsCount = len(accounts)

	// Extract unique providers
	providerSet := make(map[string]bool)
	for _, acc := range accounts {
		if acc.Provider != "" {
			providerSet[acc.Provider] = true
		}
		if acc.Active {
			status.ActiveAccount = &acc
		}
	}
	for p := range providerSet {
		status.Providers = append(status.Providers, p)
	}

	return status, nil
}

// GetAccounts returns the list of configured accounts
func (a *CAAMAdapter) GetAccounts(ctx context.Context) ([]CAAMAccount, error) {
	status, err := a.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	return status.Accounts, nil
}

// GetActiveAccount returns the currently active account
func (a *CAAMAdapter) GetActiveAccount(ctx context.Context) (*CAAMAccount, error) {
	status, err := a.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	return status.ActiveAccount, nil
}

// SwitchAccount switches to a different account
func (a *CAAMAdapter) SwitchAccount(ctx context.Context, accountID string) error {
	path, installed := a.Detect()
	if !installed {
		return ErrToolNotInstalled
	}

	ctx, cancel := context.WithTimeout(ctx, a.Timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "switch", accountID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ErrTimeout
		}
		return fmt.Errorf("failed to switch account: %w: %s", err, stderr.String())
	}

	// Invalidate cache after switch
	caamStatusMutex.Lock()
	caamStatusExpiry = time.Time{}
	caamStatusMutex.Unlock()

	return nil
}

// InvalidateCache forces the next GetStatus call to fetch fresh data
func (a *CAAMAdapter) InvalidateCache() {
	caamStatusMutex.Lock()
	caamStatusExpiry = time.Time{}
	caamStatusMutex.Unlock()
}

// IsAvailable is a convenience method that returns true if caam is installed
// and has at least one account configured
func (a *CAAMAdapter) IsAvailable(ctx context.Context) bool {
	status, err := a.GetStatus(ctx)
	if err != nil {
		return false
	}
	return status.Available && status.AccountsCount > 0
}

// HasMultipleAccounts returns true if caam has more than one account configured
func (a *CAAMAdapter) HasMultipleAccounts(ctx context.Context) bool {
	status, err := a.GetStatus(ctx)
	if err != nil {
		return false
	}
	return status.AccountsCount > 1
}
