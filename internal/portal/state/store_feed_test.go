package state

import (
	"slices"
	"testing"
)

func TestEffectivePrefixesDropsTooWide(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	if err := store.CreateSource(Source{ID: "src-1", Name: "feed", URL: "http://x", Interval: 300, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceSourcePrefixes("src-1", []string{"0.0.0.0/0", "0.0.0.0/1", "::/0", "2000::/3", "10.0.0.0/8", "198.51.100.7/32", "2001:db8::/32"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePrefix(Prefix{CIDR: "128.0.0.0/1", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	got := store.EffectivePrefixes()
	want := []string{"10.0.0.0/8", "198.51.100.7/32", "2001:db8::/32"}
	if !slices.Equal(got, want) {
		t.Fatalf("feed = %v, want %v", got, want)
	}
}
