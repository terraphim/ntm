package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/Dicklesworthstone/ntm/internal/ensemble"
	"github.com/Dicklesworthstone/ntm/tests/testutil"
)

type ensemblePresetsResponse struct {
	Count   int `json:"count"`
	Presets []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	} `json:"presets"`
}

func TestE2EEnsembleExportImport_Local(t *testing.T) {
	testutil.E2ETestPrecheck(t)
	logger := testutil.NewTestLoggerStdout(t)

	workDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", homeDir)

	exportPath := filepath.Join(workDir, "project-diagnosis.toml")
	logger.LogSection("export preset")
	_, err := logger.Exec("ntm", "ensemble", "export", "project-diagnosis", "--output", exportPath, "--force")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var payload ensemble.EnsembleExport
	if _, err := toml.DecodeFile(exportPath, &payload); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	payload.Name = "project-diagnosis-import"
	importPath := filepath.Join(workDir, "project-diagnosis-import.toml")
	f, err := os.Create(importPath)
	if err != nil {
		t.Fatalf("create import file: %v", err)
	}
	if err := toml.NewEncoder(f).Encode(payload); err != nil {
		_ = f.Close()
		t.Fatalf("encode import file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close import file: %v", err)
	}

	logger.LogSection("import preset")
	_, err = logger.Exec("ntm", "ensemble", "import", importPath)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	logger.LogSection("verify imported list")
	out := runCmd(t, workDir, "ntm", "ensemble", "presets", "--imported", "--format=json")
	var resp ensemblePresetsResponse
	if err := json.Unmarshal(extractJSON(out), &resp); err != nil {
		t.Fatalf("parse presets output: %v\n%s", err, string(out))
	}
	if resp.Count == 0 {
		t.Fatalf("expected imported presets, got none")
	}
	found := false
	for _, p := range resp.Presets {
		if p.Name == payload.Name && p.Source == "imported" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected imported preset %q, got %+v", payload.Name, resp.Presets)
	}
}

func TestE2EEnsembleExportImport_Remote(t *testing.T) {
	testutil.E2ETestPrecheck(t)
	logger := testutil.NewTestLoggerStdout(t)

	workDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", homeDir)

	exportPath := filepath.Join(workDir, "project-diagnosis.toml")
	_, err := logger.Exec("ntm", "ensemble", "export", "project-diagnosis", "--output", exportPath, "--force")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var payload ensemble.EnsembleExport
	if _, err := toml.DecodeFile(exportPath, &payload); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	payload.Name = "project-diagnosis-remote"

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatalf("encode export payload: %v", err)
	}
	data := buf.Bytes()
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	logger.LogSection("remote import without allow-remote")
	out, err := runCmdAllowFail(t, workDir, "ntm", "ensemble", "import", server.URL)
	if err == nil {
		t.Fatalf("expected failure for remote import without allow-remote")
	}
	if !strings.Contains(string(out), "allow-remote") {
		t.Fatalf("expected allow-remote error, got: %s", string(out))
	}

	logger.LogSection("remote import without checksum")
	out, err = runCmdAllowFail(t, workDir, "ntm", "ensemble", "import", server.URL, "--allow-remote")
	if err == nil {
		t.Fatalf("expected failure for remote import without sha256")
	}
	if !strings.Contains(string(out), "sha256") {
		t.Fatalf("expected sha256 error, got: %s", string(out))
	}

	logger.LogSection("remote import with checksum")
	_, err = logger.Exec("ntm", "ensemble", "import", server.URL, "--allow-remote", "--sha256", checksum)
	if err != nil {
		t.Fatalf("remote import failed: %v", err)
	}
}
