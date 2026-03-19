package bird

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type StandardCommunity struct {
	High uint16
	Low  uint16
}

type LargeCommunity struct {
	Global uint32
	Local1 uint32
	Local2 uint32
}

type ExportPolicy struct {
	Standard []StandardCommunity
	Large    []LargeCommunity
}

func ParseExportPolicy(stdRaw, largeRaw string) (ExportPolicy, error) {
	std, err := parseStandardCommunities(stdRaw)
	if err != nil {
		return ExportPolicy{}, err
	}
	large, err := parseLargeCommunities(largeRaw)
	if err != nil {
		return ExportPolicy{}, err
	}
	return ExportPolicy{Standard: std, Large: large}, nil
}

func parseStandardCommunities(raw string) ([]StandardCommunity, error) {
	tokens := splitList(raw)
	out := make([]StandardCommunity, 0, len(tokens))
	for _, tok := range tokens {
		parts := strings.Split(tok, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid std community %q (expected A:B)", tok)
		}
		a, err := parseUintBound(parts[0], 16)
		if err != nil {
			return nil, fmt.Errorf("invalid std community %q: %w", tok, err)
		}
		b, err := parseUintBound(parts[1], 16)
		if err != nil {
			return nil, fmt.Errorf("invalid std community %q: %w", tok, err)
		}
		out = append(out, StandardCommunity{High: uint16(a), Low: uint16(b)})
	}
	return out, nil
}

func parseLargeCommunities(raw string) ([]LargeCommunity, error) {
	tokens := splitList(raw)
	out := make([]LargeCommunity, 0, len(tokens))
	for _, tok := range tokens {
		parts := strings.Split(tok, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid large community %q (expected A:B:C)", tok)
		}
		a, err := parseUintBound(parts[0], 32)
		if err != nil {
			return nil, fmt.Errorf("invalid large community %q: %w", tok, err)
		}
		b, err := parseUintBound(parts[1], 32)
		if err != nil {
			return nil, fmt.Errorf("invalid large community %q: %w", tok, err)
		}
		c, err := parseUintBound(parts[2], 32)
		if err != nil {
			return nil, fmt.Errorf("invalid large community %q: %w", tok, err)
		}
		out = append(out, LargeCommunity{Global: uint32(a), Local1: uint32(b), Local2: uint32(c)})
	}
	return out, nil
}

func splitList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func parseUintBound(raw string, bits int) (uint64, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(raw), 10, bits)
	if err != nil {
		return 0, err
	}
	return v, nil
}

type Peer struct {
	ID          string
	Name        string
	Neighbor    string
	NeighborASN int
	Enabled     bool
	Family      string
	Password    string
	Multihop    int
}

type ApplyResult struct {
	RouteRenderedPath string `json:"route_rendered_path"`
	PeerRenderedPath  string `json:"peer_rendered_path"`
	RouteCount        int    `json:"route_count"`
	PeerCount         int    `json:"peer_count"`
	ReloadRan         bool   `json:"reload_ran"`
	ReloadError       string `json:"reload_error,omitempty"`
}

type Manager struct {
	routeOutputPath string
	peerOutputPath  string
	reloadEnabled   bool
	command         []string
	exportPolicy    ExportPolicy
}

func NewManager(routeOutputPath, peerOutputPath string, reloadEnabled bool, command []string, exportPolicy ExportPolicy) *Manager {
	return &Manager{
		routeOutputPath: routeOutputPath,
		peerOutputPath:  peerOutputPath,
		reloadEnabled:   reloadEnabled,
		command:         command,
		exportPolicy:    exportPolicy,
	}
}

func (m *Manager) Apply(prefixes []string, peers []Peer, localASN int) (ApplyResult, error) {
	v4 := make([]string, 0)
	v6 := make([]string, 0)
	for _, p := range prefixes {
		pref, err := netip.ParsePrefix(p)
		if err != nil {
			continue
		}
		if pref.Addr().Is4() {
			v4 = append(v4, pref.String())
		} else {
			v6 = append(v6, pref.String())
		}
	}
	sort.Strings(v4)
	sort.Strings(v6)

	routeRendered := renderRoutes(v4, v6)
	if err := writeAtomically(m.routeOutputPath, routeRendered); err != nil {
		return ApplyResult{}, fmt.Errorf("write routes bird config: %w", err)
	}

	enabledPeers := make([]Peer, 0, len(peers))
	for _, peer := range peers {
		if !peer.Enabled {
			continue
		}
		enabledPeers = append(enabledPeers, peer)
	}
	sort.Slice(enabledPeers, func(i, j int) bool {
		if enabledPeers[i].Name == enabledPeers[j].Name {
			return enabledPeers[i].Neighbor < enabledPeers[j].Neighbor
		}
		return enabledPeers[i].Name < enabledPeers[j].Name
	})
	peerRendered := renderPeers(enabledPeers, localASN, m.exportPolicy)
	if err := writeAtomically(m.peerOutputPath, peerRendered); err != nil {
		return ApplyResult{}, fmt.Errorf("write peers bird config: %w", err)
	}

	res := ApplyResult{
		RouteRenderedPath: m.routeOutputPath,
		PeerRenderedPath:  m.peerOutputPath,
		RouteCount:        len(v4) + len(v6),
		PeerCount:         len(enabledPeers),
	}
	if !m.reloadEnabled {
		return res, nil
	}
	res.ReloadRan = true
	if err := m.reload(); err != nil {
		res.ReloadError = err.Error()
	}
	return res, nil
}

