package checkpoint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturer_CaptureGitState(t *testing.T) {
	// Create temp dir for git repo
	tmpDir, err := os.MkdirTemp("", "ntm-capture-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	if err := exec.Command("git", "-C", tmpDir, "init").Run(); err != nil {
		t.Fatalf("Failed to git init: %v", err)
	}

	// Configure git user for commits
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	// Create a file and commit it
	readme := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readme, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	c := NewCapturer()

	// Test success case
	state, err := c.captureGitState(tmpDir, "session", "chk-1")
	if err != nil {
		t.Errorf("captureGitState failed on valid repo: %v", err)
	}
	if state.Branch == "" {
		t.Error("Expected branch to be captured")
	}

	// Test failure case: corrupt the repo
	// Deleting .git/HEAD makes many git commands fail
	if err := os.Remove(filepath.Join(tmpDir, ".git", "HEAD")); err != nil {
		t.Fatalf("Failed to remove .git/HEAD: %v", err)
	}

	_, err = c.captureGitState(tmpDir, "session", "chk-2")
	if err == nil {
		t.Error("captureGitState should fail on corrupt repo")
	}
}

func TestCapturer_CaptureGitState_DirtySavesPatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ntm-capture-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := exec.Command("git", "-C", tmpDir, "init").Run(); err != nil {
		t.Fatalf("Failed to git init: %v", err)
	}
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test User").Run()

	readme := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readme, []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "Initial commit").Run()

	// Modify tracked file to produce a diff.
	if err := os.WriteFile(readme, []byte("updated"), 0644); err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	storage := NewStorageWithDir(tmpDir)
	c := NewCapturerWithStorage(storage)

	checkpointID := "chk-dirty"
	if err := os.MkdirAll(storage.CheckpointDir("session", checkpointID), 0755); err != nil {
		t.Fatalf("Failed to create checkpoint dir: %v", err)
	}

	state, err := c.captureGitState(tmpDir, "session", checkpointID)
	if err != nil {
		t.Fatalf("captureGitState failed on dirty repo: %v", err)
	}
	if !state.IsDirty {
		t.Fatal("expected dirty state")
	}
	if state.PatchFile != GitPatchFile {
		t.Fatalf("expected patch file %q, got %q", GitPatchFile, state.PatchFile)
	}

	patch, err := storage.LoadGitPatch("session", checkpointID)
	if err != nil {
		t.Fatalf("LoadGitPatch failed: %v", err)
	}
	if patch == "" {
		t.Fatal("expected git patch content")
	}
	if !strings.Contains(patch, "updated") {
		t.Fatalf("expected patch to contain updated content, got: %s", patch)
	}
}
