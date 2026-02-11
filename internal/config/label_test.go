package config

import (
	"strings"
	"testing"
)

func TestParseSessionLabel(t *testing.T) {
	tests := []struct {
		input     string
		wantBase  string
		wantLabel string
	}{
		{"myproject", "myproject", ""},
		{"myproject--frontend", "myproject", "frontend"},
		{"my-project--frontend", "my-project", "frontend"},
		{"foo--bar--baz", "foo", "bar--baz"},
		{"proj--my-label", "proj", "my-label"},
		{"--frontend", "", "frontend"},       // degenerate: empty base
		{"myproject--", "myproject", ""},      // degenerate: empty label
		{"a--b", "a", "b"},                   // minimal
		{"abc", "abc", ""},                   // no separator
		{"a-b-c--d-e-f", "a-b-c", "d-e-f"},  // dashes everywhere
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotBase, gotLabel := ParseSessionLabel(tt.input)
			if gotBase != tt.wantBase {
				t.Errorf("ParseSessionLabel(%q) base = %q, want %q", tt.input, gotBase, tt.wantBase)
			}
			if gotLabel != tt.wantLabel {
				t.Errorf("ParseSessionLabel(%q) label = %q, want %q", tt.input, gotLabel, tt.wantLabel)
			}
		})
	}
}

func TestFormatSessionName(t *testing.T) {
	tests := []struct {
		base  string
		label string
		want  string
	}{
		{"myproject", "", "myproject"},
		{"myproject", "frontend", "myproject--frontend"},
		{"my-project", "backend", "my-project--backend"},
		{"a", "b", "a--b"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatSessionName(tt.base, tt.label)
			if got != tt.want {
				t.Errorf("FormatSessionName(%q, %q) = %q, want %q", tt.base, tt.label, got, tt.want)
			}
		})
	}
}

func TestFormatSessionName_RoundTrip(t *testing.T) {
	// FormatSessionName(ParseSessionLabel(x)) == x for valid labeled names
	inputs := []string{
		"myproject",
		"myproject--frontend",
		"my-project--backend",
		"a--b",
	}
	for _, input := range inputs {
		base, label := ParseSessionLabel(input)
		got := FormatSessionName(base, label)
		if got != input {
			t.Errorf("round-trip failed: input=%q, base=%q, label=%q, got=%q", input, base, label, got)
		}
	}
}

func TestHasLabel(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"myproject", false},
		{"my-project", false},
		{"myproject--frontend", true},
		{"a--b", true},
		{"foo--bar--baz", true},
		{"--frontend", true}, // degenerate but has --
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HasLabel(tt.input)
			if got != tt.want {
				t.Errorf("HasLabel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSessionBase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myproject", "myproject"},
		{"myproject--frontend", "myproject"},
		{"my-project--frontend", "my-project"},
		{"foo--bar--baz", "foo"},
		{"a--b", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SessionBase(tt.input)
			if got != tt.want {
				t.Errorf("SessionBase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateLabel(t *testing.T) {
	valid := []string{
		"frontend",
		"backend",
		"bugfix-123",
		"my_label",
		"a",
		"A1b2C3",
		"test-run-1",
	}
	for _, label := range valid {
		t.Run("valid_"+label, func(t *testing.T) {
			if err := ValidateLabel(label); err != nil {
				t.Errorf("ValidateLabel(%q) unexpected error: %v", label, err)
			}
		})
	}

	invalid := []struct {
		label       string
		errContains string
	}{
		{"", "empty"},
		{strings.Repeat("a", 51), "50 characters"},
		{"my--label", "separator"},
		{"-bad", "alphanumeric"},
		{"_bad", "alphanumeric"},
		{"bad!", "alphanumeric"},
		{"bad label", "alphanumeric"},
	}
	for _, tt := range invalid {
		name := tt.label
		if name == "" {
			name = "empty"
		}
		if len(name) > 20 {
			name = name[:20] + "..."
		}
		t.Run("invalid_"+name, func(t *testing.T) {
			err := ValidateLabel(tt.label)
			if err == nil {
				t.Errorf("ValidateLabel(%q) expected error containing %q, got nil", tt.label, tt.errContains)
				return
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("ValidateLabel(%q) error = %q, want containing %q", tt.label, err.Error(), tt.errContains)
			}
		})
	}
}

func TestGetProjectDir_WithLabel(t *testing.T) {
	cfg := &Config{ProjectsBase: "/home/user/projects"}

	// Unlabeled: unchanged behavior
	got := cfg.GetProjectDir("myproject")
	want := "/home/user/projects/myproject"
	if got != want {
		t.Errorf("GetProjectDir(%q) = %q, want %q", "myproject", got, want)
	}

	// Labeled: label stripped, returns base project dir
	got = cfg.GetProjectDir("myproject--frontend")
	if got != want {
		t.Errorf("GetProjectDir(%q) = %q, want %q", "myproject--frontend", got, want)
	}

	got = cfg.GetProjectDir("myproject--backend")
	if got != want {
		t.Errorf("GetProjectDir(%q) = %q, want %q", "myproject--backend", got, want)
	}

	// Multiple sessions resolve to SAME directory
	dir1 := cfg.GetProjectDir("myproject")
	dir2 := cfg.GetProjectDir("myproject--frontend")
	dir3 := cfg.GetProjectDir("myproject--backend")
	if dir1 != dir2 || dir2 != dir3 {
		t.Errorf("labeled sessions should resolve to same dir: %q, %q, %q", dir1, dir2, dir3)
	}

	// Different projects still resolve to different dirs
	dirA := cfg.GetProjectDir("project-a")
	dirB := cfg.GetProjectDir("project-b")
	if dirA == dirB {
		t.Errorf("different projects should resolve to different dirs: %q, %q", dirA, dirB)
	}

	// Dashes in project name preserved
	got = cfg.GetProjectDir("my-project--label")
	want = "/home/user/projects/my-project"
	if got != want {
		t.Errorf("GetProjectDir(%q) = %q, want %q", "my-project--label", got, want)
	}
}
