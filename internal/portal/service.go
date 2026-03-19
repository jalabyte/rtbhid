package portal

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	pb "rtbhid/gen/rtbhid"
	"rtbhid/internal/portal/state"
	"rtbhid/internal/syncer"
)

const heartbeatInterval = 15

type Service struct {
	store    *state.Store
	fetcher  *syncer.Fetcher
	localASN int
	tickSec  int

	// Agent config streams: agentID -> channel
	mu      sync.RWMutex
	streams map[string]chan *pb.ConfigUpdate
	version int64
}

func NewService(store *state.Store, fetcher *syncer.Fetcher, localASN int, tickSec int) *Service {
	return &Service{
		store:    store,
		fetcher:  fetcher,
		localASN: localASN,
		tickSec:  tickSec,
		streams:  make(map[string]chan *pb.ConfigUpdate),
	}
}

// Start begins the source sync scheduler goroutine.
func (s *Service) Start(ctx context.Context) {
	go s.syncScheduler(ctx)
}

// ---------------------------------------------------------------------------
// Agent stream management
// ---------------------------------------------------------------------------

// RegisterStream creates a buffered channel for the agent and sends the
// current config immediately. Returns the channel the agent should read from.
func (s *Service) RegisterStream(agentID string) chan *pb.ConfigUpdate {
	ch := make(chan *pb.ConfigUpdate, 10)

	s.mu.Lock()
	s.streams[agentID] = ch
	s.mu.Unlock()

	// Send current config immediately
	s.pushConfigToStream(agentID, ch)

	log.Printf("agent %s stream registered", agentID)
	return ch
}

// UnregisterStream removes the agent's channel and closes it.
func (s *Service) UnregisterStream(agentID string) {
	s.mu.Lock()
	ch, ok := s.streams[agentID]
	if ok {
		delete(s.streams, agentID)
	}
	s.mu.Unlock()

	if ok {
		close(ch)
		log.Printf("agent %s stream unregistered", agentID)
	}
}

// PushConfigToAll computes the effective config and pushes to all connected agents.
func (s *Service) PushConfigToAll() {
	s.mu.RLock()
	agents := make(map[string]chan *pb.ConfigUpdate, len(s.streams))
	for id, ch := range s.streams {
		agents[id] = ch
	}
	s.mu.RUnlock()

	for agentID, ch := range agents {
		s.pushConfigToStream(agentID, ch)
	}
}

// pushConfigToStream sends a config update to a single agent's channel.
func (s *Service) pushConfigToStream(agentID string, ch chan *pb.ConfigUpdate) {
	update := s.buildConfigUpdate()
	select {
	case ch <- update:
		log.Printf("pushed config v%d to agent %s (prefixes=%d, peers=%d)",
			update.Version, agentID, len(update.Prefixes), len(update.Peers))
	default:
		log.Printf("WARNING: agent %s config channel full, dropping update", agentID)
	}
}

// ---------------------------------------------------------------------------
// Config computation
// ---------------------------------------------------------------------------

// buildConfigUpdate reads effective prefixes and enabled peers from the store
// and builds a ConfigUpdate with an incremented version counter.
func (s *Service) buildConfigUpdate() *pb.ConfigUpdate {
	s.mu.Lock()
	s.version++
	ver := s.version
	s.mu.Unlock()

	prefixes := s.store.EffectivePrefixes()

	// Collect enabled peers
	allPeers := s.store.ListPeers()
	var peers []*pb.PeerConfig
	for _, p := range allPeers {
		if !p.Enabled {
			continue
		}
		peers = append(peers, &pb.PeerConfig{
			Name:             p.Name,
			NeighborIp:       p.NeighborIP,
			RemoteAs:         uint32(p.RemoteAS),
			LocalAs:          uint32(p.LocalAS),
			Multihop:         uint32(p.Multihop),
			StdCommunities:   p.StdCommunities,
			LargeCommunities: p.LargeCommunities,
		})
	}

	return &pb.ConfigUpdate{
		Prefixes: prefixes,
		Peers:    peers,
		Version:  ver,
	}
}

// ---------------------------------------------------------------------------
// CRUD wrappers — each triggers a config push after mutation
// ---------------------------------------------------------------------------

func (s *Service) CreatePrefix(cidr, note string) (state.Prefix, error) {
	p, err := s.store.CreatePrefix(state.Prefix{CIDR: cidr, Note: note, Enabled: true})
	if err != nil {
		return state.Prefix{}, err
	}
	_ = s.store.AddEvent("prefix.create", fmt.Sprintf("created prefix %s", cidr), "")
	s.PushConfigToAll()
	return p, nil
}

func (s *Service) DeletePrefix(id int64) error {
	if err := s.store.DeletePrefix(id); err != nil {
		return err
	}
	_ = s.store.AddEvent("prefix.delete", fmt.Sprintf("deleted prefix id=%d", id), "")
	s.PushConfigToAll()
	return nil
}

