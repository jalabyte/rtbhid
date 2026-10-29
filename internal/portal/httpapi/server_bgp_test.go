package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rtbhid/internal/portal/state"
)

func TestBGPOverviewAndMetrics(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	if err := server.svc.HandleRegister("agent-1", "router-01", "router-01", "embedded", "2.1.0"); err != nil {
		t.Fatalf("register: %v", err)
	}
	server.svc.HandleHeartbeat("agent-1", 10, 2, "running", "custom", []state.BGPSession{
		{Peer: "192.0.2.1", Name: "peer-a", RemoteAS: 7713, LocalAS: 65000, State: "Established", UptimeSec: 60, PrefixesIn: 4},
		{Peer: "192.0.2.2", RemoteAS: 9341, LocalAS: 65000, State: "OpenSent", Passive: true, LastError: "hold timer"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bgp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bgp status %d: %s", rr.Code, rr.Body.String())
	}
	var overview map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &overview); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if overview["agents"].(float64) != 1 || overview["established"].(float64) != 1 {
		t.Fatalf("unexpected overview: %v", overview)
	}

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("metrics status %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"rtbhid_bgp_agents 1",
		"rtbhid_bgp_sessions_total 2",
		"rtbhid_bgp_sessions_established 1",
		`rtbhid_bgp_session_up{agent="agent-1",peer="192.0.2.1",remote_as="7713",passive="false"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q:\n%s", want, body)
		}
	}

	// Optional bearer token protection.
	server.SetMetricsToken("test-token")
	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", rr.Code)
	}
}
