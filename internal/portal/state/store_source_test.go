package state

import "testing"

func TestSourceSyncErrorRecordedAndCleared(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	if err := store.CreateSource(Source{ID: "src-1", Name: "feed", URL: "http://x", Interval: 300, Enabled: true}); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if err := store.SetSourceSyncError("src-1", "no such host"); err != nil {
		t.Fatalf("set error: %v", err)
	}
	if got, _ := store.GetSource("src-1"); got.LastError != "no such host" {
		t.Fatalf("last_error = %q, want %q", got.LastError, "no such host")
	}
	if err := store.SetSourceSyncResult("src-1", 5); err != nil {
		t.Fatalf("set result: %v", err)
	}
	if got := store.ListSources(); len(got) != 1 || got[0].LastError != "" || got[0].LastCount != 5 {
		t.Fatalf("after success: %+v", got)
	}

	if err := store.CreateExcludeSource(Source{ID: "ex-1", Name: "own", URL: "http://y", Interval: 300, Enabled: true}); err != nil {
		t.Fatalf("create exclude source: %v", err)
	}
	if err := store.SetExcludeSourceSyncError("ex-1", "timeout"); err != nil {
		t.Fatalf("set exclude error: %v", err)
	}
	if got := store.ListExcludeSources(); len(got) != 1 || got[0].LastError != "timeout" {
		t.Fatalf("exclude source: %+v", got)
	}
}
