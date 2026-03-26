package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rtbhid/internal/portal"
	"rtbhid/internal/portal/state"
	"rtbhid/internal/syncer"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()

	dbPath := t.TempDir() + "/test.db"
	store, err := state.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	fetcher := syncer.NewFetcher("rtbhid-test/1.0")
	svc := portal.NewService(store, fetcher, 65500, 300, "")

	server, err := NewServer(svc, nil, AuthConfig{Enabled: false})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return server
}

func TestLookup_SingleIP(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	reqBody := map[string]any{
		"queries": []string{"192.168.1.1"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	results, ok := resp["results"].([]any)
	if !ok {
		t.Errorf("expected results array in response, got: %v", resp)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestLookup_CIDR(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	reqBody := map[string]any{
		"queries": []string{"192.168.1.0/24"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	results, ok := resp["results"].([]any)
	if !ok {
		t.Errorf("expected results array in response, got: %v", resp)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestLookup_BulkRequest(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	reqBody := map[string]any{
		"queries": []string{
			"192.168.1.1",
			"10.0.0.1",
			"172.16.0.1",
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	results, ok := resp["results"].([]any)
	if !ok {
		t.Errorf("expected results array in response, got: %v", resp)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestLookup_InvalidJSON(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := resp["error"]; !ok {
		t.Errorf("expected error field in response, got: %v", resp)
	}
}

func TestLookup_EmptyBody(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	reqBody := map[string]any{}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := resp["error"]; !ok {
		t.Errorf("expected error field in response, got: %v", resp)
	}
}

func TestLookup_TooManyQueries(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	queries := make([]string, 11)
	for i := 0; i < 11; i++ {
		queries[i] = "192.168.1." + string(rune('0'+i))
	}

	reqBody := map[string]any{
		"queries": queries,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if _, ok := resp["error"]; !ok {
		t.Errorf("expected error field in response, got: %v", resp)
	}
}

func TestLookup_MethodNotAllowed(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/lookup", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d: %s", http.StatusMethodNotAllowed, rr.Code, rr.Body.String())
	}
}
