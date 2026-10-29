package portal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultIDASNSourceURL is the APNIC extended delegation file. It contains
// `asn` records with country codes, which lets RTBHID build the Indonesian
// ASN whitelist without a bundled dataset.
const DefaultIDASNSourceURL = "https://ftp.apnic.net/stats/apnic/delegated-apnic-extended-latest"

// IDASNRegistry downloads and parses the Indonesian ASN list published by
// APNIC.
type IDASNRegistry struct {
	url    string
	client *http.Client
}

// NewIDASNRegistry creates a registry. An empty url uses
// DefaultIDASNSourceURL.
func NewIDASNRegistry(url string) *IDASNRegistry {
	if strings.TrimSpace(url) == "" {
		url = DefaultIDASNSourceURL
	}
	return &IDASNRegistry{
		url: url,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Fetch downloads the registry and returns the Indonesian ASNs.
func (r *IDASNRegistry) Fetch(ctx context.Context) ([]uint32, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "rtbhid-portal/1.0")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", r.url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", r.url, resp.StatusCode)
	}
	return ParseAPNICIDASNs(io.LimitReader(resp.Body, 64<<20))
}

// ParseAPNICIDASNs extracts `asn` records with country code ID from an APNIC
// delegated statistics file.
func ParseAPNICIDASNs(r io.Reader) ([]uint32, error) {
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
		if !strings.EqualFold(fields[0], "apnic") || !strings.EqualFold(fields[1], "ID") || !strings.EqualFold(fields[2], "asn") {
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
	if len(out) == 0 {
		return nil, fmt.Errorf("no Indonesian ASN records found")
	}
	return out, nil
}
