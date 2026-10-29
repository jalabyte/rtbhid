package portal

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ASN holder names are not present in RIR delegation files (which only have
// ASN/country/status). Fetch a bulk name dataset instead.
const (
	DefaultASNNameSourceURL = "https://bgp.tools/asns.csv"
	DefaultASNNameTSVURL    = "https://iptoasn.com/data/ip2asn-v4-u32.tsv.gz"
	DefaultRIPEASNNameURL   = "https://ftp.ripe.net/ripe/asnames/asn.txt"
	DefaultASNNameUserAgent = "rtbhid-portal/1.0 (contact: support@jalabyte.com)"
)

// DefaultASNNameSourceURLs is tried in order; the first source that yields
// names wins. bgp.tools is complete and supports a descriptive user agent;
// iptoasn and RIPE act as fallbacks.
var DefaultASNNameSourceURLs = []string{
	DefaultASNNameSourceURL,
	DefaultASNNameTSVURL,
	DefaultRIPEASNNameURL,
}

// ASNNameRegistry fetches ASN holder names from bulk datasets.
type ASNNameRegistry struct {
	urls      []string
	userAgent string
	client    *http.Client
	log       *log.Logger
}

// NewASNNameRegistry creates a name registry. An empty URL uses the default
// fallback list.
func NewASNNameRegistry(url string) *ASNNameRegistry {
	if strings.TrimSpace(url) == "" {
		return NewASNNameRegistryURLs(nil)
	}
	return NewASNNameRegistryURLs([]string{url})
}

// NewASNNameRegistryURLs creates a name registry. Empty input uses the default
// fallback list.
func NewASNNameRegistryURLs(urls []string) *ASNNameRegistry {
	clean := make([]string, 0, len(urls))
	for _, u := range urls {
		if u = strings.TrimSpace(u); u != "" {
			clean = append(clean, u)
		}
	}
	if len(clean) == 0 {
		clean = append(clean, DefaultASNNameSourceURLs...)
	}
	return &ASNNameRegistry{
		urls:      clean,
		userAgent: DefaultASNNameUserAgent,
		client: &http.Client{
			Timeout: 180 * time.Second,
		},
		log: log.Default(),
	}
}

// Fetch tries each source until one returns names.
func (r *ASNNameRegistry) Fetch(ctx context.Context) (map[uint32]string, error) {
	var failures []string
	for _, url := range r.urls {
		names, err := r.fetchOne(ctx, url)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", url, err))
			r.log.Printf("asn names: source %s failed: %v", url, err)
			continue
		}
		if len(names) == 0 {
			failures = append(failures, url+": no names")
			continue
		}
		return names, nil
	}
	if len(failures) == 0 {
		return nil, fmt.Errorf("no ASN name sources configured")
	}
	return nil, fmt.Errorf("all ASN name sources failed: %s", strings.Join(failures, "; "))
}

func (r *ASNNameRegistry) fetchOne(ctx context.Context, url string) (map[uint32]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", r.userAgent)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body := io.Reader(io.LimitReader(resp.Body, 128<<20))
	if strings.HasSuffix(strings.ToLower(url), ".gz") {
		gz, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		return ParseIPToASNNames(gz)
	}
	if strings.HasSuffix(strings.ToLower(url), ".csv") || strings.Contains(url, "bgp.tools") {
		return ParseASNNameCSV(body)
	}
	return ParseASNNames(body)
}

// ParseASNNameCSV parses bgp.tools-style CSV with asn,name,class,cc columns.
func ParseASNNameCSV(r io.Reader) (map[uint32]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	asnIdx, nameIdx := -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "asn":
			asnIdx = i
		case "name":
			nameIdx = i
		}
	}
	if asnIdx < 0 || nameIdx < 0 {
		return nil, fmt.Errorf("unexpected ASN CSV header")
	}

	out := make(map[uint32]string)
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV: %w", err)
		}
		if asnIdx >= len(rec) || nameIdx >= len(rec) {
			continue
		}
		asnField := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(rec[asnIdx])), "AS")
		asn, err := strconv.ParseUint(asnField, 10, 32)
		if err != nil || asn == 0 {
			continue
		}
		name := strings.TrimSpace(rec[nameIdx])
		if name == "" {
			continue
		}
		if _, exists := out[uint32(asn)]; !exists {
			out[uint32(asn)] = name
		}
	}
	return out, nil
}

// ParseIPToASNNames parses iptoasn.com TSV rows:
// range_start, range_end, ASN, country, description.
func ParseIPToASNNames(r io.Reader) (map[uint32]string, error) {
	out := make(map[uint32]string)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		asn, err := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 32)
		if err != nil || asn == 0 {
			continue
		}
		name := strings.TrimSpace(fields[4])
		if name == "" || strings.EqualFold(name, "Not routed") || strings.EqualFold(fields[3], "None") {
			continue
		}
		if _, exists := out[uint32(asn)]; !exists {
			out[uint32(asn)] = name
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ASN names: %w", err)
	}
	return out, nil
}

// ParseASNNames parses lines of the form "AS1234 Name" or "1234 Name".
func ParseASNNames(r io.Reader) (map[uint32]string, error) {
	out := make(map[uint32]string)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		asnField := strings.TrimPrefix(strings.ToUpper(fields[0]), "AS")
		asn, err := strconv.ParseUint(asnField, 10, 32)
		if err != nil || asn == 0 {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		if name == "" {
			continue
		}
		out[uint32(asn)] = name
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ASN names: %w", err)
	}
	return out, nil
}
