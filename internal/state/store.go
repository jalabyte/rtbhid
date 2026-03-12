package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxEvents = 300

type Store struct {
	mu   sync.Mutex
	path string
	data Data
}

func New(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = Data{
		ManualPrefixes: map[string]ManualPrefix{},
		Sources:        map[string]Source{},
		SourcePrefixes: map[string]map[string]any{},
		Peers:          map[string]BGPPeer{},
		Events:         []Event{},
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.persistLocked()
		}
		return fmt.Errorf("read state: %w", err)
	}
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}
	if s.data.ManualPrefixes == nil {
		s.data.ManualPrefixes = map[string]ManualPrefix{}
	}
	if s.data.Sources == nil {
		s.data.Sources = map[string]Source{}
	}
	if s.data.SourcePrefixes == nil {
		s.data.SourcePrefixes = map[string]map[string]any{}
	}
	if s.data.Peers == nil {
		s.data.Peers = map[string]BGPPeer{}
	}
	if s.data.Events == nil {
		s.data.Events = []Event{}
	}
	return nil
}

func NormalizePrefix(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("empty prefix")
	}
	if p, err := netip.ParsePrefix(trimmed); err == nil {
		return p.Masked().String(), nil
	}
	if addr, err := netip.ParseAddr(trimmed); err == nil {
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		p := netip.PrefixFrom(addr, bits)
		return p.Masked().String(), nil
	}
	return "", fmt.Errorf("invalid IP/CIDR: %s", trimmed)
}

func NormalizeNeighbor(value string) (normalized, family string, err error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", errors.New("empty neighbor")
	}
	addr, parseErr := netip.ParseAddr(trimmed)
	if parseErr != nil {
		return "", "", fmt.Errorf("invalid neighbor IP: %s", trimmed)
	}
	if addr.Is4() {
		return addr.String(), "ipv4", nil
	}
	return addr.String(), "ipv6", nil
}

func (s *Store) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

func (s *Store) AddEvent(level, msg, sourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Events = append([]Event{{
		Time:     time.Now().UTC(),
		Level:    level,
		Message:  msg,
		SourceID: sourceID,
	}}, s.data.Events...)
	if len(s.data.Events) > maxEvents {
		s.data.Events = s.data.Events[:maxEvents]
	}
	return s.persistLocked()
}

func (s *Store) ListEvents(limit int) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > len(s.data.Events) {
		limit = len(s.data.Events)
	}
	out := make([]Event, 0, limit)
	out = append(out, s.data.Events[:limit]...)
	return out
}

func (s *Store) UpsertManualPrefix(prefix string, enabled bool) (ManualPrefix, error) {
	norm, err := NormalizePrefix(prefix)
	if err != nil {
		return ManualPrefix{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	existing, ok := s.data.ManualPrefixes[norm]
	if !ok {
		existing = ManualPrefix{Prefix: norm, AddedAt: now}
	}
	existing.Enabled = enabled
	existing.UpdatedAt = now
	s.data.ManualPrefixes[norm] = existing
	return existing, s.persistLocked()
}

func (s *Store) DeleteManualPrefix(prefix string) error {
	norm, err := NormalizePrefix(prefix)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data.ManualPrefixes, norm)
	return s.persistLocked()
}

func (s *Store) ListManualPrefixes() []ManualPrefix {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]ManualPrefix, 0, len(s.data.ManualPrefixes))
	for _, v := range s.data.ManualPrefixes {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Prefix < out[j].Prefix
	})
	return out
}

func (s *Store) UpsertSource(src Source) (Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	existing, ok := s.data.Sources[src.ID]
	if !ok {
		src.CreatedAt = now
	} else {
		src.CreatedAt = existing.CreatedAt
		src.LastSyncAt = existing.LastSyncAt
		src.LastSuccessAt = existing.LastSuccessAt
		src.LastError = existing.LastError
		src.LastCount = existing.LastCount
	}
	src.UpdatedAt = now
	s.data.Sources[src.ID] = src
	if _, exists := s.data.SourcePrefixes[src.ID]; !exists {
		s.data.SourcePrefixes[src.ID] = map[string]any{}
	}
	return src, s.persistLocked()
}

func (s *Store) DeleteSource(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data.Sources, id)
	delete(s.data.SourcePrefixes, id)
	return s.persistLocked()
}

func (s *Store) GetSource(id string) (Source, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, ok := s.data.Sources[id]
	return src, ok
}

func (s *Store) ListSources() []Source {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Source, 0, len(s.data.Sources))
	for _, v := range s.data.Sources {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *Store) TouchSourceSyncResult(id string, count int, syncErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	src, ok := s.data.Sources[id]
	if !ok {
		return fmt.Errorf("source %s not found", id)
	}
	now := time.Now().UTC()
	src.LastSyncAt = &now
	src.LastCount = count
	if syncErr != nil {
		src.LastError = syncErr.Error()
	} else {
		src.LastError = ""
		src.LastSuccessAt = &now
	}
	src.UpdatedAt = now
	s.data.Sources[id] = src
	return s.persistLocked()
}

func (s *Store) SetSourcePrefixes(sourceID string, prefixes []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Sources[sourceID]; !ok {
		return fmt.Errorf("source %s not found", sourceID)
	}
	set := map[string]any{}
	for _, p := range prefixes {
		set[p] = struct{}{}
	}
	s.data.SourcePrefixes[sourceID] = set
	return s.persistLocked()
}

func (s *Store) UpsertPeer(peer BGPPeer) (BGPPeer, error) {
	neighbor, family, err := NormalizeNeighbor(peer.Neighbor)
	if err != nil {
		return BGPPeer{}, err
	}
	if strings.TrimSpace(peer.Name) == "" {
		return BGPPeer{}, errors.New("peer name is required")
	}
	if peer.NeighborASN <= 0 {
		return BGPPeer{}, errors.New("neighbor_asn must be > 0")
	}
	if peer.Multihop <= 0 {
		peer.Multihop = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	peer.Neighbor = neighbor
	peer.Family = family
	existing, ok := s.data.Peers[peer.ID]
	if !ok {
		peer.CreatedAt = now
	} else {
		peer.CreatedAt = existing.CreatedAt
	}
	peer.UpdatedAt = now
	s.data.Peers[peer.ID] = peer
	return peer, s.persistLocked()
}

func (s *Store) DeletePeer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Peers, id)
	return s.persistLocked()
}

func (s *Store) GetPeer(id string) (BGPPeer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	peer, ok := s.data.Peers[id]
	return peer, ok
}

func (s *Store) ListPeers() []BGPPeer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]BGPPeer, 0, len(s.data.Peers))
	for _, v := range s.data.Peers {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Neighbor < out[j].Neighbor
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *Store) EffectivePrefixes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	set := map[string]any{}
	for p, v := range s.data.ManualPrefixes {
		if v.Enabled {
			set[p] = struct{}{}
		}
	}
	for id, src := range s.data.Sources {
		if !src.Enabled {
			continue
		}
		for p := range s.data.SourcePrefixes[id] {
			set[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (s *Store) SourcePrefixCount(sourceID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data.SourcePrefixes[sourceID])
}

func (s *Store) TotalManualCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data.ManualPrefixes)
}

func (s *Store) TotalPeerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.data.Peers)
}
