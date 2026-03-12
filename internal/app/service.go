package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"rtbhid/internal/bird"
	"rtbhid/internal/state"
	"rtbhid/internal/syncer"
)

type SourceCreateInput struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	Enabled         bool   `json:"enabled"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
}

type SourcePatchInput struct {
	Name            *string `json:"name,omitempty"`
	URL             *string `json:"url,omitempty"`
	Enabled         *bool   `json:"enabled,omitempty"`
	IntervalSeconds *int    `json:"interval_seconds,omitempty"`
	TimeoutSeconds  *int    `json:"timeout_seconds,omitempty"`
}

type PeerCreateInput struct {
	Name        string `json:"name"`
	Neighbor    string `json:"neighbor"`
	NeighborASN int    `json:"neighbor_asn"`
	Enabled     *bool  `json:"enabled,omitempty"`
	Description string `json:"description,omitempty"`
	Password    string `json:"password,omitempty"`
	Multihop    int    `json:"multihop,omitempty"`
}

type PeerPatchInput struct {
	Name        *string `json:"name,omitempty"`
	Neighbor    *string `json:"neighbor,omitempty"`
	NeighborASN *int    `json:"neighbor_asn,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Description *string `json:"description,omitempty"`
	Password    *string `json:"password,omitempty"`
	Multihop    *int    `json:"multihop,omitempty"`
}

type Status struct {
	EffectivePrefixCount int               `json:"effective_prefix_count"`
	ManualPrefixCount    int               `json:"manual_prefix_count"`
	SourceCount          int               `json:"source_count"`
	PeerCount            int               `json:"peer_count"`
	LocalASN             int               `json:"local_asn"`
	LastApplyTime        *time.Time        `json:"last_apply_time,omitempty"`
	LastApplyResult      *bird.ApplyResult `json:"last_apply_result,omitempty"`
}

type Service struct {
	store         *state.Store
	fetcher       *syncer.Fetcher
	bird          *bird.Manager
	localASN      int
	schedulerTick time.Duration

	mu           sync.Mutex
	runningSyncs map[string]bool
	triggerCh    chan string
	reconcileMu  sync.Mutex
	lastApplyAt  *time.Time
	lastApplyRes *bird.ApplyResult
}

func NewService(store *state.Store, fetcher *syncer.Fetcher, birdManager *bird.Manager, tickSec, localASN int) *Service {
	if tickSec < 5 {
		tickSec = 5
	}
	if localASN <= 0 {
		localASN = 65500
	}
	return &Service{
		store:         store,
		fetcher:       fetcher,
		bird:          birdManager,
		localASN:      localASN,
		schedulerTick: time.Duration(tickSec) * time.Second,
		runningSyncs:  map[string]bool{},
		triggerCh:     make(chan string, 128),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.reconcile("startup", "")
	go s.schedulerLoop(ctx)
}

func (s *Service) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(s.schedulerTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case id := <-s.triggerCh:
			s.runSyncAsync(ctx, id)
		case <-ticker.C:
			now := time.Now().UTC()
			sources := s.store.ListSources()
			for _, src := range sources {
				if !src.Enabled {
					continue
				}
				if src.IntervalSeconds < 10 {
					src.IntervalSeconds = 300
				}
				if src.LastSyncAt == nil || now.Sub(*src.LastSyncAt) >= time.Duration(src.IntervalSeconds)*time.Second {
					s.runSyncAsync(ctx, src.ID)
				}
			}
		}
	}
}

func (s *Service) runSyncAsync(ctx context.Context, sourceID string) {
	s.mu.Lock()
	if s.runningSyncs[sourceID] {
		s.mu.Unlock()
		return
	}
	s.runningSyncs[sourceID] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.runningSyncs, sourceID)
			s.mu.Unlock()
		}()

		src, ok := s.store.GetSource(sourceID)
		if !ok {
			return
		}
		if !src.Enabled {
			return
		}

		prefixes, err := s.fetcher.Fetch(ctx, src.URL, src.TimeoutSeconds)
		if err != nil {
			_ = s.store.TouchSourceSyncResult(sourceID, s.store.SourcePrefixCount(sourceID), err)
			_ = s.store.AddEvent("error", fmt.Sprintf("sync failed: %v", err), sourceID)
			return
		}
		if err := s.store.SetSourcePrefixes(sourceID, prefixes); err != nil {
			_ = s.store.AddEvent("error", fmt.Sprintf("failed to save source prefixes: %v", err), sourceID)
			return
		}
		if err := s.store.TouchSourceSyncResult(sourceID, len(prefixes), nil); err != nil {
			_ = s.store.AddEvent("error", fmt.Sprintf("failed to update sync status: %v", err), sourceID)
			return
		}
		_ = s.store.AddEvent("info", fmt.Sprintf("sync success: %d prefixes", len(prefixes)), sourceID)
		s.reconcile("sync", sourceID)
	}()
}

