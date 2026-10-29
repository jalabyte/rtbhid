// Package bgpserver implements a small BGP-4 speaker used by RTBHID agents
// instead of BIRD. It announces blackhole prefixes to configured peers and,
// optionally, accepts inbound peering sessions from an explicit ASN allowlist
// (for example, Indonesian network ASNs).
package bgpserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Community is an RFC 1997 standard community.
type Community struct {
	High uint16
	Low  uint16
}

func (c Community) Uint32() uint32 {
	return uint32(c.High)<<16 | uint32(c.Low)
}

func (c Community) String() string { return fmt.Sprintf("%d:%d", c.High, c.Low) }

// LargeCommunity is an RFC 8092 large community.
type LargeCommunity struct {
	Global uint32
	Local1 uint32
	Local2 uint32
}

func (c LargeCommunity) String() string {
	return fmt.Sprintf("%d:%d:%d", c.Global, c.Local1, c.Local2)
}

// PeerConfig describes a statically configured peer.
type PeerConfig struct {
	ID       string
	Name     string
	Neighbor string
	// Port is the remote TCP port. Defaults to 179.
	Port     int
	RemoteAS uint32
	LocalAS  uint32
	Password string
	Multihop int
	Enabled  bool
}

// Config configures the BGP server.
type Config struct {
	// LocalASN is the local autonomous system number.
	LocalASN uint32
	// RouterID is the BGP identifier. If zero, a value is derived from the
	// first non-loopback IPv4 address.
	RouterID netip.Addr
	// ListenAddr is the TCP listen address, e.g. ":179".
	ListenAddr string
	// HoldTime is the proposed hold time. Defaults to 90s.
	HoldTime time.Duration
	// NexthopV4 / NexthopV6 override the next hop used in announcements.
	// When unset the local address of each session is used.
	NexthopV4 netip.Addr
	NexthopV6 netip.Addr
	// StdCommunities and LargeCommunities are attached to every announcement.
	StdCommunities   []Community
	LargeCommunities []LargeCommunity
	// MaxPrefixes limits prefixes accepted from a peer (default 1000).
	MaxPrefixes int
	// MaxDynamicSessions limits inbound auto-accepted sessions (default 256).
	MaxDynamicSessions int
	// EnableIPv6 advertises the IPv6 unicast capability.
	EnableIPv6 bool
	// DialTimeout for outbound connections. Defaults to 15s.
	DialTimeout time.Duration
	// Logger overrides the standard logger.
	Logger *log.Logger
}

