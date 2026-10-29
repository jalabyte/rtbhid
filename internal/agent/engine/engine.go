// Package engine abstracts the BGP implementation used by an RTBHID agent.
package engine

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strings"
	"time"

	"rtbhid/internal/agent/bird"
	"rtbhid/internal/bgpserver"
)

// Engine applies routing and peering state.
type Engine interface {
	Apply(prefixes []string, peers []bird.Peer, allowedASNs []uint32, autoAccept bool) (bird.ApplyResult, error)
	Close()
}

// SessionStatus is a single BGP session as reported to the portal.
type SessionStatus struct {
	Peer       string
	Name       string
	RemoteAS   uint32
	LocalAS    uint32
	State      string
	Passive    bool
	UptimeSec  int64
	PrefixesIn int
	LastError  string
}

// Reporter is implemented by engines that can report live BGP sessions.
type Reporter interface {
	BGPStatus() (engineName string, sessions []SessionStatus)
}

// BirdManager is the BIRD config-file engine.
type BirdManager struct {
	mgr      *bird.Manager
	localASN int
}

// NewBird wraps a BIRD manager.
func NewBird(mgr *bird.Manager, localASN int) *BirdManager {
	return &BirdManager{mgr: mgr, localASN: localASN}
}

func (b *BirdManager) Apply(prefixes []string, peers []bird.Peer, _ []uint32, _ bool) (bird.ApplyResult, error) {
	return b.mgr.Apply(prefixes, peers, b.localASN)
}

func (b *BirdManager) Close() {}

// BGPStatus reports the engine name. BIRD session detail would require parsing
// birdc output, so no per-session rows are reported.
func (b *BirdManager) BGPStatus() (string, []SessionStatus) {
	return "bird", nil
}

// CustomConfig configures the embedded BGP server.
type CustomConfig struct {
	LocalASN         int
	ListenAddr       string
	RouterID         string
	NexthopV4        string
	NexthopV6        string
	HoldTimeSec      int
	EnableIPv6       bool
	MaxPrefixes      int
	MaxDynamic       int
	ExportStdComms   string
	ExportLargeComms string
	Logger           *log.Logger
}

// Custom is the embedded BGP server engine.
type Custom struct {
	srv      *bgpserver.Server
	localASN int
}

// NewCustom starts the embedded BGP server.
func NewCustom(cfg CustomConfig) (*Custom, error) {
	exportPolicy, err := bird.ParseExportPolicy(cfg.ExportStdComms, cfg.ExportLargeComms)
	if err != nil {
		return nil, fmt.Errorf("parse export communities: %w", err)
	}
	bgpCfg := bgpserver.Config{
		LocalASN:           uint32(cfg.LocalASN),
		ListenAddr:         cfg.ListenAddr,
		HoldTime:           time.Duration(cfg.HoldTimeSec) * time.Second,
		StdCommunities:     toCommunities(exportPolicy.Standard),
		LargeCommunities:   toLargeCommunities(exportPolicy.Large),
		EnableIPv6:         cfg.EnableIPv6,
		MaxPrefixes:        cfg.MaxPrefixes,
		MaxDynamicSessions: cfg.MaxDynamic,
		Logger:             cfg.Logger,
	}
	if cfg.RouterID != "" {
		addr, err := netip.ParseAddr(strings.TrimSpace(cfg.RouterID))
		if err != nil || !addr.Is4() {
			return nil, fmt.Errorf("invalid RTBH_BGP_ROUTER_ID %q: must be an IPv4 address", cfg.RouterID)
		}
		bgpCfg.RouterID = addr
	}
	if cfg.NexthopV4 != "" {
		addr, err := netip.ParseAddr(strings.TrimSpace(cfg.NexthopV4))
		if err != nil || !addr.Is4() {
			return nil, fmt.Errorf("invalid RTBH_BGP_NEXTHOP_V4 %q", cfg.NexthopV4)
		}
		bgpCfg.NexthopV4 = addr
	}
	if cfg.NexthopV6 != "" {
		addr, err := netip.ParseAddr(strings.TrimSpace(cfg.NexthopV6))
		if err != nil || !addr.Is6() {
			return nil, fmt.Errorf("invalid RTBH_BGP_NEXTHOP_V6 %q", cfg.NexthopV6)
		}
		bgpCfg.NexthopV6 = addr
	}

	srv := bgpserver.New(bgpCfg)
	if err := srv.Start(context.Background()); err != nil {
		return nil, err
	}
	return &Custom{srv: srv, localASN: cfg.LocalASN}, nil
}

func (c *Custom) Apply(prefixes []string, peers []bird.Peer, allowedASNs []uint32, autoAccept bool) (bird.ApplyResult, error) {
	peerCfg := make([]bgpserver.PeerConfig, 0, len(peers))
	for _, p := range peers {
		peerCfg = append(peerCfg, bgpserver.PeerConfig{
			ID:       p.ID,
			Name:     p.Name,
			Neighbor: p.Neighbor,
			RemoteAS: uint32(p.NeighborASN),
			LocalAS:  uint32(c.localASN),
			Password: p.Password,
			Multihop: p.Multihop,
			Enabled:  p.Enabled,
		})
	}
	c.srv.SetRoutes(prefixes)
	c.srv.SetPeers(peerCfg)
	c.srv.SetAllowlist(allowedASNs, autoAccept)

	return bird.ApplyResult{
		RouteCount: len(prefixes),
		PeerCount:  len(peerCfg),
	}, nil
}

func (c *Custom) Close() {
	c.srv.Stop()
}

// Sessions exposes the current BGP sessions (custom engine only).
func (c *Custom) Sessions() []bgpserver.SessionInfo {
	return c.srv.Stats()
}

// BGPStatus reports live sessions for the embedded BGP server.
func (c *Custom) BGPStatus() (string, []SessionStatus) {
	stats := c.srv.Stats()
	out := make([]SessionStatus, 0, len(stats))
	for _, s := range stats {
		out = append(out, SessionStatus{
			Peer:       s.Peer,
			Name:       s.Name,
			RemoteAS:   s.RemoteAS,
			LocalAS:    s.LocalAS,
			State:      s.State,
			Passive:    s.Passive,
			UptimeSec:  s.UptimeSec,
			PrefixesIn: s.PrefixesIn,
			LastError:  s.LastError,
		})
	}
	return "custom", out
}

func toCommunities(in []bird.StandardCommunity) []bgpserver.Community {
	out := make([]bgpserver.Community, 0, len(in))
	for _, c := range in {
		out = append(out, bgpserver.Community{High: c.High, Low: c.Low})
	}
	return out
}

func toLargeCommunities(in []bird.LargeCommunity) []bgpserver.LargeCommunity {
	out := make([]bgpserver.LargeCommunity, 0, len(in))
	for _, c := range in {
		out = append(out, bgpserver.LargeCommunity{Global: c.Global, Local1: c.Local1, Local2: c.Local2})
	}
	return out
}
