package state

import (
	"testing"
	"time"
)

// setupTestStore creates a new in-memory store for testing
func setupTestStore(t *testing.T) *Store {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}

// createTestSource creates a source with prefixes for testing
func createTestSource(t *testing.T, store *Store, name string, cidrs []string) string {
	src := Source{
		ID:        "test-source-" + name,
		Name:      name,
		URL:       "http://test.example.com/" + name,
		Interval:  3600,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateSource(src); err != nil {
		t.Fatalf("failed to create test source: %v", err)
	}
	if err := store.ReplaceSourcePrefixes(src.ID, cidrs); err != nil {
		t.Fatalf("failed to replace source prefixes: %v", err)
	}
	return src.ID
}

// createTestPrefix creates a manual prefix for testing
func createTestPrefix(t *testing.T, store *Store, cidr string, enabled bool) int64 {
	p := Prefix{
		CIDR:      cidr,
		Enabled:   enabled,
		Note:      "test prefix",
		CreatedAt: time.Now().UTC(),
	}
	result, err := store.CreatePrefix(p)
	if err != nil {
		t.Fatalf("failed to create test prefix: %v", err)
	}
	return result.ID
}

func TestLookupIP_SingleIPv4(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a source with IPv4 prefix
	createTestSource(t, store, "ipv4", []string{"192.168.1.0/24"})

	// Test: Single IPv4 lookup should match the /24 prefix
	results, err := store.LookupIP([]string{"192.168.1.1"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Query != "192.168.1.1" {
		t.Errorf("expected query '192.168.1.1', got '%s'", results[0].Query)
	}

	if len(results[0].Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results[0].Matches))
	}

	if results[0].Matches[0].CIDR != "192.168.1.0/24" {
		t.Errorf("expected CIDR '192.168.1.0/24', got '%s'", results[0].Matches[0].CIDR)
	}
}

func TestLookupIP_SingleIPv6(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a source with IPv6 prefix
	createTestSource(t, store, "ipv6", []string{"2001:db8::/32"})

	// Test: Single IPv6 lookup should match the /32 prefix
	results, err := store.LookupIP([]string{"2001:db8::1"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Query != "2001:db8::1" {
		t.Errorf("expected query '2001:db8::1', got '%s'", results[0].Query)
	}

	if len(results[0].Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results[0].Matches))
	}

	if results[0].Matches[0].CIDR != "2001:db8::/32" {
		t.Errorf("expected CIDR '2001:db8::/32', got '%s'", results[0].Matches[0].CIDR)
	}
}

func TestLookupIP_CIDRPrefixLookup(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a source with specific prefix
	createTestSource(t, store, "cidr", []string{"192.168.1.0/24"})

	// Test: Searching for exact CIDR should match that prefix
	results, err := store.LookupIP([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results[0].Matches))
	}

	if results[0].Matches[0].CIDR != "192.168.1.0/24" {
		t.Errorf("expected CIDR '192.168.1.0/24', got '%s'", results[0].Matches[0].CIDR)
	}
}

func TestLookupIP_BulkLookup(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create sources with various prefixes
	createTestSource(t, store, "bulk", []string{
		"192.168.1.0/24",
		"192.168.2.0/24",
		"10.0.0.0/8",
	})

	// Test: Bulk lookup with multiple IPs
	queries := []string{
		"192.168.1.1",
		"192.168.2.5",
		"10.5.6.7",
		"172.16.0.1", // Should not match any
	}

	results, err := store.LookupIP(queries)
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// First three should have matches
	if len(results[0].Matches) != 1 || results[0].Matches[0].CIDR != "192.168.1.0/24" {
		t.Errorf("expected match for 192.168.1.0/24, got %v", results[0].Matches)
	}

	if len(results[1].Matches) != 1 || results[1].Matches[0].CIDR != "192.168.2.0/24" {
		t.Errorf("expected match for 192.168.2.0/24, got %v", results[1].Matches)
	}

	if len(results[2].Matches) != 1 || results[2].Matches[0].CIDR != "10.0.0.0/8" {
		t.Errorf("expected match for 10.0.0.0/8, got %v", results[2].Matches)
	}

	// Fourth should have no matches
	if len(results[3].Matches) != 0 {
		t.Errorf("expected no matches for 172.16.0.1, got %d matches", len(results[3].Matches))
	}
}

func TestLookupIP_InvalidInput(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Test: Invalid IP should return error
	_, err := store.LookupIP([]string{"999.999.999.999"})
	if err == nil {
		t.Fatal("expected error for invalid IP, got nil")
	}
}

