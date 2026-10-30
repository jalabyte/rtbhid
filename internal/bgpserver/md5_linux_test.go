package bgpserver

import (
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"
)

func TestMD5LinuxLoopback(t *testing.T) {
	srv := startTestServer(t, testConfig(65000))
	// Port 1: keep the outbound dial loop away from the test listener.
	srv.SetPeers([]PeerConfig{{Name: "md5", Neighbor: "127.0.0.1", Port: 1, RemoteAS: 65001, Password: "s3cret", Enabled: true}})

	signed := net.Dialer{Timeout: 2 * time.Second, Control: func(n, _ string, c syscall.RawConn) error {
		return setMD5(c, n, netip.MustParseAddr("127.0.0.1"), "s3cret")
	}}
	conn, err := signed.Dial("tcp4", srv.Addr())
	if err != nil {
		t.Fatalf("signed dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if msg, err := readMessage(conn); err != nil || msg.Type != msgOpen {
		t.Fatalf("expected OPEN over signed session, got %v %v", msg, err)
	}

	if c, err := net.DialTimeout("tcp4", srv.Addr(), time.Second); err == nil {
		c.Close()
		t.Fatal("unsigned connection must be dropped by the kernel")
	}
}
