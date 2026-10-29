package bgpserver

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"
)

func testConfig(asn uint32) Config {
	return Config{
		LocalASN:       asn,
		ListenAddr:     "127.0.0.1:0",
		HoldTime:       3 * time.Second,
		MaxPrefixes:    1000,
		DialTimeout:    2 * time.Second,
		EnableIPv6:     true,
		NexthopV6:      netip.MustParseAddr("2001:db8::1"),
		StdCommunities: []Community{{High: 65535, Low: 666}},
	}
}

func startTestServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	srv := New(cfg)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(srv.Stop)
	return srv
}

// testPeer performs a minimal BGP handshake and returns the connection.
func testPeer(t *testing.T, addr string, asn uint32, ipv6 bool) (net.Conn, []*message) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	open, err := readMessage(conn)
	if err != nil {
		t.Fatalf("read server OPEN: %v", err)
	}
	if open.Type != msgOpen {
		t.Fatalf("expected OPEN from server, got type %d", open.Type)
	}

	if _, err := conn.Write(encodeOpen(asn, 3, 0x0a000001, ipv6)); err != nil {
		t.Fatalf("send OPEN: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	next, err := readMessage(conn)
	if err != nil {
		t.Fatalf("read after OPEN: %v", err)
	}
	if next.Type == msgNotification {
		return conn, []*message{next}
	}
	if next.Type != msgKeepalive {
		t.Fatalf("expected KEEPALIVE, got type %d", next.Type)
	}
	if _, err := conn.Write(encodeKeepalive()); err != nil {
		t.Fatalf("send KEEPALIVE: %v", err)
	}
	return conn, nil
}

func readUpdates(t *testing.T, conn net.Conn, want int, timeout time.Duration) []prefixSet {
	t.Helper()
	var out []prefixSet
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(deadline)
		msg, err := readMessage(conn)
		if err != nil {
			break
		}
		switch msg.Type {
		case msgUpdate:
			u, err := decodeUpdate(msg.Body)
			if err != nil {
				t.Fatalf("decode update: %v", err)
			}
			out = append(out, prefixSet{update: u})
			if len(out) >= want {
				return out
			}
		case msgKeepalive:
			continue
		default:
			t.Logf("unexpected message type %d", msg.Type)
		}
	}
	return out
}

