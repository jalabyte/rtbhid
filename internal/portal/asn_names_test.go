package portal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseASNNames(t *testing.T) {
	input := strings.Join([]string{
		"# comment",
		"1 LVLT-1 - Level 3 Parent, LLC, US",
		"AS7713 PT Contoh Indonesia, ID",
		"402969 Example ARIN Holder",
		"badline",
		"0 Zero ASN",
		"",
	}, "\n")

	names, err := ParseASNNames(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if names[1] != "LVLT-1 - Level 3 Parent, LLC, US" {
		t.Fatalf("unexpected name for AS1: %q", names[1])
	}
	if names[7713] != "PT Contoh Indonesia, ID" {
		t.Fatalf("unexpected name for AS7713: %q", names[7713])
	}
	if names[402969] != "Example ARIN Holder" {
		t.Fatalf("unexpected name for AS402969: %q", names[402969])
	}
	if _, ok := names[0]; ok {
		t.Fatal("AS0 should be skipped")
	}
}

func TestParseASNNameCSV(t *testing.T) {
	input := "asn,name,class,cc\n" +
		"AS7713,\"PT Telkom Indonesia Tbk, ID\",Carrier,ID\n" +
		"AS139205,PT Jala Informatika Nusantara,Unknown,ID\n" +
		"AS0,Bad ASN,Unknown,ID\n"
	names, err := ParseASNNameCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if names[7713] != "PT Telkom Indonesia Tbk, ID" {
		t.Fatalf("unexpected CSV name: %q", names[7713])
	}
	if names[139205] != "PT Jala Informatika Nusantara" {
		t.Fatalf("unexpected CSV name: %q", names[139205])
	}
	if _, ok := names[0]; ok {
		t.Fatal("AS0 should be skipped")
	}
}

func TestParseIPToASNNames(t *testing.T) {
	input := "16777216\t16777471\t13335\tUS\tCLOUDFLARENET\n" +
		"16777472\t16778239\t0\tNone\tNot routed\n" +
		"17432576\t17434111\t7713\tID\tTELKOMNET-AS-AP PT Telekomunikasi Indonesia\n"
	names, err := ParseIPToASNNames(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse TSV: %v", err)
	}
	if names[13335] != "CLOUDFLARENET" {
		t.Fatalf("unexpected TSV name: %q", names[13335])
	}
	if names[7713] != "TELKOMNET-AS-AP PT Telekomunikasi Indonesia" {
		t.Fatalf("unexpected TSV name: %q", names[7713])
	}
	if _, ok := names[0]; ok {
		t.Fatal("AS0 should be skipped")
	}
}

func TestImportEnrichesASNNames(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/id", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "apnic|ID|asn|7713|1|19991022|allocated\n")
		fmt.Fprint(w, "arin|ID|asn|402969|1|20260831|assigned\n")
	})
	mux.HandleFunc("/names", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "7713 PT Contoh Indonesia, ID\n")
		fmt.Fprint(w, "402969 Example ARIN Holder, ID\n")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := newTestService(t)
	svc.SetIDASNSourceURLs([]string{srv.URL + "/id"})
	svc.SetASNNameSourceURL(srv.URL + "/names")

	created, total, err := svc.ImportIndonesianASNs(context.Background())
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if created != 2 || total != 2 {
		t.Fatalf("unexpected import counts: created=%d total=%d", created, total)
	}
	for asn, want := range map[uint32]string{
		7713:   "PT Contoh Indonesia, ID",
		402969: "Example ARIN Holder, ID",
	} {
		entry, ok := svc.store.GetWhitelistASN(asn)
		if !ok {
			t.Fatalf("AS%d missing from whitelist", asn)
		}
		if entry.Name != want {
			t.Fatalf("AS%d name = %q, want %q", asn, entry.Name, want)
		}
	}
}

func TestImportSucceedsWhenNameSourceFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/id", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "apnic|ID|asn|7713|1|19991022|allocated\n")
	})
	mux.HandleFunc("/names", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := newTestService(t)
	svc.SetIDASNSourceURLs([]string{srv.URL + "/id"})
	svc.SetASNNameSourceURL(srv.URL + "/names")

	created, _, err := svc.ImportIndonesianASNs(context.Background())
	if err != nil {
		t.Fatalf("import should succeed without names: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected one ASN, got %d", created)
	}
	entry, ok := svc.store.GetWhitelistASN(7713)
	if !ok || entry.Name != "" {
		t.Fatalf("expected empty name, got %+v", entry)
	}
}
