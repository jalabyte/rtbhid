package syncer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"rtbhid/internal/state"
)

type Fetcher struct {
	client    *http.Client
	userAgent string
}

func NewFetcher(userAgent string) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		userAgent: userAgent,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, sourceURL string, timeoutSec int) ([]string, error) {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid source url: %w", err)
	}

	if u.Scheme == "file" {
		file, err := os.Open(u.Path)
		if err != nil {
			return nil, fmt.Errorf("open file source: %w", err)
		}
		defer file.Close()
		return parseList(file)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}

	timeout := 30 * time.Second
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("source returned status %d", resp.StatusCode)
	}
	return parseList(resp.Body)
}

func parseList(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	set := map[string]any{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		for _, token := range strings.FieldsFunc(line, func(r rune) bool {
			return r == ',' || r == '\t' || r == ' '
		}) {
			if token == "" {
				continue
			}
			norm, err := state.NormalizePrefix(token)
			if err != nil {
				continue
			}
			set[norm] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan source list: %w", err)
	}

	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
