package portal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseAPNICIDASNs(t *testing.T) {
	input := strings.Join([]string{
		"2|apnic|20260101|1|19850701|20260101|+0000",
		"apnic|ID|asn|7713|1|19991022|allocated",
		"apnic|ID|asn|9341|1|19991022|allocated",
		"apnic|SG|asn|3758|1|19991022|allocated",
		"apnic|ID|asn|133000|2|20140101|allocated",
		"apnic|ID|ipv4|1.1.1.0|256|20110101|allocated",
		"apnic|ID|asn|not-a-number|1|20110101|allocated",
		// Country-ID ASNs can be delegated by other RIRs.
		"arin|ID|asn|64500|1|20100101|allocated",
		"ripencc|ID|asn|64501|1|20100101|allocated",
		"lacnic|ID|asn|64502|1|20100101|allocated",
		"afrinic|ID|asn|64503|1|20100101|allocated",
		"ripencc|NL|asn|64504|1|20100101|allocated",
		"",
	}, "\n")

	asns, err := ParseIDASNs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []uint32{7713, 9341, 133000, 133001, 64500, 64501, 64502, 64503}
	if len(asns) != len(want) {
		t.Fatalf("got %v, want %v", asns, want)
	}
	for i := range want {
		if asns[i] != want[i] {
			t.Fatalf("got %v, want %v", asns, want)
		}
	}
}

func TestIDASNRegistryFetchMergesSourcesAndWarns(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/apnic", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "apnic|ID|asn|7713|1|19991022|allocated\n")
	})
	mux.HandleFunc("/arin", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "arin|ID|asn|402969|1|20260831|assigned\n")
	})
	mux.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	reg := NewIDASNRegistryURLs([]string{srv.URL + "/apnic", srv.URL + "/arin", srv.URL + "/fail"})
	asns, err := reg.Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(asns) != 2 || asns[0] != 7713 || asns[1] != 402969 {
		t.Fatalf("unexpected ASNs: %v", asns)
	}
	if warnings := reg.Warnings(); len(warnings) != 1 {
		t.Fatalf("expected one source warning, got %v", warnings)
	}
}

func TestParseIDASNsEmptySource(t *testing.T) {
	asns, err := ParseIDASNs(strings.NewReader("ripencc|NL|asn|3758|1|x|allocated\n"))
	if err != nil {
		t.Fatalf("empty source should not error: %v", err)
	}
	if len(asns) != 0 {
		t.Fatalf("expected no ASNs, got %v", asns)
	}
}
