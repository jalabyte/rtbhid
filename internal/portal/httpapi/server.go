// Package httpapi provides the portal REST API handlers.
package httpapi

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"rtbhid/internal/portal"
	"rtbhid/internal/portal/state"
)

//go:embed login.html
var loginHTML []byte

// Server wraps the portal Service and exposes a REST API for the React dashboard.
type Server struct {
	svc          *portal.Service
	auth         *authManager
	webAssets    fs.FS
	metricsToken string
	agentInstall AgentInstallConfig
}

// SetMetricsToken requires a bearer token (or ?token=) on GET /metrics. An empty
// token keeps the endpoint public, which is the default for trusted networks.
func (s *Server) SetMetricsToken(token string) {
	s.metricsToken = strings.TrimSpace(token)
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
	mux.HandleFunc("/api/v1/agent-tokens", s.handleAgentTokens)
	mux.HandleFunc("/api/v1/agent-tokens/", s.handleAgentTokenByID)
	mux.HandleFunc("/install-agent.sh", s.handleInstallScript)
	mux.HandleFunc("/download/", s.handleAgentDownload)

	// Prefixes
	mux.HandleFunc("/api/v1/prefixes", s.handlePrefixes)
	mux.HandleFunc("/api/v1/prefixes/bulk", s.handlePrefixBulk)
	mux.HandleFunc("/api/v1/prefixes/", s.handlePrefixByID)

	// Sources
	mux.HandleFunc("/api/v1/sources", s.handleSources)
	mux.HandleFunc("/api/v1/sources/", s.handleSourceByID)

	// Excludes
	mux.HandleFunc("/api/v1/excludes", s.handleExcludes)
	mux.HandleFunc("/api/v1/excludes/bulk", s.handleExcludeBulk)
	mux.HandleFunc("/api/v1/excludes/", s.handleExcludeByID)

	// Exclude Sources
	mux.HandleFunc("/api/v1/exclude-sources", s.handleExcludeSources)
	mux.HandleFunc("/api/v1/exclude-sources/prefixes", s.handleExcludeSourcePrefixes)
	mux.HandleFunc("/api/v1/exclude-sources/", s.handleExcludeSourceByID)

	// Peers
	mux.HandleFunc("/api/v1/peers", s.handlePeers)
	mux.HandleFunc("/api/v1/peers/bulk", s.handlePeerBulk)
	mux.HandleFunc("/api/v1/peers/", s.handlePeerByID)

	// ASN whitelist for auto-accepted BGP peers
	mux.HandleFunc("/api/v1/whitelist", s.handleWhitelist)
	mux.HandleFunc("/api/v1/whitelist/", s.handleWhitelistByID)

	// BGP monitoring
	mux.HandleFunc("/api/v1/bgp", s.handleBGP)
	mux.HandleFunc("/metrics", s.handleMetrics)

	// Events
	mux.HandleFunc("/api/v1/events", s.handleEvents)

	// Lookup
	mux.HandleFunc("/api/v1/lookup", s.handleLookup)

	// Prefix builder
	mux.HandleFunc("/api/v1/prefix-builder", s.handlePrefixBuilder)

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
	bgp := s.svc.BGPOverview()
	writeJSON(w, http.StatusOK, map[string]any{
		"agents":             len(s.svc.ListAgents()),
		"prefixes":           len(s.svc.ListPrefixes()),
		"sources":            len(s.svc.ListSources()),
		"excludes":           len(s.svc.ListExcludes()),
		"peers":              len(s.svc.ListPeers()),
		"whitelist":          s.svc.GetWhitelistCount(),
		"auto_accept":        s.svc.AutoAcceptEnabled(),
		"bgp_sessions":       bgp.Sessions,
		"bgp_established":    bgp.Established,
		"effective_prefixes": s.svc.GetEffectivePrefixCount(),
		"local_asn":          s.svc.GetLocalASN(),
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

func (s *Server) handlePrefixBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Items []string `json:"items"`
		Note  string   `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("items array is required"))
		return
	}
	created, skipped, invalid, err := s.svc.BulkCreatePrefixes(req.Items, strings.TrimSpace(req.Note))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"created": created,
		"skipped": skipped,
		"invalid": invalid,
	})
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

	// GET /api/v1/sources/:id/prefixes?page=1&limit=100
	if len(parts) == 2 && parts[1] == "prefixes" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		page, limit := parsePagination(r)
		prefixes, total, err := s.svc.GetSourcePrefixes(id, page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": prefixes,
			"total": total,
			"page":  page,
			"limit": limit,
		})
		return
	}

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

	// PUT /api/v1/sources/:id/toggle
	if len(parts) == 2 && parts[1] == "toggle" {
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
		if err := s.svc.ToggleSource(id, req.Enabled); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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

func (s *Server) handleExcludeBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Items []string `json:"items"`
		Note  string   `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("items array is required"))
		return
	}
	created, skipped, invalid, err := s.svc.BulkCreateExcludes(req.Items, strings.TrimSpace(req.Note))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"created": created,
		"skipped": skipped,
		"invalid": invalid,
	})
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

