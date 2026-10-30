package state

import "testing"

func TestAgentBGPSessions(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	if err := store.RegisterAgent(Agent{ID: "agent-1", Name: "router-01", Status: "online"}); err != nil {
		t.Fatalf("register agent: %v", err)
	}

	sessions := []BGPSession{
		{AgentID: "agent-1", Peer: "192.0.2.1", Name: "peer-a", RemoteAS: 7713, LocalAS: 65000, State: "Established", Passive: false, UptimeSec: 120, PrefixesIn: 3},
		{AgentID: "agent-1", Peer: "192.0.2.2", Name: "peer-b", RemoteAS: 9341, LocalAS: 65000, State: "OpenSent", Passive: true, LastError: "hold timer"},
	}
	if err := store.ReplaceAgentBGPSessions("agent-1", "custom", sessions); err != nil {
		t.Fatalf("replace sessions: %v", err)
	}

	got := store.ListAgentBGPSessions()
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(got))
	}
	if got[0].AgentName != "router-01" {
		t.Fatalf("expected agent name join, got %q", got[0].AgentName)
	}
	engines := store.AgentBGPEngines()
	if engines["agent-1"] != "custom" {
		t.Fatalf("expected engine custom, got %q", engines["agent-1"])
	}

	// Replacing with fewer sessions must remove stale rows.
	if err := store.ReplaceAgentBGPSessions("agent-1", "custom", sessions[:1]); err != nil {
		t.Fatalf("replace again: %v", err)
	}
	if got := store.ListAgentBGPSessions(); len(got) != 1 {
		t.Fatalf("expected stale session removed, got %d", len(got))
	}

	if err := store.DeleteAgent("agent-1"); err != nil {
		t.Fatalf("delete agent: %v", err)
	}
	if got := store.ListAgentBGPSessions(); len(got) != 0 {
		t.Fatalf("expected telemetry cleared with agent, got %+v", got)
	}
	if _, ok := store.AgentBGPEngines()["agent-1"]; ok {
		t.Fatal("expected BGP engine status cleared with agent")
	}
}
