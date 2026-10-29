package portal

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pb "rtbhid/gen/rtbhid"
	"rtbhid/internal/portal/state"
)

func TestBuildConfigUpdateIncludesWhitelist(t *testing.T) {
	svc := newTestService(t)
	if err := svc.store.ReplaceIDASNRegistry([]uint32{7713, 9341}); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	if _, err := svc.AddWhitelistASN(7713, "", false); err != nil {
		t.Fatalf("add 7713: %v", err)
	}
	if _, err := svc.AddWhitelistASN(9341, "", false); err != nil {
		t.Fatalf("add 9341: %v", err)
	}
	entry, ok := svc.store.GetWhitelistASN(9341)
	if !ok {
		t.Fatal("missing 9341")
	}
	if err := svc.ToggleWhitelistASN(entry.ID, false); err != nil {
		t.Fatalf("disable 9341: %v", err)
	}

	update := svc.buildConfigUpdate()
	if !update.AutoAccept {
		t.Fatal("expected auto_accept true by default")
	}
	if !reflect.DeepEqual(update.AllowedAsns, []uint32{7713}) {
		t.Fatalf("expected only enabled ASN 7713, got %v", update.AllowedAsns)
	}

	svc.SetAutoAccept(false)
	next := svc.buildConfigUpdate()
	if next.AutoAccept {
		t.Fatal("expected auto_accept false after toggle")
	}
	if next.Version <= update.Version {
		t.Fatalf("expected version to increase, got %d then %d", update.Version, next.Version)
	}
}

func receiveUpdate(t *testing.T, ch <-chan *pb.ConfigUpdate) *pb.ConfigUpdate {
	t.Helper()
	select {
	case update := <-ch:
		if update == nil {
			t.Fatal("config stream closed unexpectedly")
		}
		return update
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for config update")
		return nil
	}
}

func TestConfigStreamPushesWhitelistChanges(t *testing.T) {
	svc := newTestService(t)
	if err := svc.store.ReplaceIDASNRegistry([]uint32{7713}); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	if _, err := svc.AddWhitelistASN(7713, "", false); err != nil {
		t.Fatalf("add whitelist: %v", err)
	}

	ch := svc.RegisterStream("agent-stream")
	first := receiveUpdate(t, ch)
	if !reflect.DeepEqual(first.AllowedAsns, []uint32{7713}) {
		t.Fatalf("initial stream update missing whitelist: %v", first.AllowedAsns)
	}
	if !first.AutoAccept {
		t.Fatal("initial stream update should have auto_accept true")
	}

	svc.SetAutoAccept(false)
	second := receiveUpdate(t, ch)
	if second.AutoAccept {
		t.Fatal("stream did not deliver auto_accept=false")
	}
	if second.Version <= first.Version {
		t.Fatalf("stream version did not increase: %d then %d", first.Version, second.Version)
	}

	svc.UnregisterStream("agent-stream")
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for stream close")
		}
	}
}

func TestConfigUpdateLargeWhitelistFitsGRPC(t *testing.T) {
	svc := newTestService(t)
	entries := make([]state.WhitelistASN, 0, 5000)
	for i := 0; i < 5000; i++ {
		entries = append(entries, state.WhitelistASN{
			ID:      fmt.Sprintf("asn-%d", i),
			ASN:     uint32(100000 + i),
			Country: "ID",
			Source:  "rir",
			Enabled: true,
		})
	}
	if _, err := svc.store.ReplaceWhitelistBySources([]string{"rir"}, entries); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}

	update := svc.buildConfigUpdate()
	if len(update.AllowedAsns) != 5000 {
		t.Fatalf("expected 5000 allowed ASNs, got %d", len(update.AllowedAsns))
	}
	payload, err := proto.Marshal(update)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if len(payload) >= 4*1024*1024 {
		t.Fatalf("config update exceeds default gRPC 4MB limit: %d bytes", len(payload))
	}
	t.Logf("5000-ASN config update is %d bytes", len(payload))
}
