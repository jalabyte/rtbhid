package grpcclient

import (
	"testing"

	"rtbhid/internal/agent/engine"
)

func TestNewSelectsEngine(t *testing.T) {
	custom, err := New(Config{
		BGPEngine:      "custom",
		BGPListenAddr:  "127.0.0.1:0",
		BGPHoldTimeSec: 5,
		LocalASN:       65000,
	})
	if err != nil {
		t.Fatalf("custom engine: %v", err)
	}
	if _, ok := custom.engine.(*engine.Custom); !ok {
		t.Fatalf("expected custom engine, got %T", custom.engine)
	}
	custom.engine.Close()

	if _, err := New(Config{BGPEngine: "bogus"}); err == nil {
		t.Fatal("expected an error for an unknown engine")
	}

	dir := t.TempDir()
	birdClient, err := New(Config{
		BGPEngine:       "bird",
		BirdOutputPath:  dir + "/routes.conf",
		BirdPeersOutput: dir + "/peers.conf",
		LocalASN:        65000,
	})
	if err != nil {
		t.Fatalf("bird engine: %v", err)
	}
	if _, ok := birdClient.engine.(*engine.BirdManager); !ok {
		t.Fatalf("expected bird engine, got %T", birdClient.engine)
	}
	birdClient.engine.Close()
}