func (s *Server) handleExcludeSourcePrefixes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	offset := 0
	limit := 100

	if raw := r.URL.Query().Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n >= 0 {
			offset = n
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	items, total, err := s.svc.ListExcludeSourcePrefixes(offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

func (s *Server) handleExcludeSourceByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/exclude-sources/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]

	// GET /api/v1/exclude-sources/:id/prefixes?page=1&limit=100
	if len(parts) == 2 && parts[1] == "prefixes" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		page, limit := parsePagination(r)
		prefixes, total, err := s.svc.GetExcludeSourcePrefixes(id, page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": prefixes,
			"total": total,
			"page":  page,
			"limit": limit,
		})
		return
	}

	// PUT /api/v1/exclude-sources/:id/toggle
	if len(parts) == 2 && parts[1] == "toggle" {
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
		if err := s.svc.ToggleExcludeSource(id, req.Enabled); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

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

func (s *Server) handlePeerBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Items []portal.PeerBulkInput `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no peers provided"))
		return
	}
	if err := s.svc.BulkCreatePeers(req.Items); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"created": len(req.Items),
	})
}

func (s *Server) handlePeerByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/peers/")
	if rest == "" {
		http.NotFound(w, r)
		return
	}

	// Check for /toggle suffix
	if strings.HasSuffix(rest, "/toggle") {
		id := strings.TrimSuffix(rest, "/toggle")
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
		if err := s.svc.TogglePeer(id, req.Enabled); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	id := rest
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
// ASN whitelist
// ---------------------------------------------------------------------------

func (s *Server) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"items":             s.svc.ListWhitelistASNs(),
			"count":             s.svc.GetWhitelistCount(),
			"auto_accept":       s.svc.AutoAcceptEnabled(),
			"strict_id":         s.svc.StrictIDWhitelist(),
			"id_registry_count": s.svc.IDASNRegistryCount(),
		})
	case http.MethodPost:
		var req struct {
			ASN        uint32 `json:"asn"`
			Name       string `json:"name"`
			AllowNonID bool   `json:"allow_non_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if req.ASN == 0 {
			writeError(w, http.StatusBadRequest, errors.New("asn is required"))
			return
		}
		entry, err := s.svc.AddWhitelistASN(req.ASN, strings.TrimSpace(req.Name), req.AllowNonID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleWhitelistByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/whitelist/")
	if rest == "" {
		http.NotFound(w, r)
		return
	}

	switch rest {
	case "import":
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		created, total, err := s.svc.ImportIndonesianASNs(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created":           created,
			"total":             total,
			"count":             s.svc.GetWhitelistCount(),
			"id_registry_count": s.svc.IDASNRegistryCount(),
			"warnings":          s.svc.IDASNWarnings(),
		})
		return
	case "auto-accept":
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
		s.svc.SetAutoAccept(req.Enabled)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "enabled": req.Enabled})
		return
	}

	// /api/v1/whitelist/{id}/toggle
	if strings.HasSuffix(rest, "/toggle") {
		id := strings.TrimSuffix(rest, "/toggle")
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
		if err := s.svc.ToggleWhitelistASN(id, req.Enabled); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}

	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	if err := s.svc.DeleteWhitelistASN(rest); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ---------------------------------------------------------------------------
