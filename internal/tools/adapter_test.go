package tools

import "testing"

func TestParseStandardVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantRaw   string
	}{
		{"simple version", "1.2.3", 1, 2, 3, "1.2.3"},
		{"with prefix", "v1.2.3", 1, 2, 3, "v1.2.3"},
		{"embedded in text", "bv version 0.31.0", 0, 31, 0, "bv version 0.31.0"},
		{"large numbers", "10.200.3000", 10, 200, 3000, "10.200.3000"},
		{"zero version", "0.0.0", 0, 0, 0, "0.0.0"},
		{"no version found", "no version here", 0, 0, 0, "no version here"},
		{"empty string", "", 0, 0, 0, ""},
		{"with whitespace", "  1.5.0  ", 1, 5, 0, "1.5.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v, err := ParseStandardVersion(tc.input)
			if err != nil {
				t.Fatalf("ParseStandardVersion(%q) error: %v", tc.input, err)
			}
			if v.Major != tc.wantMajor || v.Minor != tc.wantMinor || v.Patch != tc.wantPatch {
				t.Errorf("ParseStandardVersion(%q) = %d.%d.%d, want %d.%d.%d",
					tc.input, v.Major, v.Minor, v.Patch, tc.wantMajor, tc.wantMinor, tc.wantPatch)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    Version
		want string
	}{
		{"with raw", Version{Major: 1, Minor: 2, Patch: 3, Raw: "v1.2.3"}, "v1.2.3"},
		{"raw takes priority", Version{Major: 1, Minor: 0, Patch: 0, Raw: "custom-1.0"}, "custom-1.0"},
		{"no raw", Version{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
		{"zero version no raw", Version{}, "0.0.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.v.String()
			if got != tc.want {
				t.Errorf("Version.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAllTools(t *testing.T) {
	t.Parallel()

	tools := AllTools()

	if len(tools) == 0 {
		t.Fatal("AllTools() returned empty list")
	}

	// Check that known tools are present
	required := []ToolName{ToolBV, ToolBD, ToolAM, ToolCM, ToolCASS, ToolS2P, ToolDCG, ToolUBS}
	for _, r := range required {
		found := false
		for _, tool := range tools {
			if tool == r {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllTools() missing required tool %q", r)
		}
	}

	// Check uniqueness
	seen := make(map[ToolName]bool)
	for _, tool := range tools {
		if seen[tool] {
			t.Errorf("AllTools() contains duplicate: %q", tool)
		}
		seen[tool] = true
	}
}

func TestNewLimitedBuffer(t *testing.T) {
	t.Parallel()

	t.Run("within limit", func(t *testing.T) {
		t.Parallel()
		buf := NewLimitedBuffer(100)
		n, err := buf.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if n != 5 {
			t.Errorf("Write() = %d, want 5", n)
		}
		if buf.String() != "hello" {
			t.Errorf("buffer content = %q, want %q", buf.String(), "hello")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		t.Parallel()
		buf := NewLimitedBuffer(5)
		_, err := buf.Write([]byte("hello world"))
		if err != ErrOutputLimitExceeded {
			t.Errorf("Write() error = %v, want ErrOutputLimitExceeded", err)
		}
	})

	t.Run("exact limit", func(t *testing.T) {
		t.Parallel()
		buf := NewLimitedBuffer(5)
		_, err := buf.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		// Second write should fail
		_, err = buf.Write([]byte("!"))
		if err != ErrOutputLimitExceeded {
			t.Errorf("second Write() error = %v, want ErrOutputLimitExceeded", err)
		}
	})

	t.Run("multiple writes within limit", func(t *testing.T) {
		t.Parallel()
		buf := NewLimitedBuffer(20)
		buf.Write([]byte("hello "))
		buf.Write([]byte("world"))
		if buf.String() != "hello world" {
			t.Errorf("buffer content = %q, want %q", buf.String(), "hello world")
		}
	})
}
