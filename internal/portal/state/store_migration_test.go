package state

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

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

// TestUpgradeFromLegacySchema builds a database with only the pre-feature
// tables and rows, then opens it with the current migrations.
func TestUpgradeFromLegacySchema(t *testing.T) {
	dbPath := t.TempDir() + "/legacy.db"

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	for _, migration := range migrations {
		if strings.Contains(migration, "asn_whitelist") ||
			strings.Contains(migration, "id_asn_registry") ||
			strings.Contains(migration, "agent_bgp") {
			continue
		}
		if _, err := raw.Exec(migration); err != nil {
			t.Fatalf("legacy migration: %v", err)
		}
	}
	if _, err := raw.Exec(`INSERT INTO agents (id, name, status) VALUES ('legacy', 'legacy-router', 'online')`); err != nil {
		t.Fatalf("insert legacy agent: %v", err)
	}
	if _, err := raw.Exec(`
		INSERT INTO peers (id, name, neighbor_ip, remote_as, local_as, multihop, enabled, std_communities, large_communities)
		VALUES ('peer-legacy', 'legacy-peer', '192.0.2.1', 65001, 65000, 1, 1, '[]', '[]')`); err != nil {
		t.Fatalf("insert legacy peer: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("upgrade open: %v", err)
	}
	defer store.Close()

	if _, ok := store.GetAgent("legacy"); !ok {
		t.Fatal("legacy agent row was not preserved")
	}
	if got := len(store.ListPeers()); got != 1 {
		t.Fatalf("legacy peer row was not preserved, got %d peers", got)
	}

	// New tables must be usable after the upgrade.
	if err := store.ReplaceIDASNRegistry([]uint32{7713}); err != nil {
		t.Fatalf("registry after upgrade: %v", err)
	}
	if _, err := store.ReplaceWhitelistBySources([]string{"rir"}, []WhitelistASN{
		{ID: "asn-1", ASN: 7713, Country: "ID", Source: "rir", Enabled: true},
	}); err != nil {
		t.Fatalf("whitelist after upgrade: %v", err)
	}
	if err := store.ReplaceAgentBGPSessions("legacy", "custom", []BGPSession{
		{Peer: "192.0.2.1", State: "Established"},
	}); err != nil {
		t.Fatalf("telemetry after upgrade: %v", err)
	}
	if got := len(store.ListAgentBGPSessions()); got != 1 {
		t.Fatalf("expected one telemetry session after upgrade, got %d", got)
	}
}
