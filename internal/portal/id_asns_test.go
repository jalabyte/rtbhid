package portal

import (
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
		"",
	}, "\n")

	asns, err := ParseAPNICIDASNs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []uint32{7713, 9341, 133000, 133001}
	if len(asns) != len(want) {
		t.Fatalf("got %v, want %v", asns, want)
	}
	for i := range want {
		if asns[i] != want[i] {
			t.Fatalf("got %v, want %v", asns, want)
		}
	}
}

func TestParseAPNICIDASNsEmpty(t *testing.T) {
	if _, err := ParseAPNICIDASNs(strings.NewReader("apnic|SG|asn|3758|1|x|allocated\n")); err == nil {
		t.Fatal("expected error when no Indonesian ASNs are present")
	}
}