func TestLookupIP_EmptyResult(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a source with limited prefixes
	createTestSource(t, store, "limited", []string{"192.168.1.0/24"})

	// Test: IP that doesn't match any prefix should return empty matches
	results, err := store.LookupIP([]string{"10.0.0.1"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 0 {
		t.Errorf("expected 0 matches for non-matching IP, got %d", len(results[0].Matches))
	}
}

func TestLookupIP_HierarchicalOrdering(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a source with overlapping prefixes (different specificity)
	createTestSource(t, store, "hierarchy", []string{
		"192.168.0.0/16",
		"192.168.1.0/24",
		"192.168.1.128/25",
	})

	// Test: Lookup should return matches ordered by most specific (longest prefix) first
	results, err := store.LookupIP([]string{"192.168.1.200"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Should have 3 matches, ordered by prefix length (most specific first)
	if len(results[0].Matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(results[0].Matches))
	}

	// Most specific first (/25)
	if results[0].Matches[0].CIDR != "192.168.1.128/25" {
		t.Errorf("expected first match to be '192.168.1.128/25' (most specific), got '%s'", results[0].Matches[0].CIDR)
	}

	// Then /24
	if results[0].Matches[1].CIDR != "192.168.1.0/24" {
		t.Errorf("expected second match to be '192.168.1.0/24', got '%s'", results[0].Matches[1].CIDR)
	}

	// Then /16
	if results[0].Matches[2].CIDR != "192.168.0.0/16" {
		t.Errorf("expected third match to be '192.168.0.0/16', got '%s'", results[0].Matches[2].CIDR)
	}
}

func TestLookupIP_MixedSourcesAndManual(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a source with prefixes
	sourceID := createTestSource(t, store, "mixed", []string{"192.168.1.0/24"})

	// Create a manual prefix
	createTestPrefix(t, store, "10.0.0.0/8", true)

	// Test: Lookup should match both source and manual prefixes
	results, err := store.LookupIP([]string{"192.168.1.5", "10.5.6.7"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First IP should match source prefix
	if len(results[0].Matches) != 1 {
		t.Errorf("expected 1 match for source IP, got %d", len(results[0].Matches))
	} else {
		if results[0].Matches[0].CIDR != "192.168.1.0/24" {
			t.Errorf("expected CIDR '192.168.1.0/24', got '%s'", results[0].Matches[0].CIDR)
		}
		if results[0].Matches[0].Type != "source" {
			t.Errorf("expected type 'source', got '%s'", results[0].Matches[0].Type)
		}
		if results[0].Matches[0].SourceID != sourceID {
			t.Errorf("expected source ID '%s', got '%s'", sourceID, results[0].Matches[0].SourceID)
		}
	}

	// Second IP should match manual prefix
	if len(results[1].Matches) != 1 {
		t.Errorf("expected 1 match for manual IP, got %d", len(results[1].Matches))
	} else {
		if results[1].Matches[0].CIDR != "10.0.0.0/8" {
			t.Errorf("expected CIDR '10.0.0.0/8', got '%s'", results[1].Matches[0].CIDR)
		}
		if results[1].Matches[0].Type != "manual" {
			t.Errorf("expected type 'manual', got '%s'", results[1].Matches[0].Type)
		}
	}
}

func TestLookupIP_DisabledSourceExcluded(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create an enabled source
	createTestSource(t, store, "enabled", []string{"192.168.1.0/24"})

	// Create a disabled source
	src := Source{
		ID:        "disabled-source",
		Name:      "disabled",
		URL:       "http://test.example.com/disabled",
		Interval:  3600,
		Enabled:   false,
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateSource(src); err != nil {
		t.Fatalf("failed to create disabled source: %v", err)
	}
	if err := store.ReplaceSourcePrefixes(src.ID, []string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("failed to replace source prefixes: %v", err)
	}

	// Test: IP in disabled source should not match
	results, err := store.LookupIP([]string{"10.0.0.1"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 0 {
		t.Errorf("expected 0 matches for disabled source, got %d", len(results[0].Matches))
	}

	// But enabled source should still match
	results2, err := store.LookupIP([]string{"192.168.1.1"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results2) != 1 || len(results2[0].Matches) != 1 {
		t.Errorf("expected 1 match for enabled source, got %d", len(results2[0].Matches))
	}
}

func TestLookupIP_ExcludeListPreventsMatch(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	// Create a source with prefixes
	createTestSource(t, store, "excludetest", []string{"192.168.1.0/24"})

	// Add to exclude list
	ex := Exclude{
		CIDR:      "192.168.1.0/24",
		Note:      "test exclude",
		CreatedAt: time.Now().UTC(),
	}
	if _, err := store.CreateExclude(ex); err != nil {
		t.Fatalf("failed to create exclude: %v", err)
	}

	// Test: IP in excluded prefix should not match
	results, err := store.LookupIP([]string{"192.168.1.1"})
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if len(results[0].Matches) != 0 {
		t.Errorf("expected 0 matches for excluded prefix, got %d", len(results[0].Matches))
	}
}
