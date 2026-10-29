package portal

import (
	"testing"

	"rtbhid/internal/portal/state"
	"rtbhid/internal/syncer"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	store, err := state.New(t.TempDir() + "/portal.db")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewService(store, syncer.NewFetcher("rtbhid-test/1.0"), 65000, 300, "")
}

func TestStrictIDWhitelistValidation(t *testing.T) {
	svc := newTestService(t)

	// Strict by default; empty registry blocks manual adds.
	if _, err := svc.AddWhitelistASN(7713, "", false); err == nil {
		t.Fatal("expected error with an empty Indonesian registry")
	}

	if err := svc.store.ReplaceIDASNRegistry([]uint32{7713, 9341}); err != nil {
		t.Fatalf("seed registry: %v", err)
	}

	if _, err := svc.AddWhitelistASN(7713, "ID ISP", false); err != nil {
		t.Fatalf("expected registered ID ASN to be accepted: %v", err)
	}
	entry, ok := svc.store.GetWhitelistASN(7713)
	if !ok || entry.Country != "ID" {
		t.Fatalf("expected country ID, got %+v", entry)
	}

	if _, err := svc.AddWhitelistASN(65002, "", false); err == nil {
		t.Fatal("expected non-ID ASN to be rejected in strict mode")
	}

	override, err := svc.AddWhitelistASN(65002, "test", true)
	if err != nil {
		t.Fatalf("allow_non_id override failed: %v", err)
	}
	if override.Country == "ID" {
		t.Fatalf("non-ID override should not be marked ID: %+v", override)
	}

	svc.SetStrictIDWhitelist(false)
	if _, err := svc.AddWhitelistASN(65003, "", false); err != nil {
		t.Fatalf("strict mode off should allow non-ID ASN: %v", err)
	}
}
