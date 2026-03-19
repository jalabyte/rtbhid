package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite single-writer

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	for _, sql := range migrations {
		if _, err := s.db.Exec(sql); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// --------------------------------------------------------------------------
// Agent CRUD
// --------------------------------------------------------------------------

func (s *Store) RegisterAgent(a Agent) error {
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO agents (id, name, hostname, bird_version, listen_addr, status, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Hostname, a.BirdVersion, a.ListenAddr, a.Status, a.LastSeen, a.CreatedAt, a.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateAgentHeartbeat(id string, status string, prefixCount int, peerCount int) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		UPDATE agents SET status = ?, last_seen = ?, updated_at = ? WHERE id = ?`,
		status, now, now, id,
	)
	return err
}

func (s *Store) ListAgents() []Agent {
	rows, err := s.db.Query(`
		SELECT id, name, hostname, bird_version, listen_addr, status, last_seen, created_at, updated_at
		FROM agents ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Hostname, &a.BirdVersion, &a.ListenAddr, &a.Status, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out
}

func (s *Store) GetAgent(id string) (Agent, bool) {
	var a Agent
	err := s.db.QueryRow(`
		SELECT id, name, hostname, bird_version, listen_addr, status, last_seen, created_at, updated_at
		FROM agents WHERE id = ?`, id).
		Scan(&a.ID, &a.Name, &a.Hostname, &a.BirdVersion, &a.ListenAddr, &a.Status, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return Agent{}, false
	}
	return a, true
}

func (s *Store) DeleteAgent(id string) error {
	_, err := s.db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	return err
}

func (s *Store) MarkStaleAgents(timeout time.Duration) int {
	deadline := time.Now().UTC().Add(-timeout)
	result, err := s.db.Exec(`
		UPDATE agents SET status = 'offline', updated_at = ?
		WHERE status != 'offline' AND (last_seen IS NULL OR last_seen < ?)`,
		deadline, deadline,
	)
	if err != nil {
		return 0
	}
	n, _ := result.RowsAffected()
	return int(n)
}

// --------------------------------------------------------------------------
// Prefix CRUD
// --------------------------------------------------------------------------

func (s *Store) CreatePrefix(p Prefix) (Prefix, error) {
	normed, err := normalizeCIDR(p.CIDR)
	if err != nil {
		return Prefix{}, err
	}
	p.CIDR = normed
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`
		INSERT INTO prefixes (cidr, enabled, note, created_at) VALUES (?, ?, ?, ?)`,
		p.CIDR, p.Enabled, p.Note, p.CreatedAt,
	)
	if err != nil {
		return Prefix{}, err
	}
	p.ID, _ = res.LastInsertId()
	return p, nil
}

func (s *Store) ListPrefixes() []Prefix {
	rows, err := s.db.Query(`SELECT id, cidr, enabled, note, created_at FROM prefixes ORDER BY cidr`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Prefix
	for rows.Next() {
		var p Prefix
		if err := rows.Scan(&p.ID, &p.CIDR, &p.Enabled, &p.Note, &p.CreatedAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (s *Store) DeletePrefix(id int64) error {
	_, err := s.db.Exec(`DELETE FROM prefixes WHERE id = ?`, id)
	return err
}

func (s *Store) TogglePrefix(id int64, enabled bool) error {
	_, err := s.db.Exec(`UPDATE prefixes SET enabled = ? WHERE id = ?`, enabled, id)
	return err
}

// --------------------------------------------------------------------------
// Source CRUD
// --------------------------------------------------------------------------

func (s *Store) CreateSource(src Source) error {
	if src.CreatedAt.IsZero() {
		src.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		INSERT INTO sources (id, name, url, interval, enabled, last_sync, last_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		src.ID, src.Name, src.URL, src.Interval, src.Enabled, src.LastSync, src.LastCount, src.CreatedAt,
	)
	return err
}

func (s *Store) ListSources() []Source {
	rows, err := s.db.Query(`SELECT id, name, url, interval, enabled, last_sync, last_count, created_at FROM sources ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.ID, &src.Name, &src.URL, &src.Interval, &src.Enabled, &src.LastSync, &src.LastCount, &src.CreatedAt); err != nil {
			continue
		}
		out = append(out, src)
	}
	return out
}

func (s *Store) GetSource(id string) (Source, bool) {
	var src Source
	err := s.db.QueryRow(`SELECT id, name, url, interval, enabled, last_sync, last_count, created_at FROM sources WHERE id = ?`, id).
		Scan(&src.ID, &src.Name, &src.URL, &src.Interval, &src.Enabled, &src.LastSync, &src.LastCount, &src.CreatedAt)
	if err != nil {
		return Source{}, false
	}
	return src, true
}

func (s *Store) UpdateSource(src Source) error {
	_, err := s.db.Exec(`
		UPDATE sources SET name = ?, url = ?, interval = ?, enabled = ? WHERE id = ?`,
		src.Name, src.URL, src.Interval, src.Enabled, src.ID,
	)
	return err
}

func (s *Store) DeleteSource(id string) error {
	_, err := s.db.Exec(`DELETE FROM sources WHERE id = ?`, id)
	return err
}

func (s *Store) ReplaceSourcePrefixes(sourceID string, cidrs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM source_prefixes WHERE source_id = ?`, sourceID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO source_prefixes (source_id, cidr) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cidr := range cidrs {
		normed, err := normalizeCIDR(cidr)
		if err != nil {
			continue // skip invalid CIDRs silently
		}
		if _, err := stmt.Exec(sourceID, normed); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetSourceSyncResult(id string, count int) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE sources SET last_sync = ?, last_count = ? WHERE id = ?`, now, count, id)
	return err
}

// --------------------------------------------------------------------------
// Exclude CRUD
// --------------------------------------------------------------------------

func (s *Store) CreateExclude(ex Exclude) (Exclude, error) {
	normed, err := normalizeCIDR(ex.CIDR)
	if err != nil {
		return Exclude{}, err
	}
	ex.CIDR = normed
	if ex.CreatedAt.IsZero() {
		ex.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`
		INSERT INTO excludes (cidr, note, created_at) VALUES (?, ?, ?)`,
		ex.CIDR, ex.Note, ex.CreatedAt,
	)
	if err != nil {
		return Exclude{}, err
	}
	ex.ID, _ = res.LastInsertId()
	return ex, nil
}

func (s *Store) ListExcludes() []Exclude {
	rows, err := s.db.Query(`SELECT id, cidr, note, created_at FROM excludes ORDER BY cidr`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Exclude
	for rows.Next() {
		var ex Exclude
		if err := rows.Scan(&ex.ID, &ex.CIDR, &ex.Note, &ex.CreatedAt); err != nil {
			continue
		}
		out = append(out, ex)
	}
	return out
}

func (s *Store) DeleteExclude(id int64) error {
	_, err := s.db.Exec(`DELETE FROM excludes WHERE id = ?`, id)
	return err
}

// --------------------------------------------------------------------------
// ExcludeSource CRUD
// --------------------------------------------------------------------------

func (s *Store) CreateExcludeSource(src Source) error {
	if src.CreatedAt.IsZero() {
		src.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		INSERT INTO exclude_sources (id, name, url, interval, enabled, last_sync, last_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		src.ID, src.Name, src.URL, src.Interval, src.Enabled, src.LastSync, src.LastCount, src.CreatedAt,
	)
	return err
}

func (s *Store) ListExcludeSources() []Source {
	rows, err := s.db.Query(`SELECT id, name, url, interval, enabled, last_sync, last_count, created_at FROM exclude_sources ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.ID, &src.Name, &src.URL, &src.Interval, &src.Enabled, &src.LastSync, &src.LastCount, &src.CreatedAt); err != nil {
			continue
		}
		out = append(out, src)
	}
	return out
}

func (s *Store) GetExcludeSource(id string) (Source, bool) {
	var src Source
	err := s.db.QueryRow(`SELECT id, name, url, interval, enabled, last_sync, last_count, created_at FROM exclude_sources WHERE id = ?`, id).
		Scan(&src.ID, &src.Name, &src.URL, &src.Interval, &src.Enabled, &src.LastSync, &src.LastCount, &src.CreatedAt)
	if err != nil {
		return Source{}, false
	}
	return src, true
}

func (s *Store) DeleteExcludeSource(id string) error {
	_, err := s.db.Exec(`DELETE FROM exclude_sources WHERE id = ?`, id)
	return err
}

func (s *Store) ReplaceExcludeSourcePrefixes(sourceID string, cidrs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM exclude_source_prefixes WHERE source_id = ?`, sourceID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO exclude_source_prefixes (source_id, cidr) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cidr := range cidrs {
		normed, err := normalizeCIDR(cidr)
		if err != nil {
			continue
		}
		if _, err := stmt.Exec(sourceID, normed); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetExcludeSourceSyncResult(id string, count int) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE exclude_sources SET last_sync = ?, last_count = ? WHERE id = ?`, now, count, id)
	return err
}

// --------------------------------------------------------------------------
// Peer CRUD
// --------------------------------------------------------------------------

func (s *Store) CreatePeer(p Peer) error {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	stdJSON, _ := json.Marshal(p.StdCommunities)
	largeJSON, _ := json.Marshal(p.LargeCommunities)
	_, err := s.db.Exec(`
		INSERT INTO peers (id, name, neighbor_ip, remote_as, local_as, multihop, enabled, std_communities, large_communities, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.NeighborIP, p.RemoteAS, p.LocalAS, p.Multihop, p.Enabled, string(stdJSON), string(largeJSON), p.CreatedAt,
	)
	return err
}

func (s *Store) ListPeers() []Peer {
	rows, err := s.db.Query(`
		SELECT id, name, neighbor_ip, remote_as, local_as, multihop, enabled, std_communities, large_communities, created_at
		FROM peers ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Peer
	for rows.Next() {
		var p Peer
		var stdJSON, largeJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.NeighborIP, &p.RemoteAS, &p.LocalAS, &p.Multihop, &p.Enabled, &stdJSON, &largeJSON, &p.CreatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(stdJSON), &p.StdCommunities)
		_ = json.Unmarshal([]byte(largeJSON), &p.LargeCommunities)
		out = append(out, p)
	}
	return out
}

func (s *Store) UpdatePeer(p Peer) error {
	stdJSON, _ := json.Marshal(p.StdCommunities)
	largeJSON, _ := json.Marshal(p.LargeCommunities)
	_, err := s.db.Exec(`
		UPDATE peers SET name = ?, neighbor_ip = ?, remote_as = ?, local_as = ?, multihop = ?, enabled = ?, std_communities = ?, large_communities = ?
		WHERE id = ?`,
		p.Name, p.NeighborIP, p.RemoteAS, p.LocalAS, p.Multihop, p.Enabled, string(stdJSON), string(largeJSON), p.ID,
	)
	return err
}

func (s *Store) DeletePeer(id string) error {
	_, err := s.db.Exec(`DELETE FROM peers WHERE id = ?`, id)
	return err
}

// --------------------------------------------------------------------------
// Event CRUD
// --------------------------------------------------------------------------

func (s *Store) AddEvent(eventType, message, agentID string) error {
	_, err := s.db.Exec(`
		INSERT INTO events (type, message, agent_id, created_at) VALUES (?, ?, ?, ?)`,
		eventType, message, agentID, time.Now().UTC(),
	)
	return err
}

func (s *Store) ListEvents(limit int) []Event {
	if limit <= 0 {
		limit = 300
	}
	rows, err := s.db.Query(`SELECT id, type, message, agent_id, created_at FROM events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.ID, &ev.Type, &ev.Message, &ev.AgentID, &ev.CreatedAt); err != nil {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// --------------------------------------------------------------------------
// EffectivePrefixes - core aggregation logic
// --------------------------------------------------------------------------

func (s *Store) EffectivePrefixes() []string {
	// 1. Collect enabled manual prefixes
	manualSet := map[string]bool{}
	prefixes := s.ListPrefixes()
	for _, p := range prefixes {
		if p.Enabled {
			manualSet[p.CIDR] = true
		}
	}

	// 2. Collect enabled source prefixes
	sourceSet := map[string]bool{}
	rows, err := s.db.Query(`
		SELECT sp.cidr FROM source_prefixes sp
		INNER JOIN sources s ON s.id = sp.source_id
		WHERE s.enabled = 1`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cidr string
			if err := rows.Scan(&cidr); err == nil {
				sourceSet[cidr] = true
			}
		}
	}

	// 3. Build exclusion set (CIDR-aware)
	excludes := s.ListExcludes()
	excludeExact := map[string]bool{}
	var excludeParsed []netip.Prefix
	for _, ex := range excludes {
		excludeExact[ex.CIDR] = true
		if pref, err := netip.ParsePrefix(ex.CIDR); err == nil {
			excludeParsed = append(excludeParsed, pref.Masked())
		}
	}

	// 3b. Add exclude source prefixes to exclusion set
	rows2, err2 := s.db.Query(`
		SELECT esp.cidr FROM exclude_source_prefixes esp
		INNER JOIN exclude_sources es ON es.id = esp.source_id
		WHERE es.enabled = 1`)
	if err2 == nil {
		defer rows2.Close()
		for rows2.Next() {
			var cidr string
			if err := rows2.Scan(&cidr); err == nil {
				excludeExact[cidr] = true
				if pref, err := netip.ParsePrefix(cidr); err == nil {
					excludeParsed = append(excludeParsed, pref.Masked())
				}
			}
		}
	}

	// 4. Merge: manual + source - excludes
	result := map[string]bool{}
	for cidr := range manualSet {
		result[cidr] = true
	}
	for cidr := range sourceSet {
		if excludeExact[cidr] {
			continue
		}
		if pref, err := netip.ParsePrefix(cidr); err == nil {
			if isExcludedByCIDR(pref, excludeParsed) {
				continue
			}
		}
		result[cidr] = true
	}

	// 5. Sort and return
	out := make([]string, 0, len(result))
	for cidr := range result {
		out = append(out, cidr)
	}
	sort.Strings(out)
	return out
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func normalizeCIDR(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("empty CIDR")
	}

	// Try as a prefix first
	if p, err := netip.ParsePrefix(trimmed); err == nil {
		return p.Masked().String(), nil
	}

	// Try as a bare IP address
	if addr, err := netip.ParseAddr(trimmed); err == nil {
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		return netip.PrefixFrom(addr, bits).Masked().String(), nil
	}

	return "", fmt.Errorf("invalid IP/CIDR: %s", trimmed)
}

func isExcludedByCIDR(target netip.Prefix, excluded []netip.Prefix) bool {
	target = target.Masked()
	for _, ex := range excluded {
		if ex.Addr().Is4() != target.Addr().Is4() {
			continue
		}
		if ex.Bits() <= target.Bits() && ex.Contains(target.Addr()) {
			return true
		}
	}
	return false
}
