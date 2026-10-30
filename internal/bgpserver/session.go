package bgpserver

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	stateOpenSent    = "OpenSent"
	stateOpenConfirm = "OpenConfirm"
	stateEstablished = "Established"
	stateClosed      = "Closed"
)

type session struct {
	srv     *Server
	conn    net.Conn
	static  *PeerConfig
	passive bool

	remoteAS uint32
	localAS  uint32
	routerID uint32
	as4      bool
	ipv6     bool
	ibgp     bool
	hold     time.Duration

	established   atomic.Bool
	establishedAt atomic.Int64
	state         atomic.Value // string
	lastErr       atomic.Value // string

	writeMu sync.Mutex

	// kick wakes writerLoop to sync routes; refresh forces a full resend
	// (ROUTE-REFRESH). sentV4/sentV6 alias the shared snapshot last sent and
	// are only touched by writerLoop.
	kick    chan struct{}
	refresh atomic.Bool
	sentV4  []v4Prefix
	sentV6  []v6Prefix

	warnedUpdate bool // reader goroutine only

	closeOnce sync.Once
	closed    chan struct{}
}

func newSession(srv *Server, conn net.Conn, static *PeerConfig, passive bool) *session {
	sess := &session{
		srv:      srv,
		conn:     conn,
		static:   static,
		passive:  passive,
		localAS:  srv.cfg.LocalASN,
		routerID: routerIDUint32(srv.routerID()),
		hold:     srv.cfg.HoldTime,
		closed:   make(chan struct{}),
		kick:     make(chan struct{}, 1),
	}
	if static != nil {
		if static.LocalAS != 0 {
			sess.localAS = static.LocalAS
		}
		sess.remoteAS = static.RemoteAS
	}
	sess.state.Store(stateOpenSent)
	sess.lastErr.Store("")
	return sess
}

func routerIDUint32(addr netip.Addr) uint32 {
	a := addr.As4()
	return binary.BigEndian.Uint32(a[:])
}

func (s *session) remoteIP() string {
	host, _, err := net.SplitHostPort(s.conn.RemoteAddr().String())
	if err != nil {
		return s.conn.RemoteAddr().String()
	}
	return host
}

func (s *session) name() string {
	if s.static != nil && s.static.Name != "" {
		return s.static.Name
	}
	return "dynamic-" + s.remoteIP()
}

func (s *session) setState(state string) {
	s.state.Store(state)
}

func (s *session) setErr(err error) {
	if err == nil {
		s.lastErr.Store("")
	} else {
		s.lastErr.Store(err.Error())
	}
}

func (s *session) info() SessionInfo {
	info := SessionInfo{
		Peer:     s.remoteIP(),
		Name:     s.name(),
		RemoteAS: s.remoteAS,
		LocalAS:  s.localAS,
		State:    s.state.Load().(string),
		Passive:  s.passive,
	}
	if v, ok := s.lastErr.Load().(string); ok {
		info.LastError = v
	}
	if ts := s.establishedAt.Load(); ts > 0 {
		info.Established = time.Unix(0, ts)
		info.UptimeSec = int64(time.Since(info.Established).Seconds())
	}
	return info
}

func (s *session) send(b []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := s.conn.Write(b); err != nil {
		s.setErr(err)
		return err
	}
	return nil
}

func (s *session) close(notify bool) {
	s.closeOnce.Do(func() {
		if notify {
			// Best effort with a short deadline so a dead peer can't stall close.
			s.writeMu.Lock()
			_ = s.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			_, _ = s.conn.Write(encodeNotification(notifCease, 0, nil))
			s.writeMu.Unlock()
		}
		_ = s.conn.Close()
		s.setState(stateClosed)
		close(s.closed)
	})
}