func (s *Service) reconcile(reason, sourceID string) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()

	prefixes := s.store.EffectivePrefixes()
	peerItems := s.store.ListPeers()
	peers := make([]bird.Peer, 0, len(peerItems))
	for _, p := range peerItems {
		peers = append(peers, bird.Peer{
			ID:          p.ID,
			Name:        p.Name,
			Neighbor:    p.Neighbor,
			NeighborASN: p.NeighborASN,
			Enabled:     p.Enabled,
			Family:      p.Family,
			Password:    p.Password,
			Multihop:    p.Multihop,
		})
	}

	res, err := s.bird.Apply(prefixes, peers, s.localASN)
	if err != nil {
		_ = s.store.AddEvent("error", fmt.Sprintf("bird apply failed: %v", err), sourceID)
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	s.lastApplyAt = &now
	s.lastApplyRes = &res
	s.mu.Unlock()

	if res.ReloadError != "" {
		_ = s.store.AddEvent("error", fmt.Sprintf("bird reload failed (%s): %s", reason, res.ReloadError), sourceID)
		return
	}
	_ = s.store.AddEvent("info", fmt.Sprintf("bird config applied (%s): %d routes, %d peers", reason, res.RouteCount, res.PeerCount), sourceID)
}

func (s *Service) TriggerSync(sourceID string) {
	select {
	case s.triggerCh <- sourceID:
	default:
		go func() { s.triggerCh <- sourceID }()
	}
}

func (s *Service) ListManualPrefixes() []state.ManualPrefix {
	return s.store.ListManualPrefixes()
}

func (s *Service) UpsertManualPrefix(prefix string, enabled bool) (state.ManualPrefix, error) {
	entry, err := s.store.UpsertManualPrefix(prefix, enabled)
	if err != nil {
		return state.ManualPrefix{}, err
	}
	s.reconcile("manual-update", "")
	return entry, nil
}

func (s *Service) DeleteManualPrefix(prefix string) error {
	if err := s.store.DeleteManualPrefix(prefix); err != nil {
		return err
	}
	s.reconcile("manual-delete", "")
	return nil
}

func (s *Service) ListSources() []state.Source {
	return s.store.ListSources()
}

func (s *Service) CreateSource(in SourceCreateInput) (state.Source, error) {
	if in.Name == "" {
		return state.Source{}, errors.New("name is required")
	}
	if in.URL == "" {
		return state.Source{}, errors.New("url is required")
	}
	if in.IntervalSeconds <= 0 {
		in.IntervalSeconds = 300
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = 30
	}

	src := state.Source{
		ID:              newResourceID("src"),
		Name:            in.Name,
		URL:             in.URL,
		Enabled:         in.Enabled,
		IntervalSeconds: in.IntervalSeconds,
		TimeoutSeconds:  in.TimeoutSeconds,
	}
	persisted, err := s.store.UpsertSource(src)
	if err != nil {
		return state.Source{}, err
	}
	_ = s.store.AddEvent("info", "source created", persisted.ID)
	if persisted.Enabled {
		s.TriggerSync(persisted.ID)
	}
	s.reconcile("source-create", persisted.ID)
	return persisted, nil
}

func (s *Service) PatchSource(id string, patch SourcePatchInput) (state.Source, error) {
	src, ok := s.store.GetSource(id)
	if !ok {
		return state.Source{}, fmt.Errorf("source %s not found", id)
	}
	if patch.Name != nil {
		src.Name = *patch.Name
	}
	if patch.URL != nil {
		src.URL = *patch.URL
	}
	if patch.Enabled != nil {
		src.Enabled = *patch.Enabled
	}
	if patch.IntervalSeconds != nil {
		if *patch.IntervalSeconds <= 0 {
			return state.Source{}, errors.New("interval_seconds must be > 0")
		}
		src.IntervalSeconds = *patch.IntervalSeconds
	}
	if patch.TimeoutSeconds != nil {
		if *patch.TimeoutSeconds <= 0 {
			return state.Source{}, errors.New("timeout_seconds must be > 0")
		}
		src.TimeoutSeconds = *patch.TimeoutSeconds
	}

	persisted, err := s.store.UpsertSource(src)
	if err != nil {
		return state.Source{}, err
	}
	_ = s.store.AddEvent("info", "source updated", id)
	if persisted.Enabled {
		s.TriggerSync(id)
	}
	s.reconcile("source-patch", persisted.ID)
	return persisted, nil
}

