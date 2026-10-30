package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentEnrollment(t *testing.T) {
	server := setupTestServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rtbhd-linux-amd64"), []byte("BIN"), 0o755); err != nil {
		t.Fatal(err)
	}
	server.SetAgentInstall(AgentInstallConfig{PortalURL: "agent.example.net:443", PortalTLS: true, BinaryDir: dir, LocalASN: 65010})
	h := server.Handler()
	do := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	rr := do(http.MethodPost, "/api/v1/agent-tokens", `{"name":"edge-1"}`, "")
	if rr.Code != http.StatusCreated {
		t.Fatalf("create token: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		Token string              `json:"token"`
		Item  struct{ ID string } `json:"item"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	script := do(http.MethodGet, "/install-agent.sh", "", "").Body.String()
	if !strings.Contains(script, "PORTAL_URL='agent.example.net:443'") || !strings.Contains(script, "ASN=65010") {
		t.Fatalf("script missing baked values:\n%s", script[:400])
	}

	if rr := do(http.MethodGet, "/download/rtbhd-linux-amd64", "", "wrong"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("download without valid token: %d", rr.Code)
	}
	if rr := do(http.MethodGet, "/download/rtbhd-linux-amd64", "", created.Token); rr.Code != 200 || rr.Body.String() != "BIN" {
		t.Fatalf("download with token: %d %q", rr.Code, rr.Body.String())
	}

	svc := server.svc
	if err := svc.AuthorizeAgent(created.Token, "agent-a"); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := svc.AuthorizeAgent(created.Token, "agent-a"); err != nil {
		t.Fatalf("same agent again: %v", err)
	}
	if err := svc.AuthorizeAgent(created.Token, "agent-b"); err == nil {
		t.Fatal("token reused by a second agent was accepted")
	}
	if rr := do(http.MethodDelete, "/api/v1/agent-tokens/"+created.Item.ID, "", ""); rr.Code != 200 {
		t.Fatalf("revoke: %d", rr.Code)
	}
	if err := svc.AuthorizeAgent(created.Token, "agent-a"); err == nil {
		t.Fatal("revoked token still accepted")
	}
}
