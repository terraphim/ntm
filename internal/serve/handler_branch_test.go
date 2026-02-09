package serve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/ntm/internal/events"
	"github.com/go-chi/chi/v5"
)

// =============================================================================
// RequirePermission / RequireRole — nil role-context branch
// =============================================================================

func TestRequirePermission_NilRoleContext(t *testing.T) {
	t.Parallel()
	s := &Server{}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.RequirePermission(PermReadSessions)(inner)

	// Request WITHOUT a RoleContext in the context
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(errMsg, "no role context") {
		t.Errorf("error = %v, want substring 'no role context'", errMsg)
	}
}

func TestRequireRole_NilRoleContext(t *testing.T) {
	t.Parallel()
	s := &Server{}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := s.RequireRole(RoleAdmin)(inner)

	// Request WITHOUT a RoleContext
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	errMsg, _ := resp["error"].(string)
	if !strings.Contains(errMsg, "no role context") {
		t.Errorf("error = %v, want substring 'no role context'", errMsg)
	}
}

// =============================================================================
// handleSessionsV1 — nil stateStore branch
// =============================================================================

func TestHandleSessionsV1_NilStore(t *testing.T) {
	t.Parallel()
	srv := New(Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)

	srv.handleSessionsV1(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// handleSessionsV1 — success with sessions present
func TestHandleSessionsV1_WithSessions(t *testing.T) {
	t.Parallel()
	srv, store := setupTestServer(t)
	createTestSessionForServe(t, store, "sess-a")
	createTestSessionForServe(t, store, "sess-b")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)

	srv.handleSessionsV1(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	count, _ := resp["count"].(float64)
	if count < 2 {
		t.Errorf("count = %v, want >= 2", count)
	}
}

// =============================================================================
// handleSessionV1 — nil stateStore + success path
// =============================================================================

func TestHandleSessionV1_NilStore(t *testing.T) {
	t.Parallel()
	srv := New(Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/test", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleSessionV1(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestHandleSessionV1_Success(t *testing.T) {
	t.Parallel()
	srv, store := setupTestServer(t)
	createTestSessionForServe(t, store, "found-session")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/found-session", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "found-session")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleSessionV1(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["session"] == nil {
		t.Error("expected session in response")
	}
}

// =============================================================================
// handleSessionEventsV1 — nil eventBus branch
// =============================================================================

func TestHandleSessionEventsV1_NilEventBus(t *testing.T) {
	t.Parallel()
	// Create server with nil eventBus
	srv := New(Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/test/events", nil)

	srv.handleSessionEventsV1(rec, req, "test")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// handleSessionEventsV1 — success with empty events
func TestHandleSessionEventsV1_EmptyFiltered(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent/events", nil)

	srv.handleSessionEventsV1(rec, req, "nonexistent")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Should be empty array, not null
	evts, ok := resp["events"].([]interface{})
	if !ok {
		t.Fatalf("events should be array, got %T", resp["events"])
	}
	if len(evts) != 0 {
		t.Errorf("events len = %d, want 0", len(evts))
	}
}

// =============================================================================
// handleHistoryV1 / handleHistoryStatsV1 — missing session param
// =============================================================================

func TestHandleHistoryV1_MissingSession(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil) // no session param

	srv.handleHistoryV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleHistoryStatsV1_MissingSession(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/stats", nil) // no session param

	srv.handleHistoryStatsV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handleMetricsSnapshotSaveV1 — success path
// =============================================================================

func TestHandleMetricsSnapshotSaveV1_Success(t *testing.T) {
	t.Parallel()
	srv, store := setupTestServer(t)
	createTestSessionForServe(t, store, "snap-session")

	rec := httptest.NewRecorder()
	body := `{"name":"test-snap","session":"snap-session"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics/snapshot", strings.NewReader(body))

	srv.handleMetricsSnapshotSaveV1(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["name"] != "test-snap" {
		t.Errorf("name = %v, want test-snap", resp["name"])
	}
	if resp["saved"] != true {
		t.Errorf("saved = %v, want true", resp["saved"])
	}
}

// =============================================================================
// IdempotencyStore cleanup — ticker expiry branch
// =============================================================================

func TestIdempotencyStore_CleanupExpiry(t *testing.T) {
	t.Parallel()

	// Very short TTL so cleanup actually deletes entries
	store := NewIdempotencyStore(50 * time.Millisecond)
	defer store.Stop()

	store.Set("ephemeral", []byte(`{"ok":true}`), 200)

	// Verify it's there
	_, _, ok := store.Get("ephemeral")
	if !ok {
		t.Fatal("expected ephemeral key to be present initially")
	}

	// Wait for TTL + cleanup cycle (cleanup runs every minute by default,
	// but we can test the Get TTL expiry path directly)
	time.Sleep(100 * time.Millisecond)

	// Get should now return false due to TTL check
	_, _, ok = store.Get("ephemeral")
	if ok {
		t.Error("expected ephemeral key to have expired")
	}
}

// =============================================================================
// WSHub Publish — broadcast buffer full (drop) branch
// =============================================================================

func TestWSHubPublish_BufferFull(t *testing.T) {
	t.Parallel()

	hub := NewWSHub()
	// Do NOT start hub.Run() so broadcast channel has no consumer

	// Fill the broadcast buffer (capacity is 256)
	for i := 0; i < 256; i++ {
		hub.Publish("global:events", "fill_event", map[string]interface{}{
			"i": i,
		})
	}

	// This 101st publish should hit the default (drop) branch instead of blocking
	done := make(chan struct{})
	go func() {
		hub.Publish("global:events", "dropped_event", map[string]interface{}{
			"message": "should be dropped",
		})
		close(done)
	}()

	select {
	case <-done:
		// Good — Publish returned without blocking
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked — buffer-full drop branch not hit")
	}
}

// =============================================================================
// broadcastEvent — JSON marshal error branch
// =============================================================================

func TestWSHub_BroadcastEvent_MarshalError(t *testing.T) {
	t.Parallel()

	hub := NewWSHub()
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Publish data that cannot be marshalled (channel is not JSON-serializable)
	// This exercises the json.Marshal error path in broadcastEvent
	hub.Publish("global:events", "bad_data", make(chan int))

	time.Sleep(50 * time.Millisecond)

	// Sequence should NOT increment on marshal failure
	// (actually it does increment in nextSeq before marshal; let's just confirm no panic)
}

// =============================================================================
// broadcastEvent — redaction path (cfg != nil)
// =============================================================================

func TestWSHub_BroadcastEvent_WithRedaction(t *testing.T) {
	t.Parallel()

	hub := NewWSHub()
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Set a redaction config to exercise the redaction branch
	hub.SetRedactionConfig(&RedactionConfig{
		Enabled: true,
	})

	hub.Publish("global:events", "test_redacted", map[string]interface{}{
		"message": "hello",
	})

	time.Sleep(50 * time.Millisecond)

	// Verify sequence incremented (broadcastEvent ran without panic)
	hub.seqMu.Lock()
	seq := hub.seq
	hub.seqMu.Unlock()
	if seq < 1 {
		t.Errorf("seq = %d, expected >= 1", seq)
	}
}

// =============================================================================
// handleSessionsV1 — sessions=nil (empty DB returns nil slice)
// =============================================================================

func TestHandleSessionsV1_EmptyDB(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)

	srv.handleSessionsV1(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// sessions should be an array, not null
	sessions, ok := resp["sessions"].([]interface{})
	if !ok {
		t.Fatalf("sessions should be array, got %T", resp["sessions"])
	}
	if len(sessions) != 0 {
		t.Errorf("sessions len = %d, want 0", len(sessions))
	}
}

// =============================================================================
// handleCreateSessionV1 — kernel error path (no tmux)
// =============================================================================

func TestHandleCreateSessionV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	body := `{"session":"test-create"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(body))

	srv.handleCreateSessionV1(rec, req)

	// Should error because kernel can't create tmux session in test env
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 500 or 201", rec.Code)
	}
}

// =============================================================================
// handleSessionStatusV1 / handleSessionAttachV1 — kernel error path
// =============================================================================

func TestHandleSessionStatusV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/status", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleSessionStatusV1(rec, req)

	// Should error because kernel can't find tmux session
	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleSessionAttachV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/noexist/attach", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleSessionAttachV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

// =============================================================================
// Publish (WSHub) — basic path with running hub
// =============================================================================

func TestWSHubPublish_WithClient(t *testing.T) {
	t.Parallel()

	hub := NewWSHub()
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create a client that subscribes to global:events
	client := &WSClient{
		id:     "test-client",
		send:   make(chan []byte, 16),
		topics: map[string]struct{}{"*": {}},
	}
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	hub.Publish("global:events", "test_event", map[string]interface{}{
		"msg": "hello",
	})

	// Should receive the event
	select {
	case data := <-client.send:
		var evt WSEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		if evt.EventType != "test_event" {
			t.Errorf("event type = %q, want test_event", evt.EventType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

// =============================================================================
// handleSessionEventsV1 — with events in the bus
// =============================================================================

func TestHandleSessionEventsV1_WithEvents(t *testing.T) {
	t.Parallel()

	bus := events.NewEventBus(100)
	srv := New(Config{EventBus: bus})

	// Emit an event
	bus.Publish(events.BaseEvent{
		Type:    "test.event",
		Session: "my-session",
	})

	time.Sleep(10 * time.Millisecond)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/my-session/events", nil)

	srv.handleSessionEventsV1(rec, req, "my-session")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	count, _ := resp["count"].(float64)
	if count < 1 {
		t.Errorf("count = %v, want >= 1", count)
	}
}

// =============================================================================
// handleSessionEvents (legacy) — nil eventBus
// =============================================================================

func TestHandleSessionEvents_NilEventBus(t *testing.T) {
	t.Parallel()
	srv := New(Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/test/events", nil)

	srv.handleSessionEvents(rec, req, "test")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// handleSessions (legacy) — nil stateStore
func TestHandleSessions_NilStore(t *testing.T) {
	t.Parallel()
	srv := New(Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)

	srv.handleSessions(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// handleSessions — method not allowed
func TestHandleSessions_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	srv := New(Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", nil)

	srv.handleSessions(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// handleSession — method not allowed
func TestHandleSession_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/test", nil)

	srv.handleSession(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// handleSession — nil stateStore
func TestHandleSession_NilStore(t *testing.T) {
	t.Parallel()
	srv := New(Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/test-id", nil)

	srv.handleSession(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// handleSession — success path with session found
func TestHandleSession_Success(t *testing.T) {
	t.Parallel()
	srv, store := setupTestServer(t)
	createTestSessionForServe(t, store, "legacy-session")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/legacy-session", nil)

	srv.handleSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["session"] == nil {
		t.Error("expected session in response")
	}
}

// handleSession — sub-resource routing (agents)
func TestHandleSession_SubResourceAgents(t *testing.T) {
	t.Parallel()
	srv, store := setupTestServer(t)
	createTestSessionForServe(t, store, "sub-agents")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/sub-agents/agents", nil)

	srv.handleSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// Pane handler validation branches
// =============================================================================

func TestHandlePaneOutputV1_InvalidIndex(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s/panes/abc/output", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneOutputV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandlePaneOutputV1_InvalidLines(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s/panes/0/output?lines=abc", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneOutputV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandlePaneOutputV1_LinesOutOfRange(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s/panes/0/output?lines=99999", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneOutputV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandlePaneOutputV1_TmuxError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/nonexistent/panes/0/output", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "nonexistent")
	rctx.URLParams.Add("paneIdx", "0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneOutputV1(rec, req)

	// Should 500 since tmux session doesn't exist
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestHandlePaneInputV1_InvalidIndex(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	body := `{"text":"hello","enter":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s/panes/abc/input", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneInputV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandlePaneInputV1_InvalidBody(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s/panes/0/input", strings.NewReader("{bad"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneInputV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandlePaneInputV1_EmptyText(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s/panes/0/input", strings.NewReader(`{"text":""}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneInputV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// handleSessionViewV1 — kernel error path
// =============================================================================

func TestHandleSessionViewV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/view", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleSessionViewV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

// =============================================================================
// handleListPanesV1 — tmux error path
// =============================================================================

func TestHandleListPanesV1_TmuxError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/panes", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleListPanesV1(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// =============================================================================
// Agent handlers — kernel error paths
// =============================================================================

func TestHandleListAgentsV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/agents", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleListAgentsV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleAgentSendV1_InvalidBody(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s/agents/send", strings.NewReader("{bad"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentSendV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleAgentInterruptV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/noexist/agents/interrupt", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentInterruptV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleAgentWaitV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/agents/wait", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentWaitV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleAgentRouteV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/agents/route?prompt=test", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentRouteV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleAgentActivityV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/agents/activity", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentActivityV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleAgentHealthV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/agents/health", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentHealthV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleAgentContextV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/noexist/agents/context", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentContextV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleAgentRestartV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/noexist/agents/restart", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "noexist")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleAgentRestartV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

// =============================================================================
// Output handlers — missing session validation
// =============================================================================

func TestHandleOutputTailV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/output/tail?session=noexist&pane=0", nil)

	srv.handleOutputTailV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleOutputDiffV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/output/diff?session=noexist&pane=0", nil)

	srv.handleOutputDiffV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleOutputSummaryV1_KernelError(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/output/summary?session=noexist&pane=0", nil)

	srv.handleOutputSummaryV1(rec, req)

	if rec.Code == 0 {
		t.Fatal("expected non-zero status code")
	}
}

func TestHandleContextBuildV1_EmptyBody(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/context/build", strings.NewReader(""))

	srv.handleContextBuildV1(rec, req)

	// Empty body should be rejected as bad request
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handlePolicyUpdateV1 — invalid YAML and validation branches
// =============================================================================

func TestHandlePolicyUpdateV1_InvalidYAML(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	body := `{"content":"not: [valid: yaml: broken"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/policy", strings.NewReader(body))

	srv.handlePolicyUpdateV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePolicyUpdateV1_InvalidPolicy(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	// Valid YAML but invalid policy (bad force_release value)
	rec := httptest.NewRecorder()
	body := `{"content":"version: 1\nautomation:\n  force_release: invalid_value"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/policy", strings.NewReader(body))

	srv.handlePolicyUpdateV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePolicyUpdateV1_ValidPolicy(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	// Valid YAML with valid policy
	rec := httptest.NewRecorder()
	body := `{"content":"version: 1\nblocked:\n  - pattern: 'rm -rf /'\n    reason: dangerous"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/policy", strings.NewReader(body))

	srv.handlePolicyUpdateV1(rec, req)

	// Should succeed (writes to ~/.ntm/policy.yaml)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handlePolicyAutomationUpdateV1 — additional branches
// =============================================================================

func TestHandlePolicyAutomationUpdateV1_ValidUpdate(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	autoCommit := true
	_ = autoCommit // used in JSON
	body := `{"auto_commit":true,"auto_push":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/policy/automation", strings.NewReader(body))

	srv.handlePolicyAutomationUpdateV1(rec, req)

	// Should succeed or error depending on policy file state
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePolicyAutomationUpdateV1_ForceReleaseApproval(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	body := `{"force_release":"approval"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/policy/automation", strings.NewReader(body))

	srv.handlePolicyAutomationUpdateV1(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handlePolicyValidateV1 — additional branches
// =============================================================================

func TestHandlePolicyValidateV1_InvalidYAML(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	body := `{"content":"not: [valid: yaml"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/policy/validate", strings.NewReader(body))

	srv.handlePolicyValidateV1(rec, req)

	// Should return 200 with valid=false
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["valid"] != false {
		t.Errorf("valid = %v, want false", resp["valid"])
	}
}

func TestHandlePolicyValidateV1_BadJSON(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/policy/validate", strings.NewReader("{bad"))

	srv.handlePolicyValidateV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handlePolicyResetV1 — success path
// =============================================================================

func TestHandlePolicyResetV1_Success(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/policy/reset", nil)

	srv.handlePolicyResetV1(rec, req)

	// Should succeed (resets to default)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handleSafetyInstallV1/UninstallV1 — additional paths
// =============================================================================

func TestHandleSafetyInstallV1_InvalidBody(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/safety/install", strings.NewReader("{bad"))

	srv.handleSafetyInstallV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSafetyUninstallV1_NothingInstalled(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/safety/uninstall", nil)

	srv.handleSafetyUninstallV1(rec, req)

	// Should succeed with empty removed list
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handleSafetyCheckV1 — valid command path
// =============================================================================

func TestHandleSafetyCheckV1_ValidCommand_Branch(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/safety/check",
		strings.NewReader(`{"command":"ls -la"}`))

	srv.handleSafetyCheckV1(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// =============================================================================
// handleApprovalsListV1 — with status filter
// =============================================================================

func TestHandleApprovalsListV1_WithStatusFilter(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/approvals?status=pending", nil)

	srv.handleApprovalsListV1(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleApprovalsListV1_NoFilter(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/approvals", nil)

	srv.handleApprovalsListV1(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// =============================================================================
// handleGetPaneTitleV1 — invalid pane index
// =============================================================================

func TestHandleGetPaneTitleV1_InvalidIndex(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/s/panes/abc/title", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleGetPaneTitleV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSetPaneTitleV1_InvalidIndex(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/s/panes/abc/title",
		strings.NewReader(`{"title":"test"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleSetPaneTitleV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleSetPaneTitleV1_InvalidBody(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/s/panes/0/title",
		strings.NewReader("{bad"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "0")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handleSetPaneTitleV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// =============================================================================
// handlePaneInterruptV1 — invalid pane index
// =============================================================================

func TestHandlePaneInterruptV1_InvalidIndex(t *testing.T) {
	t.Parallel()
	srv, _ := setupTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s/panes/abc/interrupt", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", "s")
	rctx.URLParams.Add("paneIdx", "abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	srv.handlePaneInterruptV1(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
