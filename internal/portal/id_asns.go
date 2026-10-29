package portal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RIR delegation statistics. An Indonesian organisation can hold an ASN from
// any RIR, so the whitelist importer unions the country-ID ASN records from all
// five registries instead of relying on APNIC alone.
const (
	DefaultAPNICASNSourceURL   = "https://ftp.apnic.net/stats/apnic/delegated-apnic-extended-latest"
	DefaultARINASNSourceURL    = "https://ftp.arin.net/pub/stats/arin/delegated-arin-extended-latest"
	DefaultRIPEASNSourceURL    = "https://ftp.ripe.net/pub/stats/ripencc/delegated-ripencc-extended-latest"
	DefaultLACNICASNSourceURL  = "https://ftp.lacnic.net/pub/stats/lacnic/delegated-lacnic-extended-latest"
	DefaultAFRINICASNSourceURL = "https://ftp.afrinic.net/pub/stats/afrinic/delegated-afrinic-extended-latest"

	// DefaultIDASNSourceURL is kept for compatibility; it is the APNIC file.
	DefaultIDASNSourceURL = DefaultAPNICASNSourceURL
)

// DefaultIDASNSourceURLs lists all RIR extended delegation files.
var DefaultIDASNSourceURLs = []string{
	DefaultAPNICASNSourceURL,
	DefaultARINASNSourceURL,
	DefaultRIPEASNSourceURL,
	DefaultLACNICASNSourceURL,
	DefaultAFRINICASNSourceURL,
}

// IDASNRegistry downloads and parses Indonesian ASN records published by the
// RIRs.
type IDASNRegistry struct {
	urls   []string
	client *http.Client
	log    *log.Logger

	mu       sync.Mutex
	warnings []string
}

// NewIDASNRegistry creates a registry from a single URL. An empty url uses all
// default RIR sources.
func NewIDASNRegistry(url string) *IDASNRegistry {
	if strings.TrimSpace(url) == "" {
		return NewIDASNRegistryURLs(nil)
	}
	return NewIDASNRegistryURLs([]string{url})
}

// NewIDASNRegistryURLs creates a registry from the supplied URLs. Empty input
// uses all default RIR sources.
func NewIDASNRegistryURLs(urls []string) *IDASNRegistry {
	clean := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" {
			clean = append(clean, u)
		}
	}
	if len(clean) == 0 {
		clean = append(clean, DefaultIDASNSourceURLs...)
	}
	return &IDASNRegistry{
		urls: clean,
		client: &http.Client{
			// RIR delegation files can be large and slow; sources are fetched
			// concurrently so this is the per-source limit.
			Timeout: 300 * time.Second,
		},
		log: log.Default(),
	}
}

// Fetch downloads every configured registry and returns the union of Indonesian
// ASNs. A failing source is logged and skipped; the call only fails when no
// source yields any ASN.
func (r *IDASNRegistry) Fetch(ctx context.Context) ([]uint32, error) {
	type sourceResult struct {
		url  string
		asns []uint32
		err  error
	}

	results := make(chan sourceResult, len(r.urls))
	var wg sync.WaitGroup
	for _, url := range r.urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			asns, err := r.fetchOne(ctx, u)
			results <- sourceResult{url: u, asns: asns, err: err}
		}(url)
	}
	wg.Wait()
	close(results)

	seen := make(map[uint32]struct{})
	var out []uint32
	var failures []string
	for res := range results {
		if res.err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", res.url, res.err))
			r.log.Printf("id-asn: source %s failed: %v", res.url, res.err)
			continue
		}
		for _, asn := range res.asns {
			if _, ok := seen[asn]; ok {
				continue
			}
			seen[asn] = struct{}{}
			out = append(out, asn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	r.mu.Lock()
	r.warnings = failures
	r.mu.Unlock()

	if len(out) == 0 {
		if len(failures) > 0 {
			return nil, fmt.Errorf("all Indonesian ASN sources failed: %s", strings.Join(failures, "; "))
		}
		return nil, fmt.Errorf("no Indonesian ASN records found")
	}
	return out, nil
}

// Warnings returns non-fatal source failures from the last Fetch.
func (r *IDASNRegistry) Warnings() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.warnings...)
}

func (r *IDASNRegistry) fetchOne(ctx context.Context, url string) ([]uint32, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "rtbhid-portal/1.0")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return ParseIDASNs(io.LimitReader(resp.Body, 64<<20))
}

// ParseAPNICIDASNs extracts `asn` records with country code ID from an APNIC
// delegated statistics file. It is kept for callers that only have APNIC data.
func ParseAPNICIDASNs(r io.Reader) ([]uint32, error) {
	return ParseIDASNs(r)
}

// ParseIDASNs extracts `asn` records with country code ID from an RIR
// delegated statistics file. The registry column is ignored so the parser works
// with APNIC, ARIN, RIPE, LACNIC, and AFRINIC files.
func ParseIDASNs(r io.Reader) ([]uint32, error) {
	seen := make(map[uint32]struct{})
	var out []uint32
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[1], "ID") || !strings.EqualFold(fields[2], "asn") {
			continue
		}
		start, err := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 32)
		if err != nil {
			continue
		}
		count := uint64(1)
		if v, err := strconv.ParseUint(strings.TrimSpace(fields[4]), 10, 32); err == nil && v > 0 && v < 1024 {
			count = v
		}
		for i := uint64(0); i < count; i++ {
			asn := uint32(start + i)
			if _, ok := seen[asn]; ok {
				continue
			}
			seen[asn] = struct{}{}
			out = append(out, asn)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan registry: %w", err)
	}
	return out, nil
}
