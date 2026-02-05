package dcg

import "testing"

func TestDefaultRCHHookOptions(t *testing.T) {
	opts := DefaultRCHHookOptions()
	if opts.Timeout != 5000 {
		t.Errorf("Expected default timeout 5000, got %d", opts.Timeout)
	}
}

func TestGenerateRCHHookEntry_Basic(t *testing.T) {
	opts := RCHHookOptions{
		Patterns: []string{"^cargo build", "^go build"},
	}

	entry, err := GenerateRCHHookEntry(opts)
	if err != nil {
		t.Fatalf("GenerateRCHHookEntry failed: %v", err)
	}

	if entry.Matcher != "Bash" {
		t.Errorf("Expected matcher 'Bash', got %q", entry.Matcher)
	}

	if entry.Timeout != 5000 {
		t.Errorf("Expected timeout 5000, got %d", entry.Timeout)
	}

	if !contains(entry.Command, "rch") {
		t.Errorf("Expected command to contain rch, got %q", entry.Command)
	}

	if !contains(entry.Command, "intercept") {
		t.Errorf("Expected command to contain intercept, got %q", entry.Command)
	}

	if !contains(entry.Command, "cargo build") {
		t.Errorf("Expected command to include cargo build pattern, got %q", entry.Command)
	}
}

func TestGenerateRCHHookEntry_CustomBinary(t *testing.T) {
	opts := RCHHookOptions{
		BinaryPath: "/opt/rch",
		Patterns:   []string{"^go build"},
		Timeout:    3000,
	}

	entry, err := GenerateRCHHookEntry(opts)
	if err != nil {
		t.Fatalf("GenerateRCHHookEntry failed: %v", err)
	}

	if !contains(entry.Command, "/opt/rch") {
		t.Errorf("Expected command to include custom binary path, got %q", entry.Command)
	}

	if entry.Timeout != 3000 {
		t.Errorf("Expected timeout 3000, got %d", entry.Timeout)
	}
}

func TestGenerateRCHHookEntry_EmptyPatterns(t *testing.T) {
	_, err := GenerateRCHHookEntry(RCHHookOptions{})
	if err == nil {
		t.Fatal("Expected error with empty patterns, got nil")
	}
}

func TestShouldConfigureRCHHooks(t *testing.T) {
	if ShouldConfigureRCHHooks(false, []string{"^cargo build"}) {
		t.Error("Expected hooks disabled when RCH is disabled")
	}

	if ShouldConfigureRCHHooks(true, []string{}) {
		t.Error("Expected hooks disabled with empty patterns")
	}

	if !ShouldConfigureRCHHooks(true, []string{"^cargo build"}) {
		t.Error("Expected hooks enabled when RCH is enabled with patterns")
	}
}
