package state

import (
	"testing"
)

func TestWhitelistCRUD(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	entry := WhitelistASN{ID: "asn-test-1", ASN: 7713, Name: "Test ISP", Enabled: true}
	created, err := store.CreateWhitelistASN(entry)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Country != "ID" || created.Source != "manual" {
		t.Fatalf("unexpected defaults: %+v", created)
	}

	all := store.ListWhitelistASNs()
	if len(all) != 1 || all[0].ASN != 7713 {
		t.Fatalf("unexpected list: %+v", all)
	}
	if got := store.ListEnabledWhitelistASNs(); len(got) != 1 || got[0] != 7713 {
		t.Fatalf("unexpected enabled list: %+v", got)
	}
	if _, ok := store.GetWhitelistASN(7713); !ok {
		t.Fatal("expected lookup by ASN to succeed")
	}

	if err := store.ToggleWhitelistASN("asn-test-1", false); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if got := store.ListEnabledWhitelistASNs(); len(got) != 0 {
		t.Fatalf("expected no enabled ASNs, got %+v", got)
	}

	if err := store.DeleteWhitelistASN("asn-test-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := store.CountWhitelistASNs(); got != 0 {
		t.Fatalf("expected empty whitelist, got %d", got)
	}
}

func TestReplaceWhitelistBySourcePreservesManual(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	if _, err := store.CreateWhitelistASN(WhitelistASN{ID: "manual-1", ASN: 9000, Source: "manual", Enabled: true}); err != nil {
		t.Fatalf("create manual: %v", err)
	}

	imported := []WhitelistASN{
		{ID: "apnic-1", ASN: 7713, Source: "apnic", Enabled: true},
		{ID: "apnic-2", ASN: 9341, Source: "apnic", Enabled: true},
	}
	created, err := store.ReplaceWhitelistBySource("apnic", imported)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected 2 created, got %d", created)
	}

	// Re-import replaces the APNIC portion, keeps manual rows.
	created, err = store.ReplaceWhitelistBySource("apnic", imported[:1])
	if err != nil {
		t.Fatalf("replace again: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 created on refresh, got %d", created)
	}
	all := store.ListWhitelistASNs()
	if len(all) != 2 {
		t.Fatalf("expected manual + one apnic, got %+v", all)
	}
}
