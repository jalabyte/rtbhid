package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func postWhitelist(t *testing.T, handler http.Handler, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/whitelist", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestWhitelistAPIStrictPolicy(t *testing.T) {
	server := setupTestServer(t)
	handler := server.Handler()

	// Empty registry: manual adds are blocked by default.
	rr := postWhitelist(t, handler, map[string]any{"asn": 7713})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 with empty registry, got %d: %s", rr.Code, rr.Body.String())
	}

	if err := server.svc.Store().ReplaceIDASNRegistry([]uint32{7713}); err != nil {
		t.Fatalf("seed registry: %v", err)
	}

	rr = postWhitelist(t, handler, map[string]any{"asn": 7713, "name": "ID ISP"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for registered ID ASN, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = postWhitelist(t, handler, map[string]any{"asn": 65002})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-ID ASN, got %d: %s", rr.Code, rr.Body.String())
	}

	rr = postWhitelist(t, handler, map[string]any{"asn": 65002, "allow_non_id": true})
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 with allow_non_id, got %d: %s", rr.Code, rr.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/whitelist", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("whitelist GET %d", rr.Code)
	}
	var resp struct {
		Count           int  `json:"count"`
		StrictID        bool `json:"strict_id"`
		IDRegistryCount int  `json:"id_registry_count"`
		Items           []struct {
			ASN     uint32 `json:"asn"`
			Country string `json:"country"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 2 || !resp.StrictID || resp.IDRegistryCount != 1 {
		t.Fatalf("unexpected whitelist response: %+v", resp)
	}
	for _, item := range resp.Items {
		if item.ASN == 65002 && item.Country != "NON-ID" {
			t.Fatalf("expected NON-ID override marker, got %+v", item)
		}
	}
}
