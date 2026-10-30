package httpapi

import (
	_ "embed"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed install-agent.sh.tmpl
var installScriptTmpl string

var installScript = template.Must(template.New("install").Funcs(template.FuncMap{
	// sh single-quotes a value for the shell.
	"sh": func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" },
}).Parse(installScriptTmpl))

// AgentInstallConfig drives the one-line agent installer.
type AgentInstallConfig struct {
	// PortalURL is the gRPC address remote agents dial, e.g. agent.example.com:443.
	// Empty disables remote enrollment.
	PortalURL string
	PortalTLS bool
	// BinaryDir holds rtbhd-linux-amd64 / rtbhd-linux-arm64 for download.
	BinaryDir string
	LocalASN  int
}

func (s *Server) SetAgentInstall(cfg AgentInstallConfig) { s.agentInstall = cfg }

var validHost = regexp.MustCompile(`^[A-Za-z0-9.\-:\[\]]+$`)

// publicBaseURL is the portal URL as the browser (and so the installing
// host) sees it, behind Caddy or not.
func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if !validHost.MatchString(host) {
		host = "localhost"
	}
	return scheme + "://" + host
}

func (s *Server) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	_ = installScript.Execute(w, map[string]any{
		"BaseURL":   publicBaseURL(r),
		"PortalURL": s.agentInstall.PortalURL,
		"PortalTLS": s.agentInstall.PortalTLS,
		"LocalASN":  s.agentInstall.LocalASN,
	})
}

// handleAgentDownload serves the agent binary to holders of a valid token.
func (s *Server) handleAgentDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/download/")
	if name != "rtbhd-linux-amd64" && name != "rtbhd-linux-arm64" {
		http.NotFound(w, r)
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if !s.svc.ValidAgentToken(token) {
		writeError(w, http.StatusUnauthorized, errors.New("valid agent token required"))
		return
	}
	if s.agentInstall.BinaryDir == "" {
		writeError(w, http.StatusNotFound, errors.New("agent binaries are not published (RTBH_AGENT_BINARY_DIR)"))
		return
	}
	path := filepath.Join(s.agentInstall.BinaryDir, name)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, errors.New(name+" is not published on this portal"))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

func (s *Server) handleAgentTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		amd64, arm64 := false, false
		if dir := s.agentInstall.BinaryDir; dir != "" {
			_, err := os.Stat(filepath.Join(dir, "rtbhd-linux-amd64"))
			amd64 = err == nil
			_, err = os.Stat(filepath.Join(dir, "rtbhd-linux-arm64"))
			arm64 = err == nil
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items":       s.svc.ListAgentTokens(),
			"portal_url":  s.agentInstall.PortalURL,
			"install_url": publicBaseURL(r) + "/install-agent.sh",
			"binaries":    map[string]bool{"amd64": amd64, "arm64": arm64},
		})
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		t, plain, err := s.svc.CreateAgentToken(req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"token": plain, "item": t})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAgentTokenByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	if err := s.svc.RevokeAgentToken(strings.TrimPrefix(r.URL.Path, "/api/v1/agent-tokens/")); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