// runSession performs the OPEN/KEEPALIVE exchange and then the established
// message loop. It blocks until the session ends.
func (s *Server) runSession(sess *session) {
	if !s.registerSession(sess) {
		_ = sess.conn.Close()
		return
	}
	defer s.unregisterSession(sess)
	defer sess.close(false)

	s.log.Printf("bgpserver: inbound connection from %s (passive=%v)", sess.conn.RemoteAddr(), sess.passive)

	// Send our OPEN immediately.
	if err := sess.send(encodeOpen(sess.localAS, uint16(s.holdSeconds()), sess.routerID, s.cfg.EnableIPv6)); err != nil {
		return
	}

	// Wait for the peer OPEN.
	_ = sess.conn.SetReadDeadline(time.Now().Add(s.initTimeout()))
	msg, err := readMessage(sess.conn)
	if err != nil {
		sess.setErr(fmt.Errorf("read OPEN: %w", err))
		return
	}
	if msg.Type != msgOpen {
		_ = sess.send(encodeNotification(notifFSM, 0, nil))
		sess.setErr(fmt.Errorf("expected OPEN, got message type %d", msg.Type))
		return
	}
	open, err := decodeOpen(msg.Body)
	if err != nil {
		_ = sess.send(encodeNotification(notifOpenMessage, 0, nil))
		sess.setErr(err)
		return
	}
	if open.Version != bgpVersion {
		_ = sess.send(encodeNotification(notifOpenMessage, openUnsupportedVersion, nil))
		sess.setErr(fmt.Errorf("unsupported BGP version %d", open.Version))
		return
	}

	remoteAS := uint32(open.MyAS)
	if open.HasAS4 && open.AS4 != 0 {
		remoteAS = open.AS4
	}
	sess.remoteAS = remoteAS
	sess.as4 = open.HasAS4
	sess.ipv6 = open.IPv6Unicast && s.cfg.EnableIPv6
	sess.ibgp = remoteAS == sess.localAS

	// Validate against static configuration, if any.
	if sess.static != nil {
		if sess.static.RemoteAS != 0 && remoteAS != sess.static.RemoteAS {
			_ = sess.send(encodeNotification(notifOpenMessage, openBadPeerAS, nil))
			sess.setErr(fmt.Errorf("remote AS %d does not match configured AS %d", remoteAS, sess.static.RemoteAS))
			return
		}
	}

	// Auto-accept policy for inbound sessions without static config.
	if sess.passive && sess.static == nil {
		if !s.allowRemoteASN(remoteAS) {
			_ = sess.send(encodeNotification(notifOpenMessage, openBadPeerAS, nil))
			sess.setErr(fmt.Errorf("AS%d rejected: auto-accept disabled or ASN not in allowlist", remoteAS))
			s.log.Printf("bgpserver: rejected inbound peer %s AS%d: auto-accept disabled or ASN not in allowlist", sess.remoteIP(), remoteAS)
			return
		}
	}

	// Negotiate hold time.
	sess.hold = negotiateHold(s.cfg.HoldTime, open.HoldTime)
	if sess.hold > 0 && sess.hold < 3*time.Second {
		_ = sess.send(encodeNotification(notifOpenMessage, openUnacceptableHoldTime, nil))
		sess.setErr(fmt.Errorf("unacceptable hold time %s", sess.hold))
		return
	}

	sess.setState(stateOpenConfirm)
	if err := sess.send(encodeKeepalive()); err != nil {
		return
	}

	// Read until KEEPALIVE to reach Established.
	deadline := s.initTimeout()
	for {
		_ = sess.conn.SetReadDeadline(time.Now().Add(deadline))
		msg, err := readMessage(sess.conn)
		if err != nil {
			sess.setErr(fmt.Errorf("wait for KEEPALIVE: %w", err))
			return
		}
		switch msg.Type {
		case msgKeepalive:
			goto established
		case msgNotification:
			sess.setErr(fmt.Errorf("peer sent NOTIFICATION during open"))
			return
		default:
			_ = sess.send(encodeNotification(notifFSM, 0, nil))
			sess.setErr(fmt.Errorf("unexpected message type %d before Established", msg.Type))
			return
		}
	}

established:
	sess.established.Store(true)
	sess.establishedAt.Store(time.Now().UnixNano())
	sess.setState(stateEstablished)
	s.log.Printf("bgpserver: session established with %s AS%d (%s)", sess.remoteIP(), sess.remoteAS, sess.name())

	go sess.writerLoop()
	sess.requestSync()

	for {
		readDeadline := time.Time{}
		if sess.hold > 0 {
			readDeadline = time.Now().Add(sess.hold)
		}
		_ = sess.conn.SetReadDeadline(readDeadline)
		msg, err := readMessage(sess.conn)
		if err != nil {
			select {
			case <-sess.closed:
			default:
				sess.setErr(err)
			}
			return
		}
		switch msg.Type {
		case msgKeepalive:
			continue
		case msgUpdate:
			sess.ignoreUpdate()
		case msgRouteRefresh:
			sess.refresh.Store(true)
			sess.requestSync()
		case msgNotification:
			if len(msg.Body) >= 2 {
				sess.setErr(fmt.Errorf("peer notification code=%d subcode=%d", msg.Body[0], msg.Body[1]))
			}
			return
		default:
			_ = sess.send(encodeNotification(notifFSM, 0, nil))
			sess.setErr(fmt.Errorf("unexpected message type %d", msg.Type))
			return
		}
	}
}

