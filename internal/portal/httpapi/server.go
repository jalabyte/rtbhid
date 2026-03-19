// Package httpapi provides the portal REST API handlers.
package httpapi

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"rtbhid/internal/portal"
	"rtbhid/internal/portal/state"
)

//go:embed login.html
var loginHTML []byte

// Server wraps the portal Service and exposes a REST API for the React dashboard.
type Server struct {
	svc       *portal.Service
	auth      *authManager
	webAssets fs.FS
}

// NewServer creates a new portal API server. webAssets is the embedded React
// build output (embed.FS). Pass a nil fs.FS to skip SPA serving.
func NewServer(svc *portal.Service, webAssets fs.FS, authCfg AuthConfig) (*Server, error) {
	auth, err := newAuthManager(authCfg)
	if err != nil {
		return nil, err
	}
	return &Server{svc: svc, webAssets: webAssets, auth: auth}, nil
}

// Handler returns the full HTTP handler with CORS and logging middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Auth routes
	mux.HandleFunc("/login", s.handleLoginPage)
	mux.HandleFunc("/api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/api/v1/auth/me", s.handleAuthMe)

	// Health and status
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/status", s.handleStatus)

	// Agents
	mux.HandleFunc("/api/v1/agents", s.handleAgents)
	mux.HandleFunc("/api/v1/agents/", s.handleAgentByID)

	// Prefixes
	mux.HandleFunc("/api/v1/prefixes", s.handlePrefixes)
	mux.HandleFunc("/api/v1/prefixes/", s.handlePrefixByID)

	// Sources
	mux.HandleFunc("/api/v1/sources", s.handleSources)
	mux.HandleFunc("/api/v1/sources/", s.handleSourceByID)

	// Excludes
	mux.HandleFunc("/api/v1/excludes", s.handleExcludes)
	mux.HandleFunc("/api/v1/excludes/", s.handleExcludeByID)

	// Exclude Sources
	mux.HandleFunc("/api/v1/exclude-sources", s.handleExcludeSources)
	mux.HandleFunc("/api/v1/exclude-sources/", s.handleExcludeSourceByID)

	// Peers
	mux.HandleFunc("/api/v1/peers", s.handlePeers)
	mux.HandleFunc("/api/v1/peers/", s.handlePeerByID)

	// Events
	mux.HandleFunc("/api/v1/events", s.handleEvents)

	// Agent communication
	mux.HandleFunc("/api/v1/register", s.handleRegister)
	mux.HandleFunc("/api/v1/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/api/v1/config", s.handleConfig)

	// Fallback for "/"
	mux.HandleFunc("/", s.handleFallback)

	return withLogging(withCORS(s.auth.requireAuth(mux)))
}

// ---------------------------------------------------------------------------
// Health & Status
// ---------------------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents":             len(s.svc.ListAgents()),
		"prefixes":           len(s.svc.ListPrefixes()),
		"sources":            len(s.svc.ListSources()),
		"excludes":           len(s.svc.ListExcludes()),
		"peers":              len(s.svc.ListPeers()),
		"effective_prefixes": s.svc.GetEffectivePrefixCount(),
	})
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/login" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if !s.auth.cfg.Enabled {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if _, ok := s.auth.currentUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(loginHTML)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.auth.cfg.Enabled {
		writeError(w, http.StatusBadRequest, errors.New("auth is disabled"))
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.auth.checkCredentials(strings.TrimSpace(req.Username), req.Password) {
		writeError(w, http.StatusUnauthorized, errors.New("invalid username or password"))
		return
	}
	s.auth.issueSession(w, strings.TrimSpace(req.Username))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if s.auth.cfg.Enabled {
		s.auth.clearSession(w)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if !s.auth.cfg.Enabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":   false,
			"logged_in": true,
		})
		return
	}
	username, ok := s.auth.currentUser(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":   true,
		"logged_in": ok,
		"username":  username,
	})
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListAgents()})
}

func (s *Server) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		agent, ok := s.svc.GetAgent(id)
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("agent not found"))
			return
		}
		writeJSON(w, http.StatusOK, agent)
	case http.MethodDelete:
		if err := s.svc.DeleteAgent(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		methodNotAllowed(w)
	}
}

// ---------------------------------------------------------------------------
// Prefixes
// ---------------------------------------------------------------------------

