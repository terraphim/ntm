package cm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewClient(t *testing.T) {
	tmpDir := t.TempDir()
	pidsDir := filepath.Join(tmpDir, ".ntm", "pids")
	os.MkdirAll(pidsDir, 0755)

	sessionID := "test-session"
	info := PIDFileInfo{
		Port: 12345,
	}
	data, _ := json.Marshal(info)
	os.WriteFile(filepath.Join(pidsDir, fmt.Sprintf("cm-%s.pid", sessionID)), data, 0644)

	client, err := NewClient(tmpDir, sessionID)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.baseURL != "http://127.0.0.1:12345" {
		t.Errorf("NewClient() baseURL = %s, want http://127.0.0.1:12345", client.baseURL)
	}
}

func TestGetContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/context" {
			t.Errorf("path = %s, want /context", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ContextResult{
			RelevantBullets: []Rule{{ID: "r1", Content: "Use HTTP"}},
		})
	}))
	defer ts.Close()

	client := &Client{
		baseURL: ts.URL,
		client:  ts.Client(),
	}

	res, err := client.GetContext(context.Background(), "test task")
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}

	if len(res.RelevantBullets) != 1 || res.RelevantBullets[0].ID != "r1" {
		t.Errorf("GetContext() result = %v", res)
	}
}

func TestRecordOutcome(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/outcome" {
			t.Errorf("path = %s, want /outcome", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &Client{
		baseURL: ts.URL,
		client:  ts.Client(),
	}

	err := client.RecordOutcome(context.Background(), OutcomeReport{
		Status: OutcomeSuccess,
	})
	if err != nil {
		t.Fatalf("RecordOutcome() error = %v", err)
	}
}