func (s *Service) TogglePrefix(id int64, enabled bool) error {
	if err := s.store.TogglePrefix(id, enabled); err != nil {
		return err
	}
	_ = s.store.AddEvent("prefix.toggle", fmt.Sprintf("toggled prefix id=%d enabled=%v", id, enabled), "")
	s.PushConfigToAll()
	return nil
}

func (s *Service) CreatePeer(peer state.Peer) error {
	if err := s.store.CreatePeer(peer); err != nil {
		return err
	}
	_ = s.store.AddEvent("peer.create", fmt.Sprintf("created peer %s", peer.Name), "")
	s.PushConfigToAll()
	return nil
}

func (s *Service) UpdatePeer(peer state.Peer) error {
	if err := s.store.UpdatePeer(peer); err != nil {
		return err
	}
	_ = s.store.AddEvent("peer.update", fmt.Sprintf("updated peer %s", peer.Name), "")
	s.PushConfigToAll()
	return nil
}

func (s *Service) DeletePeer(id string) error {
	if err := s.store.DeletePeer(id); err != nil {
		return err
	}
	_ = s.store.AddEvent("peer.delete", fmt.Sprintf("deleted peer id=%s", id), "")
	s.PushConfigToAll()
	return nil
}

func (s *Service) CreateSource(src state.Source) error {
	if src.ID == "" {
		src.ID = newResourceID("src")
	}
	if err := s.store.CreateSource(src); err != nil {
		return err
	}
	_ = s.store.AddEvent("source.create", fmt.Sprintf("created source %s", src.Name), "")
	return nil
}

func (s *Service) DeleteSource(id string) error {
	if err := s.store.DeleteSource(id); err != nil {
		return err
	}
	_ = s.store.AddEvent("source.delete", fmt.Sprintf("deleted source id=%s", id), "")
	s.PushConfigToAll()
	return nil
}

func (s *Service) AddExclude(cidr, note string) (state.Exclude, error) {
	ex, err := s.store.CreateExclude(state.Exclude{CIDR: cidr, Note: note})
	if err != nil {
		return state.Exclude{}, err
	}
	_ = s.store.AddEvent("exclude.create", fmt.Sprintf("created exclude %s", cidr), "")
	s.PushConfigToAll()
	return ex, nil
}

func (s *Service) DeleteExclude(id int64) error {
	if err := s.store.DeleteExclude(id); err != nil {
		return err
	}
	_ = s.store.AddEvent("exclude.delete", fmt.Sprintf("deleted exclude id=%d", id), "")
	s.PushConfigToAll()
	return nil
}

// ---------------------------------------------------------------------------
// Exclude Source CRUD
// ---------------------------------------------------------------------------

func (s *Service) CreateExcludeSource(src state.Source) error {
	if src.ID == "" {
		src.ID = newResourceID("xsrc")
	}
	if err := s.store.CreateExcludeSource(src); err != nil {
		return err
	}
	_ = s.store.AddEvent("exclude_source.create", fmt.Sprintf("created exclude source %s", src.Name), "")
	return nil
}

func (s *Service) DeleteExcludeSource(id string) error {
	if err := s.store.DeleteExcludeSource(id); err != nil {
		return err
	}
	_ = s.store.AddEvent("exclude_source.delete", fmt.Sprintf("deleted exclude source id=%s", id), "")
	s.PushConfigToAll()
	return nil
}

func (s *Service) TriggerExcludeSourceSync(sourceID string) {
	src, ok := s.store.GetExcludeSource(sourceID)
	if !ok {
		return
	}
	go s.syncExcludeSource(src)
}

func (s *Service) syncExcludeSource(src state.Source) {
	log.Printf("syncing exclude source %s (%s)", src.Name, src.URL)
	cidrs, err := s.fetcher.Fetch(context.Background(), src.URL, src.Interval)
	if err != nil {
		log.Printf("sync error for exclude source %s: %v", src.Name, err)
		_ = s.store.AddEvent("exclude_source.sync_error", fmt.Sprintf("sync failed for %s: %v", src.Name, err), "")
		return
	}
	if err := s.store.ReplaceExcludeSourcePrefixes(src.ID, cidrs); err != nil {
		log.Printf("sync replace error for exclude source %s: %v", src.Name, err)
		return
	}
	if err := s.store.SetExcludeSourceSyncResult(src.ID, len(cidrs)); err != nil {
		log.Printf("sync result update error for exclude source %s: %v", src.Name, err)
	}
	_ = s.store.AddEvent("exclude_source.sync", fmt.Sprintf("synced %s: %d excludes", src.Name, len(cidrs)), "")
	log.Printf("synced exclude source %s: %d excludes", src.Name, len(cidrs))
	s.PushConfigToAll()
}