func (s *Server) handlePrefixes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListPrefixes()})
	case http.MethodPost:
		var req struct {
			CIDR string `json:"cidr"`
			Note string `json:"note"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if req.CIDR == "" {
			writeError(w, http.StatusBadRequest, errors.New("cidr is required"))
			return
		}
		p, err := s.svc.CreatePrefix(req.CIDR, req.Note)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, p)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handlePrefixByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/prefixes/")
	if rest == "" {
		http.NotFound(w, r)
		return
	}

	// Check for /toggle suffix
	if strings.HasSuffix(rest, "/toggle") {
		idStr := strings.TrimSuffix(rest, "/toggle")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid prefix id"))
			return
		}
		if r.Method != http.MethodPut {
			methodNotAllowed(w)
			return
		}
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.svc.TogglePrefix(id, req.Enabled); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid prefix id"))
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := s.svc.DeletePrefix(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		methodNotAllowed(w)
	}
}

// ---------------------------------------------------------------------------
// Sources
// ---------------------------------------------------------------------------

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListSources()})
	case http.MethodPost:
		var src state.Source
		if err := decodeJSON(r, &src); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.svc.CreateSource(src); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, src)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleSourceByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/sources/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]

	// POST /api/v1/sources/:id/sync
	if len(parts) == 2 && parts[1] == "sync" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		s.svc.TriggerSync(id)
		writeJSON(w, http.StatusAccepted, map[string]any{"scheduled": true})
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := s.svc.DeleteSource(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		methodNotAllowed(w)
	}
}

// ---------------------------------------------------------------------------
// Excludes
// ---------------------------------------------------------------------------

func (s *Server) handleExcludes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListExcludes()})
	case http.MethodPost:
		var req struct {
			CIDR string `json:"cidr"`
			Note string `json:"note"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if req.CIDR == "" {
			writeError(w, http.StatusBadRequest, errors.New("cidr is required"))
			return
		}
		ex, err := s.svc.AddExclude(req.CIDR, req.Note)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, ex)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleExcludeByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/excludes/")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid exclude id"))
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := s.svc.DeleteExclude(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		methodNotAllowed(w)
	}
}

// ---------------------------------------------------------------------------
// Exclude Sources
// ---------------------------------------------------------------------------

func (s *Server) handleExcludeSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListExcludeSources()})
	case http.MethodPost:
		var src state.Source
		if err := decodeJSON(r, &src); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.svc.CreateExcludeSource(src); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, src)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleExcludeSourceByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/exclude-sources/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]

	// POST /api/v1/exclude-sources/:id/sync
	if len(parts) == 2 && parts[1] == "sync" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		s.svc.TriggerExcludeSourceSync(id)
		writeJSON(w, http.StatusAccepted, map[string]any{"scheduled": true})
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := s.svc.DeleteExcludeSource(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		methodNotAllowed(w)
	}
}

// ---------------------------------------------------------------------------
// Peers
// ---------------------------------------------------------------------------

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListPeers()})
	case http.MethodPost:
		var peer state.Peer
		if err := decodeJSON(r, &peer); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.svc.CreatePeer(peer); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, peer)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handlePeerByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/peers/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var peer state.Peer
		if err := decodeJSON(r, &peer); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		peer.ID = id
		if err := s.svc.UpdatePeer(peer); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, peer)
	case http.MethodDelete:
		if err := s.svc.DeletePeer(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
	default:
		methodNotAllowed(w)
	}
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListEvents(limit)})
}

// ---------------------------------------------------------------------------
// Agent Communication
// ---------------------------------------------------------------------------

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		AgentID     string `json:"agent_id"`
		Name        string `json:"name"`
		BirdVersion string `json:"bird_version"`
		Version     string `json:"version"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, errors.New("agent_id is required"))
		return
	}
	if err := s.svc.HandleRegister(req.AgentID, req.Name, req.BirdVersion, req.Version); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		var req struct {
			AgentID     string `json:"agent_id"`
			PrefixCount int    `json:"prefix_count"`
			PeerCount   int    `json:"peer_count"`
			BirdStatus  string `json:"bird_status"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		agentID = req.AgentID
		if agentID == "" {
			writeError(w, http.StatusBadRequest, errors.New("X-Agent-ID header or agent_id field is required"))
			return
		}
		s.svc.HandleHeartbeat(agentID, req.PrefixCount, req.PeerCount, req.BirdStatus)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	var req struct {
		PrefixCount int    `json:"prefix_count"`
		PeerCount   int    `json:"peer_count"`
		BirdStatus  string `json:"bird_status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.svc.HandleHeartbeat(agentID, req.PrefixCount, req.PeerCount, req.BirdStatus)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	agentID := r.URL.Query().Get("agentID")
	if agentID == "" {
		agentID = r.URL.Query().Get("agent_id")
	}
	if agentID == "" {
		writeError(w, http.StatusBadRequest, errors.New("agentID query parameter is required"))
		return
	}
	ch := s.svc.RegisterStream(agentID)
	defer s.svc.UnregisterStream(agentID)

	select {
	case update := <-ch:
		writeJSON(w, http.StatusOK, update)
	case <-r.Context().Done():
		writeError(w, http.StatusRequestTimeout, errors.New("context cancelled"))
	}
}

// ---------------------------------------------------------------------------
// Fallback
// ---------------------------------------------------------------------------

func (s *Server) handleFallback(w http.ResponseWriter, r *http.Request) {
	if s.webAssets == nil {
		http.NotFound(w, r)
		return
	}
	// Try to serve the file from the embedded filesystem
	f, err := s.webAssets.Open(strings.TrimPrefix(r.URL.Path, "/"))
	if err == nil {
		f.Close()
		http.FileServer(http.FS(s.webAssets)).ServeHTTP(w, r)
		return
	}
	// SPA fallback: serve index.html for any non-API, non-file route
	data, err := fs.ReadFile(s.webAssets, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("only one JSON object is allowed")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Agent-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("%s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
