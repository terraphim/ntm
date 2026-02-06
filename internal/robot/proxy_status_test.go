package robot

import (
	"testing"

	"github.com/Dicklesworthstone/ntm/internal/tools"
)

func TestBuildProxyStatusInfo_WithRouteDetails(t *testing.T) {
	status := &tools.ProxyStatus{
		Running:       true,
		Version:       "1.2.3",
		ListenPort:    8080,
		UptimeSeconds: 3600,
		RouteStats: []tools.ProxyRouteStatus{
			{
				Domain:   "api.openai.com",
				Upstream: "proxy-us:443",
				Active:   true,
				BytesIn:  10,
				BytesOut: 20,
				Requests: 2,
				Errors:   1,
			},
		},
		FailoverEvents: []tools.ProxyFailoverEvent{
			{
				Timestamp: "2026-02-06T17:00:00Z",
				Domain:    "api.openai.com",
				From:      "proxy-eu",
				To:        "proxy-us",
				Reason:    "healthcheck failed",
			},
		},
	}

	info := buildProxyStatusInfo(status, &tools.ProxyAvailability{Running: true})
	if !info.DaemonRunning {
		t.Error("DaemonRunning = false, want true")
	}
	if info.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", info.Version)
	}
	if info.ListenPort != 8080 {
		t.Errorf("ListenPort = %d, want 8080", info.ListenPort)
	}
	if info.UptimeSeconds != 3600 {
		t.Errorf("UptimeSeconds = %d, want 3600", info.UptimeSeconds)
	}
	if len(info.Routes) != 1 {
		t.Fatalf("Routes len = %d, want 1", len(info.Routes))
	}
	if info.Routes[0].Domain != "api.openai.com" {
		t.Errorf("Route[0].Domain = %q, want api.openai.com", info.Routes[0].Domain)
	}
	if len(info.FailoverEvents) != 1 {
		t.Fatalf("FailoverEvents len = %d, want 1", len(info.FailoverEvents))
	}
	if info.FailoverEvents[0].To != "proxy-us" {
		t.Errorf("FailoverEvents[0].To = %q, want proxy-us", info.FailoverEvents[0].To)
	}
}

func TestBuildProxyRouteInfos_CountFallback(t *testing.T) {
	status := &tools.ProxyStatus{
		Routes: 3,
	}
	routes := buildProxyRouteInfos(status)
	if len(routes) != 3 {
		t.Fatalf("len(routes) = %d, want 3", len(routes))
	}
}

func TestBuildProxyStatusInfo_UsesAvailabilityVersionWhenStatusMissing(t *testing.T) {
	info := buildProxyStatusInfo(nil, &tools.ProxyAvailability{
		Running: true,
		Version: tools.Version{Major: 0, Minor: 2, Patch: 1, Raw: "0.2.1"},
	})
	if !info.DaemonRunning {
		t.Error("DaemonRunning = false, want true")
	}
	if info.Version != "0.2.1" {
		t.Errorf("Version = %q, want 0.2.1", info.Version)
	}
}
