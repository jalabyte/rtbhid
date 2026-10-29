package state

import "testing"

// TestMigrationsIdempotent verifies that opening an existing database a second
// time (the upgrade path) re-runs all CREATE TABLE IF NOT EXISTS migrations
// without error and that the new tables are usable.
func TestMigrationsIdempotent(t *testing.T) {
	dbPath := t.TempDir() + "/portal.db"

	first, err := New(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	second, err := New(dbPath)
	if err != nil {
		t.Fatalf("second open (upgrade path): %v", err)
	}
	defer second.Close()

	if err := second.ReplaceIDASNRegistry([]uint32{7713}); err != nil {
		t.Fatalf("registry after reopen: %v", err)
	}
	if !second.IsIDASNRegistered(7713) {
		t.Fatal("expected registry row after reopen")
	}
	if _, err := second.ReplaceWhitelistBySources([]string{"rir"}, []WhitelistASN{
		{ID: "asn-1", ASN: 7713, Country: "ID", Source: "rir", Enabled: true},
	}); err != nil {
		t.Fatalf("whitelist after reopen: %v", err)
	}
	if err := second.RegisterAgent(Agent{ID: "agent-1", Name: "router"}); err != nil {
		t.Fatalf("register agent after reopen: %v", err)
	}
	if err := second.ReplaceAgentBGPSessions("agent-1", "custom", []BGPSession{
		{Peer: "192.0.2.1", State: "Established"},
	}); err != nil {
		t.Fatalf("telemetry after reopen: %v", err)
	}
	if got := len(second.ListAgentBGPSessions()); got != 1 {
		t.Fatalf("expected one session after reopen, got %d", got)
	}
	if err := second.DeleteAgent("agent-1"); err != nil {
		t.Fatalf("delete agent after reopen: %v", err)
	}
	if got := len(second.ListAgentBGPSessions()); got != 0 {
		t.Fatalf("expected telemetry cleanup after reopen, got %d", got)
	}
}
