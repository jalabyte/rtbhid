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
	"time"
	"unicode"
)

var (
	ErrInvalidASN      = errors.New("invalid asn")
	ErrNoAddressFamily = errors.New("at least one of ipv4/ipv6 must be true")
	ErrInvalidSource   = errors.New("invalid irr source")
	defaultBGPQTimeout = 30 * time.Second
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

	all := make([]string, 0)

	if ipv4 {
		v4, err := runBGPQWithSources(ctx, normalized, "-4", sourceArg)
		if err != nil {
			return nil, err
		}
		all = append(all, v4...)
	}

	if ipv6 {
		v6, err := runBGPQWithSources(ctx, normalized, "-6", sourceArg)
		if err != nil {
			return nil, err
		}
		all = append(all, v6...)
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