// ---------------------------------------------------------------------------
// Pass-through list methods
// ---------------------------------------------------------------------------

func (s *Service) ListAgents() []state.Agent         { return s.store.ListAgents() }
func (s *Service) GetAgent(id string) (state.Agent, bool) { return s.store.GetAgent(id) }
func (s *Service) DeleteAgent(id string) error {
	if err := s.store.DeleteAgent(id); err != nil {
		return err
	}
	s.UnregisterStream(id)
	return nil
}
func (s *Service) ListPrefixes() []state.Prefix       { return s.store.ListPrefixes() }
func (s *Service) ListSources() []state.Source         { return s.store.ListSources() }
func (s *Service) ListExcludes() []state.Exclude               { return s.store.ListExcludes() }
func (s *Service) ListExcludeSources() []state.Source           { return s.store.ListExcludeSources() }
func (s *Service) ListPeers() []state.Peer                     { return s.store.ListPeers() }
func (s *Service) ListEvents(limit int) []state.Event  { return s.store.ListEvents(limit) }

// ---------------------------------------------------------------------------
// Source sync
// ---------------------------------------------------------------------------

// TriggerSync forces a sync for a specific source in a goroutine.
func (s *Service) TriggerSync(sourceID string) {
	src, ok := s.store.GetSource(sourceID)
	if !ok {
		log.Printf("sync: source %s not found", sourceID)
		return
	}
	go s.syncSource(src)
}

// syncSource fetches URLs, parses prefixes, replaces in store, and logs an event.
func (s *Service) syncSource(src state.Source) {
	log.Printf("syncing source %s (%s)", src.Name, src.URL)

	cidrs, err := s.fetcher.Fetch(context.Background(), src.URL, src.Interval)
	if err != nil {
		log.Printf("sync error for source %s: %v", src.Name, err)
		_ = s.store.AddEvent("source.sync_error", fmt.Sprintf("sync failed for %s: %v", src.Name, err), "")
		return
	}

	if err := s.store.ReplaceSourcePrefixes(src.ID, cidrs); err != nil {
		log.Printf("sync replace error for source %s: %v", src.Name, err)
		_ = s.store.AddEvent("source.sync_error", fmt.Sprintf("replace failed for %s: %v", src.Name, err), "")
		return
	}

	if err := s.store.SetSourceSyncResult(src.ID, len(cidrs)); err != nil {
		log.Printf("sync result update error for source %s: %v", src.Name, err)
	}

	_ = s.store.AddEvent("source.sync", fmt.Sprintf("synced %s: %d prefixes", src.Name, len(cidrs)), "")
	log.Printf("synced source %s: %d prefixes", src.Name, len(cidrs))

	// Push updated config to all agents since effective prefixes may have changed
	s.PushConfigToAll()
}

// syncScheduler ticks every tickSec seconds and syncs sources that are due.
func (s *Service) syncScheduler(ctx context.Context) {
	if s.tickSec <= 0 {
		s.tickSec = 300 // default 5 minutes
	}
	ticker := time.NewTicker(time.Duration(s.tickSec) * time.Second)
	defer ticker.Stop()

	// Run an initial sync on startup
	s.syncDueSources()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncDueSources()
		}
	}
}

// syncDueSources syncs all enabled sources whose interval has elapsed since last sync.
func (s *Service) syncDueSources() {
	now := time.Now().UTC()

	for _, src := range s.store.ListSources() {
		if !src.Enabled || src.Interval <= 0 {
			continue
		}
		if src.LastSync != nil && now.Sub(*src.LastSync) < time.Duration(src.Interval)*time.Second {
			continue
		}
		go s.syncSource(src)
	}

	for _, src := range s.store.ListExcludeSources() {
		if !src.Enabled || src.Interval <= 0 {
			continue
		}
		if src.LastSync != nil && now.Sub(*src.LastSync) < time.Duration(src.Interval)*time.Second {
			continue
		}
		go s.syncExcludeSource(src)
	}
}

// ---------------------------------------------------------------------------
// Heartbeat and registration
// ---------------------------------------------------------------------------

// HandleHeartbeat updates agent heartbeat in store.
func (s *Service) HandleHeartbeat(agentID string, prefixCount, peerCount int, birdStatus string) {
	_ = s.store.UpdateAgentHeartbeat(agentID, birdStatus, prefixCount, peerCount)
}

// HandleRegister registers an agent in the store.
func (s *Service) HandleRegister(agentID, name, birdVersion, version string) error {
	return s.store.RegisterAgent(state.Agent{
		ID:          agentID,
		Name:        name,
		BirdVersion: birdVersion,
		Status:      "online",
		LastSeen:    timePtr(time.Now().UTC()),
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newResourceID generates a short random ID like "src_a1b2c3d4".
func newResourceID(kind string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%x", kind, b)
}

func timePtr(t time.Time) *time.Time { return &t }
