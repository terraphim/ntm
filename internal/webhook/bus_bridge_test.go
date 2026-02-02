package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/events"
	"github.com/Dicklesworthstone/ntm/internal/redaction"
)

func TestBusBridge_DispatchesWebhookEvents(t *testing.T) {
	t.Parallel()

	recv := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		select {
		case recv <- payload:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, ".ntm.yaml"), []byte(`
webhooks:
  - name: test
    url: `+srv.URL+`
    events: ["agent.error"]
    formatter: json
`), 0o644); err != nil {
		t.Fatalf("write .ntm.yaml: %v", err)
	}

	bus := events.NewEventBus(10)
	bridge, err := StartBridgeFromProjectConfig(projectDir, "mysession", bus, &redaction.Config{Mode: redaction.ModeOff})
	if err != nil {
		t.Fatalf("StartBridgeFromProjectConfig: %v", err)
	}
	if bridge == nil {
		t.Fatalf("expected bridge, got nil")
	}
	t.Cleanup(func() { _ = bridge.Close() })

	bus.PublishSync(events.NewWebhookEvent(
		events.WebhookAgentError,
		"mysession",
		"%1",
		"codex",
		"boom",
		map[string]string{"k": "v"},
	))

	select {
	case payload := <-recv:
		if payload["type"] != "agent.error" {
			t.Fatalf("type=%v, want agent.error", payload["type"])
		}
		if payload["session"] != "mysession" {
			t.Fatalf("session=%v, want mysession", payload["session"])
		}
		if payload["message"] != "boom" {
			t.Fatalf("message=%v, want boom", payload["message"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for webhook delivery")
	}
}