type prefixSet struct {
	update *updateMessage
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestOpenRoundTrip(t *testing.T) {
	raw := encodeOpen(65000, 90, 0x0a000001, true)
	msg, err := readMessage(bytesReader(raw))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	open, err := decodeOpen(msg.Body)
	if err != nil {
		t.Fatalf("decode open: %v", err)
	}
	if open.Version != 4 || open.MyAS != 65000 || !open.HasAS4 || open.AS4 != 65000 {
		t.Fatalf("unexpected open: %+v", open)
	}
	if !open.IPv6Unicast || !open.IPv4Unicast || !open.RouteRefresh {
		t.Fatalf("missing capabilities: %+v", open)
	}
}

func TestUpdateRoundTrip(t *testing.T) {
	prefixes := [][4]byte{{10, 0, 0, 0}, {192, 0, 2, 0}}
	bits := []int{8, 24}
	raw := encodeUpdateV4(prefixes, bits, 65000, true, [4]byte{1, 2, 3, 4}, []uint32{65535<<16 | 666}, nil, false)
	msg, err := readMessage(bytesReader(raw))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	u, err := decodeUpdate(msg.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(u.Announced) != 2 {
		t.Fatalf("expected 2 announced prefixes, got %d", len(u.Announced))
	}
	if u.Announced[0].Bits != 8 || u.Announced[1].Bits != 24 {
		t.Fatalf("unexpected prefix lengths: %+v", u.Announced)
	}
	if len(u.Communities) != 1 || u.Communities[0] != 65535<<16|666 {
		t.Fatalf("unexpected communities: %+v", u.Communities)
	}
	if u.ASPathLen != 1 {
		t.Fatalf("expected AS path length 1, got %d", u.ASPathLen)
	}
}

func TestInboundAutoAcceptAndReject(t *testing.T) {
	srv := startTestServer(t, testConfig(65000))
	srv.SetRoutes([]string{"10.10.10.0/24", "192.0.2.0/24", "2001:db8::/32"})
	srv.SetAllowlist([]uint32{65001}, true)

	// Accepted ASN.
	conn, notifs := testPeer(t, srv.Addr(), 65001, true)
	if len(notifs) != 1 {
		sets := readUpdates(t, conn, 2, 3*time.Second)
		if len(sets) == 0 {
			t.Fatal("expected route announcements for accepted peer")
		}
		var v4Count, v6Count int
		for _, set := range sets {
			v4Count += len(set.update.Announced)
			v6Count += len(set.update.MPAnnounced)
		}
		if v4Count != 2 {
			t.Fatalf("expected 2 IPv4 prefixes, got %d", v4Count)
		}
		if v6Count != 1 {
			t.Fatalf("expected 1 IPv6 prefix, got %d", v6Count)
		}
	}

	// Rejected ASN.
	conn2, notifs2 := testPeer(t, srv.Addr(), 65002, false)
	if len(notifs2) != 1 || notifs2[0].Type != msgNotification {
		t.Fatal("expected NOTIFICATION for non-allowlisted ASN")
	}
	_ = conn2
}

func TestActivePeerSession(t *testing.T) {
	peerServer := startTestServer(t, testConfig(65000))
	peerServer.SetRoutes([]string{"203.0.113.0/24"})
	peerServer.SetAllowlist([]uint32{65010}, true)

	dialer := startTestServer(t, testConfig(65010))
	host, port, err := net.SplitHostPort(peerServer.Addr())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	dialer.SetPeers([]PeerConfig{{
		Name:     "peer-a",
		Neighbor: host,
		Port:     portNum,
		RemoteAS: 65000,
		Enabled:  true,
	}})

	waitFor(t, 5*time.Second, func() bool {
		stats := dialer.Stats()
		for _, s := range stats {
			if s.State == stateEstablished {
				return true
			}
		}
		return false
	})

	waitFor(t, 5*time.Second, func() bool {
		stats := peerServer.Stats()
		for _, s := range stats {
			if s.State == stateEstablished && s.RemoteAS == 65010 {
				return true
			}
		}
		return false
	})
}

func TestSetRoutesWithdrawsRemovedPrefixes(t *testing.T) {
	srv := startTestServer(t, testConfig(65000))
	srv.SetRoutes([]string{"10.1.0.0/16", "10.2.0.0/16"})
	srv.SetAllowlist([]uint32{65001}, true)

	conn, notifs := testPeer(t, srv.Addr(), 65001, false)
	if len(notifs) == 1 {
		t.Fatal("peer was unexpectedly rejected")
	}
	first := readUpdates(t, conn, 1, 3*time.Second)
	if len(first) != 1 || len(first[0].update.Announced) != 2 {
		t.Fatalf("expected initial 2 prefixes, got %+v", first)
	}

	srv.SetRoutes([]string{"10.1.0.0/16"})
	var sawWithdraw bool
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !sawWithdraw {
		_ = conn.SetReadDeadline(deadline)
		msg, err := readMessage(conn)
		if err != nil {
			break
		}
		if msg.Type != msgUpdate {
			continue
		}
		u, err := decodeUpdate(msg.Body)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(u.Withdrawn) == 1 && u.Withdrawn[0].Bits == 16 {
			sawWithdraw = true
		}
	}
	if !sawWithdraw {
		t.Fatal("expected withdrawal for removed prefix")
	}
}

func TestIBGPUpdateOmitsASPathAndSetsLocalPref(t *testing.T) {
	raw := encodeUpdateV4([][4]byte{{10, 0, 0, 0}}, []int{8}, 65000, true, [4]byte{1, 2, 3, 4}, nil, nil, true)
	msg, err := readMessage(bytesReader(raw))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	u, err := decodeUpdate(msg.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.ASPathLen != 0 {
		t.Fatalf("iBGP UPDATE must not prepend AS_PATH, got len %d", u.ASPathLen)
	}
	if u.LocalPref != 100 {
		t.Fatalf("expected LOCAL_PREF 100, got %d", u.LocalPref)
	}
}

func TestSetPeersRemovalStopsSession(t *testing.T) {
	peerServer := startTestServer(t, testConfig(65000))
	peerServer.SetAllowlist([]uint32{65010}, true)

	dialer := startTestServer(t, testConfig(65010))
	host, port, err := net.SplitHostPort(peerServer.Addr())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	dialer.SetPeers([]PeerConfig{{
		Name:     "peer-a",
		Neighbor: host,
		Port:     portNum,
		RemoteAS: 65000,
		Enabled:  true,
	}})

	waitFor(t, 5*time.Second, func() bool {
		for _, s := range dialer.Stats() {
			if s.State == stateEstablished {
				return true
			}
		}
		return false
	})

	dialer.SetPeers(nil)
	waitFor(t, 5*time.Second, func() bool {
		return len(dialer.Stats()) == 0
	})

	// Ensure the removed peer is not reconnected after the backoff window.
	time.Sleep(1500 * time.Millisecond)
	if got := len(dialer.Stats()); got != 0 {
		t.Fatalf("expected no sessions after peer removal, got %d", got)
	}
}

func TestIBGPSessionAttributes(t *testing.T) {
	srv := startTestServer(t, testConfig(65000))
	srv.SetRoutes([]string{"10.9.0.0/16"})
	srv.SetAllowlist([]uint32{65000}, true)

	conn, notifs := testPeer(t, srv.Addr(), 65000, false)
	if len(notifs) == 1 {
		t.Fatal("iBGP peer was unexpectedly rejected")
	}
	sets := readUpdates(t, conn, 1, 3*time.Second)
	if len(sets) == 0 {
		t.Fatal("expected iBGP route announcement")
	}
	u := sets[0].update
	if u.ASPathLen != 0 {
		t.Fatalf("iBGP announcement must not prepend AS_PATH, got %d", u.ASPathLen)
	}
	if u.LocalPref != 100 {
		t.Fatalf("expected LOCAL_PREF 100, got %d", u.LocalPref)
	}
}

func TestSetPeersConfigChangeStopsOldSession(t *testing.T) {
	peerServer := startTestServer(t, testConfig(65000))
	peerServer.SetAllowlist([]uint32{65010}, true)

	dialer := startTestServer(t, testConfig(65010))
	host, port, err := net.SplitHostPort(peerServer.Addr())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	peer := PeerConfig{
		Name:     "peer-a",
		Neighbor: host,
		Port:     portNum,
		RemoteAS: 65000,
		Enabled:  true,
	}
	dialer.SetPeers([]PeerConfig{peer})

	waitFor(t, 5*time.Second, func() bool {
		for _, s := range dialer.Stats() {
			if s.State == stateEstablished {
				return true
			}
		}
		return false
	})

	// Changing the configured remote AS must tear down the live session and
	// replace it, not leave the old reconnect loop running.
	peer.RemoteAS = 65011
	dialer.SetPeers([]PeerConfig{peer})

	waitFor(t, 5*time.Second, func() bool {
		for _, s := range dialer.Stats() {
			if s.State == stateEstablished {
				return false
			}
		}
		for _, s := range peerServer.Stats() {
			if s.State == stateEstablished {
				return false
			}
		}
		return true
	})

	dialer.SetPeers(nil)
}

// bytesReader adapts a byte slice to io.Reader without importing bytes in the
// test hot path.
type byteReader struct {
	data []byte
	pos  int
}

func bytesReader(b []byte) *byteReader { return &byteReader{data: b} }

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