// SessionInfo is a snapshot of a session state for observability.
type SessionInfo struct {
	Peer        string    `json:"peer"`
	Name        string    `json:"name"`
	RemoteAS    uint32    `json:"remote_as"`
	LocalAS     uint32    `json:"local_as"`
	State       string    `json:"state"`
	Passive     bool      `json:"passive"`
	UptimeSec   int64     `json:"uptime_sec"`
	PrefixesIn  int       `json:"prefixes_in"`
	Established time.Time `json:"established,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

type v4Prefix struct {
	addr [4]byte
	bits int
}

type v6Prefix struct {
	addr [16]byte
	bits int
}

type routeSnapshot struct {
	v4 []v4Prefix
	v6 []v6Prefix
}

// Server is a minimal BGP-4 speaker.
type Server struct {
	cfg Config
	log *log.Logger

	mu        sync.Mutex
	rib       routeSnapshot
	peers     map[string]*peerRuntime
	allowlist map[uint32]struct{}
	autoOK    bool
	sessions  map[*session]struct{}
	dynamic   map[string]*session
	started   bool

	ln     net.Listener
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type peerRuntime struct {
	cfg     PeerConfig
	started bool
	session atomic.Pointer[session]
}

// New creates a BGP server. It does not listen until Start is called.
func New(cfg Config) *Server {
	if cfg.HoldTime <= 0 {
		cfg.HoldTime = 90 * time.Second
	}
	if cfg.MaxPrefixes <= 0 {
		cfg.MaxPrefixes = 1000
	}
	if cfg.MaxDynamicSessions <= 0 {
		cfg.MaxDynamicSessions = 256
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 15 * time.Second
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":179"
	}
	if cfg.LocalASN == 0 {
		cfg.LocalASN = 65500
	}
	l := cfg.Logger
	if l == nil {
		l = log.Default()
	}
	return &Server{
		cfg:       cfg,
		log:       l,
		peers:     make(map[string]*peerRuntime),
		allowlist: make(map[uint32]struct{}),
		sessions:  make(map[*session]struct{}),
		dynamic:   make(map[string]*session),
	}
}

// Start binds the listen address and begins accepting sessions.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("bgpserver: already started")
	}
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("bgpserver: listen %s: %w", s.cfg.ListenAddr, err)
	}
	s.ln = ln
	s.started = true
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	s.log.Printf("bgpserver: listening on %s (local AS %d, router-id %s)",
		s.cfg.ListenAddr, s.cfg.LocalASN, s.routerID())

	s.wg.Add(1)
	go s.acceptLoop(ln)

	// Start any peers that were configured before Start.
	s.mu.Lock()
	peers := make([]*peerRuntime, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	s.mu.Unlock()
	for _, p := range peers {
		s.startPeer(p)
	}
	return nil
}

// Stop closes the listener and all sessions.
func (s *Server) Stop() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	ln := s.ln
	s.ln = nil
	s.started = false
	sessions := make([]*session, 0, len(s.sessions))
	for sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	for _, sess := range sessions {
		sess.close(true)
	}
	s.wg.Wait()
}

func (s *Server) acceptLoop(ln net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			s.log.Printf("bgpserver: accept error: %v", err)
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleInbound(conn)
		}()
	}
}

func (s *Server) handleInbound(conn net.Conn) {
	s.mu.Lock()
	dynCount := len(s.dynamic)
	staticPeer := s.findStaticPeerLocked(conn.RemoteAddr())
	maxDyn := s.cfg.MaxDynamicSessions
	s.mu.Unlock()

	if staticPeer == nil && dynCount >= maxDyn {
		s.log.Printf("bgpserver: rejecting inbound %s: dynamic session limit reached", conn.RemoteAddr())
		_ = conn.Close()
		return
	}

	sess := newSession(s, conn, staticPeer, staticPeer == nil)
	s.runSession(sess)
}

func (s *Server) findStaticPeerLocked(addr net.Addr) *PeerConfig {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return nil
	}
	ip = ip.Unmap()
	for _, p := range s.peers {
		if !p.cfg.Enabled {
			continue
		}
		nip, err := netip.ParseAddr(strings.TrimSpace(p.cfg.Neighbor))
		if err != nil {
			continue
		}
		if nip.Unmap() == ip {
			cfg := p.cfg
			return &cfg
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Configuration updates
// ---------------------------------------------------------------------------

// SetRoutes replaces the announced prefix set. Accepted values are IPs or
// CIDRs. Invalid entries are skipped.
func (s *Server) SetRoutes(prefixes []string) {
	snap := buildSnapshot(prefixes)

	s.mu.Lock()
	s.rib = snap
	sessions := s.collectSessionsLocked()
	s.mu.Unlock()

	for _, sess := range sessions {
		if sess.established.Load() {
			sess.syncRoutes()
		}
	}
}

func buildSnapshot(prefixes []string) routeSnapshot {
	var snap routeSnapshot
	seen4 := make(map[v4Prefix]struct{})
	seen6 := make(map[v6Prefix]struct{})
	for _, raw := range prefixes {
		p, err := parsePrefix(raw)
		if err != nil {
			continue
		}
		if p.Addr().Is4() {
			v := v4Prefix{addr: p.Addr().As4(), bits: p.Bits()}
			if _, ok := seen4[v]; ok {
				continue
			}
			seen4[v] = struct{}{}
			snap.v4 = append(snap.v4, v)
		} else if p.Addr().Is6() {
			v := v6Prefix{addr: p.Addr().As16(), bits: p.Bits()}
			if _, ok := seen6[v]; ok {
				continue
			}
			seen6[v] = struct{}{}
			snap.v6 = append(snap.v6, v)
		}
	}
	sort.Slice(snap.v4, func(i, j int) bool {
		if cmp := compareV4(snap.v4[i], snap.v4[j]); cmp != 0 {
			return cmp < 0
		}
		return snap.v4[i].bits < snap.v4[j].bits
	})
	sort.Slice(snap.v6, func(i, j int) bool {
		if cmp := compareV6(snap.v6[i], snap.v6[j]); cmp != 0 {
			return cmp < 0
		}
		return snap.v6[i].bits < snap.v6[j].bits
	})
	return snap
}

func parsePrefix(raw string) (netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return netip.Prefix{}, errors.New("empty prefix")
	}
	if strings.Contains(raw, "/") {
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return netip.Prefix{}, err
		}
		return p.Masked(), nil
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, err
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32), nil
	}
	return netip.PrefixFrom(addr, 128), nil
}

func compareV4(a, b v4Prefix) int {
	for i := 0; i < 4; i++ {
		if a.addr[i] < b.addr[i] {
			return -1
		}
		if a.addr[i] > b.addr[i] {
			return 1
		}
	}
	return 0
}

func compareV6(a, b v6Prefix) int {
	for i := 0; i < 16; i++ {
		if a.addr[i] < b.addr[i] {
			return -1
		}
		if a.addr[i] > b.addr[i] {
			return 1
		}
	}
	return 0
}

// SetPeers replaces the static peer set, starting and stopping sessions as
// required.
func (s *Server) SetPeers(peers []PeerConfig) {
	wanted := make(map[string]*peerRuntime)
	for _, p := range peers {
		neighbor := strings.TrimSpace(p.Neighbor)
		if neighbor == "" {
			continue
		}
		addr, err := netip.ParseAddr(neighbor)
		if err != nil {
			s.log.Printf("bgpserver: skipping peer %q: invalid neighbor address %q", p.Name, neighbor)
			continue
		}
		p.Neighbor = addr.Unmap().String()
		if p.LocalAS == 0 {
			p.LocalAS = s.cfg.LocalASN
		}
		wanted[p.Neighbor] = &peerRuntime{cfg: p}
	}

	s.mu.Lock()
	previous := s.peers
	s.peers = wanted
	var toStart []*peerRuntime
	var toStop []*peerRuntime
	for key, p := range wanted {
		old, ok := previous[key]
		if !ok || !samePeer(old.cfg, p.cfg) {
			toStart = append(toStart, p)
		}
	}
	for key, old := range previous {
		if _, ok := wanted[key]; !ok {
			toStop = append(toStop, old)
		} else if !samePeer(old.cfg, wanted[key].cfg) {
			toStop = append(toStop, old)
		}
	}
	s.mu.Unlock()

	for _, p := range toStop {
		if sess := p.session.Load(); sess != nil {
			sess.close(true)
		}
	}
	for _, p := range toStart {
		s.startPeer(p)
	}
}

func samePeer(a, b PeerConfig) bool {
	return a.Neighbor == b.Neighbor &&
		a.RemoteAS == b.RemoteAS &&
		a.LocalAS == b.LocalAS &&
		a.Password == b.Password &&
		a.Multihop == b.Multihop &&
		a.Enabled == b.Enabled
}

func (s *Server) startPeer(p *peerRuntime) {
	if !p.cfg.Enabled {
		return
	}
	s.mu.Lock()
	if !s.started || p.started {
		s.mu.Unlock()
		return
	}
	p.started = true
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.maintainPeer(p)
	}()
}

func (s *Server) maintainPeer(p *peerRuntime) {
	backoff := time.Second
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.dialPeer(p.cfg)
		if err != nil {
			s.log.Printf("bgpserver: dial %s (%s) failed: %v", p.cfg.Name, p.cfg.Neighbor, err)
			if !s.sleep(backoff) {
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second

		sess := newSession(s, conn, &p.cfg, false)
		p.session.Store(sess)
		s.runSession(sess)
		p.session.Store(nil)

		if !s.sleep(3 * time.Second) {
			return
		}
	}
}

func (s *Server) dialPeer(p PeerConfig) (net.Conn, error) {
	port := p.Port
	if port <= 0 {
		port = 179
	}
	addr := net.JoinHostPort(p.Neighbor, fmt.Sprintf("%d", port))
	d := net.Dialer{Timeout: s.cfg.DialTimeout}
	conn, err := d.DialContext(s.ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (s *Server) sleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-s.ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// SetAllowlist replaces the inbound auto-accept policy. When autoAccept is
// false inbound sessions are only accepted for statically configured peers.
func (s *Server) SetAllowlist(asns []uint32, autoAccept bool) {
	set := make(map[uint32]struct{}, len(asns))
	for _, a := range asns {
		if a > 0 {
			set[a] = struct{}{}
		}
	}
	s.mu.Lock()
	s.allowlist = set
	s.autoOK = autoAccept
	s.mu.Unlock()
}

func (s *Server) allowRemoteASN(asn uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.autoOK {
		return false
	}
	_, ok := s.allowlist[asn]
	return ok
}

// Stats returns a snapshot of all sessions.
func (s *Server) Stats() []SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SessionInfo, 0, len(s.sessions))
	for sess := range s.sessions {
		out = append(out, sess.info())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Peer < out[j].Peer })
	return out
}

// Addr returns the bound listener address, or an empty string when the
// server is not listening.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

func (s *Server) collectSessionsLocked() []*session {
	out := make([]*session, 0, len(s.sessions))
	for sess := range s.sessions {
		out = append(out, sess)
	}
	return out
}

func (s *Server) routerID() netip.Addr {
	if s.cfg.RouterID.IsValid() {
		return s.cfg.RouterID
	}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if v4 := ipnet.IP.To4(); v4 != nil {
					return netip.AddrFrom4([4]byte{v4[0], v4[1], v4[2], v4[3]})
				}
			}
		}
	}
	return netip.MustParseAddr("127.0.0.1")
}

func (s *Server) registerSession(sess *session) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return false
	}
	s.sessions[sess] = struct{}{}
	if sess.passive && sess.static == nil {
		key := sess.remoteIP()
		if old, ok := s.dynamic[key]; ok && old != sess {
			go old.close(true)
		}
		s.dynamic[key] = sess
	}
	return true
}

func (s *Server) unregisterSession(sess *session) {
	s.mu.Lock()
	delete(s.sessions, sess)
	if sess.passive && sess.static == nil {
		if s.dynamic[sess.remoteIP()] == sess {
			delete(s.dynamic, sess.remoteIP())
		}
	}
	s.mu.Unlock()
}