func (s *Server) holdSeconds() int {
	secs := int(s.cfg.HoldTime / time.Second)
	if secs <= 0 {
		secs = 90
	}
	return secs
}

func (s *Server) initTimeout() time.Duration {
	if s.cfg.HoldTime < 30*time.Second {
		return 30 * time.Second
	}
	return s.cfg.HoldTime
}

// negotiateHold returns the smaller hold time; zero (no keepalives) wins.
func negotiateHold(ours time.Duration, peer uint16) time.Duration {
	return min(ours, time.Duration(peer)*time.Second)
}

func (s *session) requestSync() {
	select {
	case s.kick <- struct{}{}:
	default: // a sync is already pending; it will read the latest RIB
	}
}

// writerLoop owns route announcements and keepalives for an Established
// session. UPDATEs reset the peer's hold timer too, so a long sync is safe.
func (s *session) writerLoop() {
	var tick <-chan time.Time
	if s.hold > 0 { // hold 0 means no keepalives (RFC 4271 4.4)
		t := time.NewTicker(max(s.hold/3, time.Second))
		defer t.Stop()
		tick = t.C
	}
	for {
		select {
		case <-s.closed:
			return
		case <-s.srv.ctx.Done():
			s.close(true)
			return
		case <-s.kick:
			if err := s.syncRoutes(); err != nil {
				s.close(false)
				return
			}
		case <-tick:
			if err := s.send(encodeKeepalive()); err != nil {
				s.close(false)
				return
			}
		}
	}
}

// ignoreUpdate discards routes from the peer: this speaker is announce-only.
// Downstream routers should export nothing to us; flag it once if they do.
func (s *session) ignoreUpdate() {
	if s.warnedUpdate {
		return
	}
	s.warnedUpdate = true
	s.setErr(fmt.Errorf("peer sends routes; ignored (announce-only), set its export filter to none"))
	s.srv.log.Printf("bgpserver: %s (%s) is sending routes; ignoring (announce-only)", s.remoteIP(), s.name())
}

// syncRoutes reconciles the announced route set for this session with the
// server RIB. Only writerLoop calls it.
func (s *session) syncRoutes() error {
	s.srv.mu.Lock()
	rib := s.srv.rib
	s.srv.mu.Unlock()

	if s.refresh.Swap(false) {
		s.sentV4, s.sentV6 = nil, nil
	}

	nextHop4 := s.nexthopV4()
	nextHop6 := s.nexthopV6()
	// Skip a family we cannot give a valid next hop (e.g. IPv4 over an IPv6
	// transport without RTBH_BGP_NEXTHOP_V4); peers reject 0.0.0.0.
	wantV4 := rib.v4
	if nextHop4 == ([4]byte{}) {
		wantV4 = nil
	}
	var wantV6 []v6Prefix
	if s.ipv6 && nextHop6.IsValid() {
		wantV6 = rib.v6
	}

	delV4, addV4 := diffSorted(s.sentV4, wantV4, compareV4)
	delV6, addV6 := diffSorted(s.sentV6, wantV6, compareV6)

	comms := s.srv.stdCommunityWords()
	large := s.srv.cfg.LargeCommunities

	for i := 0; i < len(delV4); i += updateChunkV4 {
		end := min(i+updateChunkV4, len(delV4))
		if err := s.send(encodeWithdrawV4(delV4[i:end])); err != nil {
			return err
		}
	}
	for i := 0; i < len(addV4); i += updateChunkV4 {
		end := min(i+updateChunkV4, len(addV4))
		if err := s.send(encodeUpdateV4(prefixesToV4(addV4[i:end]), bitsV4(addV4[i:end]), s.localAS, s.as4, nextHop4, comms, large, s.ibgp)); err != nil {
			return err
		}
	}
	for i := 0; i < len(delV6); i += updateChunkV6 {
		end := min(i+updateChunkV6, len(delV6))
		if err := s.send(encodeWithdrawV6(delV6[i:end])); err != nil {
			return err
		}
	}
	for i := 0; i < len(addV6); i += updateChunkV6 {
		end := min(i+updateChunkV6, len(addV6))
		if err := s.send(encodeUpdateV6(prefixesToV6(addV6[i:end]), bitsV6(addV6[i:end]), s.localAS, s.as4, nextHop6.As16(), comms, large, s.ibgp)); err != nil {
			return err
		}
	}

	s.sentV4, s.sentV6 = wantV4, wantV6
	return nil
}