// BGP monitoring
// ---------------------------------------------------------------------------

func (s *Server) handleBGP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, s.svc.BGPOverview())
}

// bgpStaleAfter is how long after the last agent report a session is considered
// stale in metrics.
const bgpStaleAfter = 60 * time.Second

func sessionStale(updated time.Time) bool {
	return updated.IsZero() || time.Since(updated) > bgpStaleAfter
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	if s.metricsToken != "" {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" {
			got = r.URL.Query().Get("token")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.metricsToken)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, errors.New("invalid metrics token"))
			return
		}
	}
	overview := s.svc.BGPOverview()
	staleSessions := 0
	for _, sess := range overview.Items {
		if sessionStale(sess.UpdatedAt) {
			staleSessions++
		}
	}

	type agentAgg struct {
		engine      string
		sessions    int
		established int
		prefixesIn  int
		prefixesOut int
	}
	agents := make(map[string]*agentAgg)
	for _, sess := range overview.Items {
		agg := agents[sess.AgentID]
		if agg == nil {
			agg = &agentAgg{engine: overview.Engines[sess.AgentID]}
			agents[sess.AgentID] = agg
		}
		agg.sessions++
		if strings.EqualFold(sess.State, "established") {
			agg.established++
			agg.prefixesIn += sess.PrefixesIn
			agg.prefixesOut += sess.PrefixesOut
		}
	}
	names := make([]string, 0, len(agents))
	for id := range agents {
		names = append(names, id)
	}
	sort.Strings(names)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	var b strings.Builder
	b.WriteString("# HELP rtbhid_bgp_agents Number of agents reporting BGP telemetry.\n")
	b.WriteString("# TYPE rtbhid_bgp_agents gauge\n")
	fmt.Fprintf(&b, "rtbhid_bgp_agents %d\n", overview.Agents)
	b.WriteString("# HELP rtbhid_bgp_sessions_total Number of BGP sessions reported by agents.\n")
	b.WriteString("# TYPE rtbhid_bgp_sessions_total gauge\n")
	fmt.Fprintf(&b, "rtbhid_bgp_sessions_total %d\n", overview.Sessions)
	b.WriteString("# HELP rtbhid_bgp_sessions_established Number of established BGP sessions.\n")
	b.WriteString("# TYPE rtbhid_bgp_sessions_established gauge\n")
	fmt.Fprintf(&b, "rtbhid_bgp_sessions_established %d\n", overview.Established)
	b.WriteString("# HELP rtbhid_bgp_prefixes_received Number of prefixes received from established peers.\n")
	b.WriteString("# TYPE rtbhid_bgp_prefixes_received gauge\n")
	fmt.Fprintf(&b, "rtbhid_bgp_prefixes_received %d\n", overview.PrefixesIn)
	b.WriteString("# HELP rtbhid_bgp_prefixes_announced Prefixes announced, summed over established sessions.\n")
	b.WriteString("# TYPE rtbhid_bgp_prefixes_announced gauge\n")
	fmt.Fprintf(&b, "rtbhid_bgp_prefixes_announced %d\n", overview.PrefixesOut)
	b.WriteString("# HELP rtbhid_bgp_sessions_stale Number of sessions whose agent report is older than 60s.\n")
	b.WriteString("# TYPE rtbhid_bgp_sessions_stale gauge\n")
	fmt.Fprintf(&b, "rtbhid_bgp_sessions_stale %d\n", staleSessions)
	b.WriteString("# HELP rtbhid_agent_bgp_sessions BGP sessions per agent.\n")
	b.WriteString("# TYPE rtbhid_agent_bgp_sessions gauge\n")
	for _, id := range names {
		agg := agents[id]
		fmt.Fprintf(&b, "rtbhid_agent_bgp_sessions{agent=\"%s\",engine=\"%s\"} %d\n", escapeLabel(id), escapeLabel(agg.engine), agg.sessions)
		fmt.Fprintf(&b, "rtbhid_agent_bgp_sessions_established{agent=\"%s\",engine=\"%s\"} %d\n", escapeLabel(id), escapeLabel(agg.engine), agg.established)
		fmt.Fprintf(&b, "rtbhid_agent_bgp_prefixes_received{agent=\"%s\",engine=\"%s\"} %d\n", escapeLabel(id), escapeLabel(agg.engine), agg.prefixesIn)
		fmt.Fprintf(&b, "rtbhid_agent_bgp_prefixes_announced{agent=\"%s\",engine=\"%s\"} %d\n", escapeLabel(id), escapeLabel(agg.engine), agg.prefixesOut)
	}
	b.WriteString("# HELP rtbhid_bgp_session_up Whether a BGP session is established (1) or not (0).\n")
	b.WriteString("# TYPE rtbhid_bgp_session_up gauge\n")
	b.WriteString("# HELP rtbhid_bgp_session_stale Whether the last report for this session is older than 60s.\n")
	b.WriteString("# TYPE rtbhid_bgp_session_stale gauge\n")
	for _, sess := range overview.Items {
		up := 0
		if strings.EqualFold(sess.State, "established") {
			up = 1
		}
		stale := 0
		if sessionStale(sess.UpdatedAt) {
			stale = 1
		}
		fmt.Fprintf(&b, "rtbhid_bgp_session_up{agent=\"%s\",peer=\"%s\",remote_as=\"%s\",passive=\"%s\"} %d\n",
			escapeLabel(sess.AgentID), escapeLabel(sess.Peer), strconv.FormatUint(uint64(sess.RemoteAS), 10), strconv.FormatBool(sess.Passive), up)
		fmt.Fprintf(&b, "rtbhid_bgp_session_stale{agent=\"%s\",peer=\"%s\"} %d\n",
			escapeLabel(sess.AgentID), escapeLabel(sess.Peer), stale)
	}
	_, _ = w.Write([]byte(b.String()))
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
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
		Hostname    string `json:"hostname"`
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
	if err := s.svc.HandleRegister(req.AgentID, req.Name, req.Hostname, req.BirdVersion, req.Version); err != nil {
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
	var req struct {
		AgentID     string             `json:"agent_id"`
		PrefixCount int                `json:"prefix_count"`
		PeerCount   int                `json:"peer_count"`
		BirdStatus  string             `json:"bird_status"`
		BGPEngine   string             `json:"bgp_engine"`
		BGPSessions []state.BGPSession `json:"bgp_sessions"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		agentID = req.AgentID
	}
	if agentID == "" {
		writeError(w, http.StatusBadRequest, errors.New("X-Agent-ID header or agent_id field is required"))
		return
	}
	s.svc.HandleHeartbeat(agentID, req.PrefixCount, req.PeerCount, req.BirdStatus, req.BGPEngine, req.BGPSessions)
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

func parsePagination(r *http.Request) (page int, limit int) {
	page = 1
	limit = 100
	if raw := r.URL.Query().Get("page"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			page = n
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 10000 {
			limit = n
		}
	}
	return page, limit
}

// ---------------------------------------------------------------------------
// Lookup
// ---------------------------------------------------------------------------

func (s *Server) handleLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		Queries []string `json:"queries"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if len(req.Queries) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("queries array is required"))
		return
	}

	if len(req.Queries) > 10 {
		writeError(w, http.StatusBadRequest, errors.New("maximum 10 queries allowed"))
		return
	}

	results, err := s.svc.LookupIPWithASN(r.Context(), req.Queries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handlePrefixBuilder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req struct {
		ASN     string   `json:"asn"`
		IPv4    *bool    `json:"ipv4"`
		IPv6    *bool    `json:"ipv6"`
		Sources []string `json:"sources"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ipv4 := true
	ipv6 := true
	if req.IPv4 != nil {
		ipv4 = *req.IPv4
	}
	if req.IPv6 != nil {
		ipv6 = *req.IPv6
	}

	prefixes, err := s.svc.BuildPrefixesFromASNWithSources(r.Context(), req.ASN, ipv4, ipv6, req.Sources)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, portal.ErrInvalidASN) || errors.Is(err, portal.ErrNoAddressFamily) || errors.Is(err, portal.ErrInvalidSource) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"asn":      req.ASN,
		"count":    len(prefixes),
		"prefixes": prefixes,
	})
}
