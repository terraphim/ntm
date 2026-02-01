package robot

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Dicklesworthstone/ntm/internal/tools"
)

func TestGetDCGCheck_MissingCommand(t *testing.T) {
	out, err := GetDCGCheck("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Success {
		t.Fatalf("expected success=false for missing command")
	}
	if out.ErrorCode != ErrCodeInvalidFlag {
		t.Fatalf("expected error_code=%s, got %s", ErrCodeInvalidFlag, out.ErrorCode)
	}
	if out.Allowed {
		t.Fatalf("expected allowed=false for missing command")
	}
}

func TestGetDCGCheck_MissingDCG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim test uses a unix shell script")
	}

	tools.NewDCGAdapter().InvalidateAvailabilityCache()

	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	out, err := GetDCGCheck("rm -rf /tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Success {
		t.Fatalf("expected success=false when dcg missing")
	}
	if out.ErrorCode != ErrCodeDependencyMissing {
		t.Fatalf("expected error_code=%s, got %s", ErrCodeDependencyMissing, out.ErrorCode)
	}
}

func TestGetDCGCheck_Allowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim test uses a unix shell script")
	}

	tools.NewDCGAdapter().InvalidateAvailabilityCache()

	tmpDir := t.TempDir()
	writeFakeDCG(t, tmpDir)
	t.Setenv("PATH", tmpDir)

	out, err := GetDCGCheck("echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Fatalf("expected success=true")
	}
	if !out.Allowed {
		t.Fatalf("expected allowed=true")
	}
	if out.DCGVersion == "" {
		t.Fatalf("expected dcg_version to be set")
	}
	if out.BinaryPath == "" {
		t.Fatalf("expected binary_path to be set")
	}
}

func TestGetDCGCheck_Blocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH shim test uses a unix shell script")
	}

	tools.NewDCGAdapter().InvalidateAvailabilityCache()

	tmpDir := t.TempDir()
	writeFakeDCG(t, tmpDir)
	t.Setenv("PATH", tmpDir)

	out, err := GetDCGCheck("rm -rf /tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.Success {
		t.Fatalf("expected success=true (check succeeded even if blocked)")
	}
	if out.Allowed {
		t.Fatalf("expected allowed=false")
	}
	if out.Reason == "" {
		t.Fatalf("expected reason to be set for blocked command")
	}
}

func writeFakeDCG(t *testing.T, dir string) {
	t.Helper()

	dcgPath := filepath.Join(dir, "dcg")
	script := `#!/bin/sh
set -eu

if [ "${1:-}" = "--version" ]; then
  echo "dcg 1.2.3"
  exit 0
fi

if [ "${1:-}" = "check" ]; then
  # args: check --json "<command>"
  shift
  if [ "${1:-}" = "--json" ]; then
    shift
  fi
  cmd="${1:-}"
  case "$cmd" in
    *"rm -rf"*)
      echo "{\"command\":\"$cmd\",\"reason\":\"blocked by fake policy\"}"
      exit 1
      ;;
  esac
  exit 0
fi

if [ "${1:-}" = "status" ]; then
  echo "{\"enabled\":true}"
  exit 0
fi

exit 0
`

	if err := os.WriteFile(dcgPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake dcg: %v", err)
	}
}
