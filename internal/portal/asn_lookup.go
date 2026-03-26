package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"time"
)

// ASNInfo represents ASN data from ipinfo.io lite API
type ASNInfo struct {
	IP            string `json:"ip"`
	ASN           string `json:"asn"`
	ASName        string `json:"as_name"`
	ASDomain      string `json:"as_domain"`
	Country       string `json:"country"`
	CountryCode   string `json:"country_code"`
	Continent     string `json:"continent"`
	ContinentCode string `json:"continent_code"`
}

// ASNLookup provides IP to ASN lookup functionality
type ASNLookup struct {
	token  string
	client *http.Client
}

// NewASNLookup creates a new ASN lookup service
// token is optional - if empty, requests will be made without authentication
// (ipinfo.io lite has rate limits for unauthenticated requests)
func NewASNLookup(token string) *ASNLookup {
	return &ASNLookup{
		token: token,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// LookupIP queries ipinfo.io lite API for ASN information of the given IP
func (a *ASNLookup) LookupIP(ctx context.Context, ip string) (*ASNInfo, error) {
	// Validate IP address
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, fmt.Errorf("invalid IP address: %w", err)
	}

	// Build URL - ipinfo.io lite endpoint
	url := fmt.Sprintf("https://api.ipinfo.io/lite/%s", addr.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add token if available
	if a.token != "" {
		// ipinfo.io supports token as query parameter or Basic Auth
		// Using query parameter for simplicity
		q := req.URL.Query()
		q.Add("token", a.token)
		req.URL.RawQuery = q.Encode()
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query ipinfo.io: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ipinfo.io returned status %d", resp.StatusCode)
	}

	var info ASNInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}

// LookupCIDR performs ASN lookup for a CIDR range
// For CIDR, we lookup the network address
func (a *ASNLookup) LookupCIDR(ctx context.Context, cidr string) (*ASNInfo, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	// Lookup the network address
	return a.LookupIP(ctx, prefix.Addr().String())
}

// Lookup performs ASN lookup for either IP or CIDR
func (a *ASNLookup) Lookup(ctx context.Context, input string) (*ASNInfo, error) {
	// Try as IP first
	if info, err := a.LookupIP(ctx, input); err == nil {
		return info, nil
	}

	// Try as CIDR
	return a.LookupCIDR(ctx, input)
}

// IsEnabled returns true if ASN lookup is enabled (token is configured)
func (a *ASNLookup) IsEnabled() bool {
	return a.token != ""
}