func (s *Service) DeleteSource(id string) error {
	if err := s.store.DeleteSource(id); err != nil {
		return err
	}
	_ = s.store.AddEvent("info", "source deleted", id)
	s.reconcile("source-delete", id)
	return nil
}

func (s *Service) SyncNow(id string) error {
	if _, ok := s.store.GetSource(id); !ok {
		return fmt.Errorf("source %s not found", id)
	}
	s.TriggerSync(id)
	return nil
}

func (s *Service) ListPeers() []state.BGPPeer {
	return s.store.ListPeers()
}

func (s *Service) CreatePeer(in PeerCreateInput) (state.BGPPeer, error) {
	if in.Name == "" {
		return state.BGPPeer{}, errors.New("name is required")
	}
	if in.Neighbor == "" {
		return state.BGPPeer{}, errors.New("neighbor is required")
	}
	if in.NeighborASN <= 0 {
		return state.BGPPeer{}, errors.New("neighbor_asn must be > 0")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	if in.Multihop <= 0 {
		in.Multihop = 1
	}
	peer := state.BGPPeer{
		ID:          newResourceID("peer"),
		Name:        in.Name,
		Neighbor:    in.Neighbor,
		NeighborASN: in.NeighborASN,
		Enabled:     enabled,
		Description: in.Description,
		Password:    in.Password,
		Multihop:    in.Multihop,
	}
	persisted, err := s.store.UpsertPeer(peer)
	if err != nil {
		return state.BGPPeer{}, err
	}
	_ = s.store.AddEvent("info", "peer created", persisted.ID)
	s.reconcile("peer-create", persisted.ID)
	return persisted, nil
}

func (s *Service) BulkCreatePeers(items []PeerCreateInput) ([]state.BGPPeer, error) {
	created := make([]state.BGPPeer, 0, len(items))
	for i, item := range items {
		peer, err := s.CreatePeer(item)
		if err != nil {
			return nil, fmt.Errorf("bulk item %d: %w", i+1, err)
		}
		created = append(created, peer)
	}
	return created, nil
}

func (s *Service) PatchPeer(id string, patch PeerPatchInput) (state.BGPPeer, error) {
	peer, ok := s.store.GetPeer(id)
	if !ok {
		return state.BGPPeer{}, fmt.Errorf("peer %s not found", id)
	}
	if patch.Name != nil {
		peer.Name = *patch.Name
	}
	if patch.Neighbor != nil {
		peer.Neighbor = *patch.Neighbor
	}
	if patch.NeighborASN != nil {
		peer.NeighborASN = *patch.NeighborASN
	}
	if patch.Enabled != nil {
		peer.Enabled = *patch.Enabled
	}
	if patch.Description != nil {
		peer.Description = *patch.Description
	}
	if patch.Password != nil {
		peer.Password = *patch.Password
	}
	if patch.Multihop != nil {
		peer.Multihop = *patch.Multihop
	}
	persisted, err := s.store.UpsertPeer(peer)
	if err != nil {
		return state.BGPPeer{}, err
	}
	_ = s.store.AddEvent("info", "peer updated", persisted.ID)
	s.reconcile("peer-patch", persisted.ID)
	return persisted, nil
}

func (s *Service) DeletePeer(id string) error {
	if err := s.store.DeletePeer(id); err != nil {
		return err
	}
	_ = s.store.AddEvent("info", "peer deleted", id)
	s.reconcile("peer-delete", id)
	return nil
}

func (s *Service) Status() Status {
	sources := s.store.ListSources()
	s.mu.Lock()
	defer s.mu.Unlock()

	return Status{
		EffectivePrefixCount: len(s.store.EffectivePrefixes()),
		ManualPrefixCount:    s.store.TotalManualCount(),
		SourceCount:          len(sources),
		PeerCount:            s.store.TotalPeerCount(),
		LocalASN:             s.localASN,
		LastApplyTime:        s.lastApplyAt,
		LastApplyResult:      s.lastApplyRes,
	}
}

func (s *Service) Events(limit int) []state.Event {
	return s.store.ListEvents(limit)
}

func newResourceID(kind string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%s", kind, time.Now().Unix(), hex.EncodeToString(b[:]))
}
