package engine

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"rtbhid/internal/agent/bird"
)

func TestCustomApplyAndStatus(t *testing.T) {
	var logBuf bytes.Buffer
	custom, err := NewCustom(CustomConfig{
		LocalASN:    65000,
		ListenAddr:  "127.0.0.1:0",
		HoldTimeSec: 5,
		EnableIPv6:  true,
		Logger:      log.New(&logBuf, "", 0),
	})
	if err != nil {
		t.Fatalf("start custom engine: %v", err)
	}
	defer custom.Close()

	result, err := custom.Apply(
		[]string{"10.0.0.0/8", "2001:db8::/32"},
		[]bird.Peer{{
			ID:          "peer-1",
			Name:        "peer-1",
			Neighbor:    "127.0.0.2",
			NeighborASN: 65001,
			Enabled:     true,
			Password:    "secret",
		}},
		[]uint32{7713},
		true,
	)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.RouteCount != 2 || result.PeerCount != 1 {
		t.Fatalf("unexpected apply result: %+v", result)
	}

	name, sessions := custom.BGPStatus()
	if name != "custom" {
		t.Fatalf("expected engine name custom, got %q", name)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no live sessions, got %+v", sessions)
	}
	if !strings.Contains(logBuf.String(), "TCP MD5") {
		t.Fatalf("expected TCP MD5 limitation warning, got %q", logBuf.String())
	}
}
