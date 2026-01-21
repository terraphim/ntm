package tools

import (
	"context"
	"testing"
	"time"
)

func TestNewCAAMAdapter(t *testing.T) {
	adapter := NewCAAMAdapter()
	if adapter == nil {
		t.Fatal("NewCAAMAdapter returned nil")
	}

	if adapter.Name() != ToolCAAM {
		t.Errorf("Expected name %s, got %s", ToolCAAM, adapter.Name())
	}

	if adapter.BinaryName() != "caam" {
		t.Errorf("Expected binary name 'caam', got %s", adapter.BinaryName())
	}
}

func TestCAAMAdapterImplementsInterface(t *testing.T) {
	// Ensure CAAMAdapter implements the Adapter interface
	var _ Adapter = (*CAAMAdapter)(nil)
}

func TestCAAMAdapterDetect(t *testing.T) {
	adapter := NewCAAMAdapter()

	// Test detection - result depends on whether caam is installed
	path, installed := adapter.Detect()

	// If installed, path should be non-empty
	if installed && path == "" {
		t.Error("caam detected but path is empty")
	}

	// If not installed, path should be empty
	if !installed && path != "" {
		t.Errorf("caam not detected but path is %s", path)
	}
}

func TestCAAMStatusStruct(t *testing.T) {
	status := CAAMStatus{
		Available:     true,
		Version:       "1.0.0",
		AccountsCount: 2,
		Providers:     []string{"claude", "openai"},
	}

	if !status.Available {
		t.Error("Expected Available to be true")
	}

	if status.AccountsCount != 2 {
		t.Errorf("Expected AccountsCount 2, got %d", status.AccountsCount)
	}

	if len(status.Providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(status.Providers))
	}
}

func TestCAAMAccountStruct(t *testing.T) {
	account := CAAMAccount{
		ID:          "acc-123",
		Provider:    "claude",
		Email:       "test@example.com",
		Active:      true,
		RateLimited: false,
	}

	if account.ID != "acc-123" {
		t.Errorf("Expected ID 'acc-123', got %s", account.ID)
	}

	if account.Provider != "claude" {
		t.Errorf("Expected Provider 'claude', got %s", account.Provider)
	}

	if !account.Active {
		t.Error("Expected Active to be true")
	}
}

func TestCAAMAdapterCacheInvalidation(t *testing.T) {
	adapter := NewCAAMAdapter()

	// Invalidate cache should not panic
	adapter.InvalidateCache()

	// Call again to ensure it's safe to call multiple times
	adapter.InvalidateCache()
}

func TestCAAMAdapterHealthWhenNotInstalled(t *testing.T) {
	adapter := NewCAAMAdapter()

	// If caam is not installed, Health should return non-healthy status
	_, installed := adapter.Detect()
	if installed {
		t.Skip("caam is installed, skipping not-installed test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := adapter.Health(ctx)
	if err != nil {
		t.Fatalf("Health returned unexpected error: %v", err)
	}

	if health.Healthy {
		t.Error("Expected unhealthy status when caam not installed")
	}

	if health.Message != "caam not installed" {
		t.Errorf("Expected message 'caam not installed', got %s", health.Message)
	}
}

func TestCAAMAdapterIsAvailable(t *testing.T) {
	adapter := NewCAAMAdapter()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// IsAvailable should not panic regardless of whether caam is installed
	available := adapter.IsAvailable(ctx)

	// If caam is not installed, should return false
	_, installed := adapter.Detect()
	if !installed && available {
		t.Error("IsAvailable returned true but caam is not installed")
	}
}

func TestCAAMAdapterHasMultipleAccounts(t *testing.T) {
	adapter := NewCAAMAdapter()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// HasMultipleAccounts should not panic regardless of whether caam is installed
	hasMultiple := adapter.HasMultipleAccounts(ctx)

	// If caam is not installed, should return false
	_, installed := adapter.Detect()
	if !installed && hasMultiple {
		t.Error("HasMultipleAccounts returned true but caam is not installed")
	}
}

func TestCAAMAdapterInRegistry(t *testing.T) {
	// Ensure CAAM adapter is registered in the global registry
	adapter, ok := Get(ToolCAAM)
	if !ok {
		t.Fatal("CAAM adapter not found in global registry")
	}

	if adapter.Name() != ToolCAAM {
		t.Errorf("Expected tool name %s, got %s", ToolCAAM, adapter.Name())
	}
}

func TestToolCAAMInAllTools(t *testing.T) {
	tools := AllTools()
	found := false
	for _, tool := range tools {
		if tool == ToolCAAM {
			found = true
			break
		}
	}

	if !found {
		t.Error("ToolCAAM not found in AllTools()")
	}
}