func writeAtomically(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir output dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func (m *Manager) reload() error {
	if len(m.command) == 0 {
		return fmt.Errorf("bird reload command not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.command[0], m.command[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		text := strings.TrimSpace(out.String())
		if text == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, text)
	}
	return nil
}

func renderRoutes(v4, v6 []string) []byte {
	var b strings.Builder
	b.WriteString("# generated by rtbhd - do not edit manually\n")
	b.WriteString("# include this file from main bird.conf\n\n")

	b.WriteString("protocol static rtbh_auto_v4 {\n")
	b.WriteString("    ipv4;\n")
	if len(v4) == 0 {
		b.WriteString("    # no IPv4 routes\n")
	}
	for _, p := range v4 {
		b.WriteString("    route ")
		b.WriteString(p)
		b.WriteString(" blackhole;\n")
	}
	b.WriteString("}\n\n")

	b.WriteString("protocol static rtbh_auto_v6 {\n")
	b.WriteString("    ipv6;\n")
	if len(v6) == 0 {
		b.WriteString("    # no IPv6 routes\n")
	}
	for _, p := range v6 {
		b.WriteString("    route ")
		b.WriteString(p)
		b.WriteString(" blackhole;\n")
	}
	b.WriteString("}\n")

	return []byte(b.String())
}

var identRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func renderPeers(peers []Peer, localASN int, policy ExportPolicy) []byte {
	var b strings.Builder
	b.WriteString("# generated by rtbhd - do not edit manually\n")
	b.WriteString("# include this file from main bird.conf\n\n")

	if localASN <= 0 {
		localASN = 65500
	}

	b.WriteString("filter rtbh_export_v4 {\n")
	b.WriteString("    if proto = \"rtbh_auto_v4\" then {\n")
	for _, c := range policy.Standard {
		b.WriteString(fmt.Sprintf("        bgp_community.add((%d, %d));\n", c.High, c.Low))
	}
	for _, c := range policy.Large {
		b.WriteString(fmt.Sprintf("        bgp_large_community.add((%d, %d, %d));\n", c.Global, c.Local1, c.Local2))
	}
	b.WriteString("        accept;\n")
	b.WriteString("    }\n")
	b.WriteString("    reject;\n")
	b.WriteString("}\n\n")

	b.WriteString("filter rtbh_export_v6 {\n")
	b.WriteString("    if proto = \"rtbh_auto_v6\" then {\n")
	for _, c := range policy.Standard {
		b.WriteString(fmt.Sprintf("        bgp_community.add((%d, %d));\n", c.High, c.Low))
	}
	for _, c := range policy.Large {
		b.WriteString(fmt.Sprintf("        bgp_large_community.add((%d, %d, %d));\n", c.Global, c.Local1, c.Local2))
	}
	b.WriteString("        accept;\n")
	b.WriteString("    }\n")
	b.WriteString("    reject;\n")
	b.WriteString("}\n\n")

	if len(peers) == 0 {
		b.WriteString("# no active BGP peers\n")
		return []byte(b.String())
	}

	for i, peer := range peers {
		neighbor := strings.TrimSpace(peer.Neighbor)
		if neighbor == "" {
			continue
		}
		protocolName := peer.ID
		if protocolName == "" {
			protocolName = fmt.Sprintf("peer_%d", i+1)
		}
		protocolName = identRe.ReplaceAllString(protocolName, "_")
		if protocolName == "" {
			protocolName = fmt.Sprintf("peer_%d", i+1)
		}
		if peer.Multihop <= 0 {
			peer.Multihop = 1
		}
		family := strings.ToLower(strings.TrimSpace(peer.Family))
		if family != "ipv6" {
			family = "ipv4"
		}

		b.WriteString("protocol bgp rtbh_")
		b.WriteString(protocolName)
		b.WriteString(" {\n")
		b.WriteString("    local as ")
		b.WriteString(fmt.Sprintf("%d", localASN))
		b.WriteString(";\n")
		b.WriteString("    neighbor ")
		b.WriteString(neighbor)
		b.WriteString(" as ")
		b.WriteString(fmt.Sprintf("%d", peer.NeighborASN))
		b.WriteString(";\n")
		if peer.Multihop > 1 {
			b.WriteString("    multihop ")
			b.WriteString(fmt.Sprintf("%d", peer.Multihop))
			b.WriteString(";\n")
		}
		if strings.TrimSpace(peer.Password) != "" {
			b.WriteString("    password \"")
			b.WriteString(escapeBirdString(peer.Password))
			b.WriteString("\";\n")
		}
		if family == "ipv4" {
			b.WriteString("    ipv4 {\n")
			b.WriteString("        import none;\n")
			b.WriteString("        export filter rtbh_export_v4;\n")
			b.WriteString("    };\n")
		} else {
			b.WriteString("    ipv6 {\n")
			b.WriteString("        import none;\n")
			b.WriteString("        export filter rtbh_export_v6;\n")
			b.WriteString("    };\n")
		}
		b.WriteString("}\n\n")
	}

	return []byte(b.String())
}

func escapeBirdString(in string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return replacer.Replace(in)
}