const (
	updateChunkV4 = 200
	updateChunkV6 = 64
)

func (s *session) nexthopV4() [4]byte {
	if s.srv.cfg.NexthopV4.IsValid() && s.srv.cfg.NexthopV4.Is4() {
		return s.srv.cfg.NexthopV4.As4()
	}
	if tcp, ok := s.conn.LocalAddr().(*net.TCPAddr); ok {
		if v4 := tcp.IP.To4(); v4 != nil {
			return [4]byte{v4[0], v4[1], v4[2], v4[3]}
		}
	}
	return [4]byte{}
}

func (s *session) nexthopV6() netip.Addr {
	if s.srv.cfg.NexthopV6.IsValid() && s.srv.cfg.NexthopV6.Is6() {
		return s.srv.cfg.NexthopV6
	}
	if tcp, ok := s.conn.LocalAddr().(*net.TCPAddr); ok {
		if ip, ok := netip.AddrFromSlice(tcp.IP); ok && ip.Is6() && !ip.Is4In6() {
			return ip
		}
	}
	return netip.Addr{}
}

func (s *Server) stdCommunityWords() []uint32 {
	if len(s.cfg.StdCommunities) == 0 {
		return nil
	}
	out := make([]uint32, 0, len(s.cfg.StdCommunities))
	for _, c := range s.cfg.StdCommunities {
		out = append(out, c.Uint32())
	}
	return out
}

func prefixesToV4(ps []v4Prefix) [][4]byte {
	out := make([][4]byte, len(ps))
	for i, p := range ps {
		out[i] = p.addr
	}
	return out
}

func bitsV4(ps []v4Prefix) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = int(p.bits)
	}
	return out
}

func prefixesToV6(ps []v6Prefix) [][16]byte {
	out := make([][16]byte, len(ps))
	for i, p := range ps {
		out[i] = p.addr
	}
	return out
}

func bitsV6(ps []v6Prefix) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = int(p.bits)
	}
	return out
}

func encodeWithdrawV4(ps []v4Prefix) []byte {
	var w []byte
	for _, p := range ps {
		w = append(w, encodeNLRI(p.addr[:], int(p.bits))...)
	}
	body := make([]byte, 0, 4+len(w))
	body = append(body, byte(len(w)>>8), byte(len(w)))
	body = append(body, w...)
	body = append(body, 0, 0)
	return encodeMessage(msgUpdate, body)
}

func encodeWithdrawV6(ps []v6Prefix) []byte {
	var nlri []byte
	for _, p := range ps {
		nlri = append(nlri, encodeNLRI(p.addr[:], int(p.bits))...)
	}
	attrs := encodePathAttrs([]pathAttr{attrMPUnreachV6(nlri)})
	body := make([]byte, 0, 4+len(attrs))
	body = append(body, 0, 0)
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	return encodeMessage(msgUpdate, body)
}

// ParseASNList parses a comma/space separated ASN list.
func ParseASNList(raw string) ([]uint32, error) {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';' || r == '\n' || r == '\t'
	})
	out := make([]uint32, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(f), "AS"))
		if f == "" {
			continue
		}
		var v uint32
		if _, err := fmt.Sscanf(f, "%d", &v); err != nil {
			return nil, fmt.Errorf("invalid ASN %q: %w", f, err)
		}
		if v == 0 || v > 4294967295 {
			return nil, fmt.Errorf("invalid ASN %q", f)
		}
		out = append(out, v)
	}
	return out, nil
}
