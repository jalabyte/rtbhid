package httpapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"rtbhid/internal/portal"
)

// Gateway provides HTTP handler functions for the agent-facing REST API.
type Gateway struct {
	svc *portal.Service
}

// NewGateway creates a new Gateway wrapping the portal service.
func NewGateway(svc *portal.Service) *Gateway {
	return &Gateway{svc: svc}
}

// Register handles POST /api/v1/register.
func (g *Gateway) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req struct {
		AgentID     string `json:"agent_id"`
		Name        string `json:"name"`
		Hostname    string `json:"hostname"`
		BirdVersion string `json:"bird_version"`
		Version     string `json:"version"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" || req.Name == "" {
		http.Error(w, "agent_id and name are required", http.StatusBadRequest)
		return
	}

	if err := g.svc.HandleRegister(req.AgentID, req.Name, req.Hostname, req.BirdVersion, req.Version); err != nil {
		if strings.Contains(err.Error(), "already registered") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "agent already registered",
			})
			return
		}
		log.Printf("register error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("agent %s (%s) registered via HTTP", req.AgentID, req.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":            true,
		"heartbeat_interval": 30,
	})
}

// Heartbeat handles POST /api/v1/heartbeat.
func (g *Gateway) Heartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		agentID = r.URL.Query().Get("agentID")
	}
	if agentID == "" {
		http.Error(w, "missing agent ID", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req struct {
		BirdStatus  string `json:"bird_status"`
		PrefixCount int    `json:"prefix_count"`
		PeerCount   int    `json:"peer_count"`
		BGPEngine   string `json:"bgp_engine"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	g.svc.HandleHeartbeat(agentID, req.PrefixCount, req.PeerCount, req.BirdStatus, req.BGPEngine, nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Config handles GET /api/v1/config?agentID=<id>.
func (g *Gateway) Config(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agentID := r.URL.Query().Get("agentID")
	if agentID == "" {
		http.Error(w, "missing agentID parameter", http.StatusBadRequest)
		return
	}

	ch := g.svc.RegisterStream(agentID)
	defer g.svc.UnregisterStream(agentID)

	update := <-ch

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(update)
}
