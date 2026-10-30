package portal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	ErrInvalidASN      = errors.New("invalid asn")
	ErrNoAddressFamily = errors.New("at least one of ipv4/ipv6 must be true")
	ErrInvalidSource   = errors.New("invalid irr source")
	// A single bgpq4 run takes 4-10s against public IRR servers; spikes are common.
	defaultBGPQTimeout = 60 * time.Second
)

type bgpqPrefix struct {
	Prefix string `json:"prefix"`
}

func (s *Service) BuildPrefixesFromASN(ctx context.Context, asn string, ipv4, ipv6 bool) ([]string, error) {
	return s.BuildPrefixesFromASNWithSources(ctx, asn, ipv4, ipv6, nil)
}

func (s *Service) BuildPrefixesFromASNWithSources(ctx context.Context, asn string, ipv4, ipv6 bool, sources []string) ([]string, error) {
	if !ipv4 && !ipv6 {
		return nil, ErrNoAddressFamily
	}

	normalized, err := normalizeASN(asn)
	if err != nil {
		return nil, err
	}

	sourceArg, err := normalizeSources(sources)
	if err != nil {
		return nil, err
	}

	ctx, cancel := ensureTimeout(ctx, defaultBGPQTimeout)
	defer cancel()

	// IPv4 and IPv6 run in parallel so a slow IRR reply only costs one wait.
	families := []string{}
	if ipv4 {
		families = append(families, "-4")
	}
	if ipv6 {
		families = append(families, "-6")
	}
	results := make([][]string, len(families))
	errs := make([]error, len(families))
	var wg sync.WaitGroup
	for i, fam := range families {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = runBGPQWithSources(ctx, normalized, fam, sourceArg)
		}()
	}
	wg.Wait()

	all := make([]string, 0)
	for i := range families {
		if errs[i] != nil {
			return nil, errs[i]
		}
		all = append(all, results[i]...)
	}

	set := map[string]struct{}{}
	for _, raw := range all {
		pref, err := netip.ParsePrefix(raw)
		if err != nil {
			continue
		}
		set[pref.Masked().String()] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for cidr := range set {
		out = append(out, cidr)
	}
	sort.Strings(out)
	return out, nil
}

func runBGPQ(ctx context.Context, asn string, familyFlag string) ([]string, error) {
	return runBGPQWithSources(ctx, asn, familyFlag, "")
}

func runBGPQWithSources(ctx context.Context, asn string, familyFlag string, sources string) ([]string, error) {
	args := []string{"-j"}
	if familyFlag != "" {
		args = append(args, familyFlag)
	}
	if sources != "" {
		args = append(args, "-S", sources)
	}
	args = append(args, asn)

	cmd := exec.CommandContext(ctx, "bgpq4", args...)
	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("IRR query timed out; the IRR server is slow, try again or select fewer sources")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("bgpq4 failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var payload map[string][]bgpqPrefix
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parse bgpq4 output: %w", err)
	}

	prefixes := make([]string, 0)
	for _, entries := range payload {
		for _, entry := range entries {
			if entry.Prefix == "" {
				continue
			}
			prefixes = append(prefixes, entry.Prefix)
		}
	}
	return prefixes, nil
}

func normalizeASN(asn string) (string, error) {
	trimmed := strings.TrimSpace(asn)
	if trimmed == "" {
		return "", ErrInvalidASN
	}

	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "AS") {
		digits := strings.TrimPrefix(upper, "AS")
		if digits == "" || !isAllDigits(digits) {
			return "", ErrInvalidASN
		}
		return "AS" + digits, nil
	}

	if !isAllDigits(upper) {
		return "", ErrInvalidASN
	}
	return "AS" + upper, nil
}

func normalizeSources(sources []string) (string, error) {
	if len(sources) == 0 {
		return "", nil
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(sources))
	for _, src := range sources {
		trimmed := strings.TrimSpace(src)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)
		if !isValidIRRSource(upper) {
			return "", ErrInvalidSource
		}
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		out = append(out, upper)
	}

	if len(out) == 0 {
		return "", nil
	}
	sort.Strings(out)
	return strings.Join(out, ","), nil
}

func isValidIRRSource(value string) bool {
	for _, r := range value {
		if unicode.IsDigit(r) || unicode.IsLetter(r) || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func isAllDigits(value string) bool {
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func ensureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), timeout)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
