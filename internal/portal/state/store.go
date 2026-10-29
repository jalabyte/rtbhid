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

func (s *Store) BulkCreatePrefixes(cidrs []string, note string) (created int, skipped int, invalid int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO prefixes (cidr, enabled, note, created_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, cidr := range cidrs {
		normed, err := normalizeCIDR(cidr)
		if err != nil {
			invalid++
			continue
		}
		res, err := stmt.Exec(normed, true, note, now)
		if err != nil {
			return 0, 0, 0, err
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			created++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, 0, err
	}
	return created, skipped, invalid, nil
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

func (s *Store) ToggleSource(id string, enabled bool) error {
	_, err := s.db.Exec(`UPDATE sources SET enabled = ? WHERE id = ?`, enabled, id)
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

// GetSourcePrefixes retrieves paginated prefixes for a specific source.
// Returns prefixes, total count, and error.
func (s *Store) GetSourcePrefixes(sourceID string, page int, limit int) ([]string, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 10000 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Get total count
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM source_prefixes WHERE source_id = ?`, sourceID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated prefixes
	rows, err := s.db.Query(`SELECT cidr FROM source_prefixes WHERE source_id = ? ORDER BY cidr LIMIT ? OFFSET ?`, sourceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var prefixes []string
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			continue
		}
		prefixes = append(prefixes, cidr)
	}
	return prefixes, total, nil
}

// GetExcludeSourcePrefixes retrieves paginated prefixes for a specific exclude source.
// Returns prefixes, total count, and error.
func (s *Store) GetExcludeSourcePrefixes(sourceID string, page int, limit int) ([]string, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 10000 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Get total count
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM exclude_source_prefixes WHERE source_id = ?`, sourceID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated prefixes
	rows, err := s.db.Query(`SELECT cidr FROM exclude_source_prefixes WHERE source_id = ? ORDER BY cidr LIMIT ? OFFSET ?`, sourceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var prefixes []string
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			continue
		}
		prefixes = append(prefixes, cidr)
	}
	return prefixes, total, nil
}

// ExcludeSourcePrefixItem represents a single exclude source prefix with source info
type ExcludeSourcePrefixItem struct {
	SourceID   string    `json:"source_id"`
	SourceName string    `json:"source_name"`
	CIDR       string    `json:"cidr"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListExcludeSourcePrefixes returns paginated prefixes from exclude_source_prefixes with source info
func (s *Store) ListExcludeSourcePrefixes(offset, limit int) ([]ExcludeSourcePrefixItem, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 10000 {
		limit = 100
	}

	// Get total count
	var total int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM exclude_source_prefixes`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated prefixes with source info
	rows, err := s.db.Query(`
		SELECT es.id, es.name, esp.cidr, es.created_at
		FROM exclude_source_prefixes esp
		INNER JOIN exclude_sources es ON es.id = esp.source_id
		ORDER BY es.created_at DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []ExcludeSourcePrefixItem
	for rows.Next() {
		var item ExcludeSourcePrefixItem
		if err := rows.Scan(&item.SourceID, &item.SourceName, &item.CIDR, &item.CreatedAt); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, total, nil
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

func (s *Store) BulkCreateExcludes(cidrs []string, note string) (created int, skipped int, invalid int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO excludes (cidr, note, created_at) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, cidr := range cidrs {
		normed, err := normalizeCIDR(cidr)
		if err != nil {
			invalid++
			continue
		}
		res, err := stmt.Exec(normed, note, now)
		if err != nil {
			return 0, 0, 0, err
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			created++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, 0, err
	}
	return created, skipped, invalid, nil
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

func (s *Store) ToggleExcludeSource(id string, enabled bool) error {
	_, err := s.db.Exec(`UPDATE exclude_sources SET enabled = ? WHERE id = ?`, enabled, id)
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

func (s *Store) TogglePeer(id string, enabled bool) error {
	_, err := s.db.Exec(`UPDATE peers SET enabled = ? WHERE id = ?`, enabled, id)
	return err
}

// --------------------------------------------------------------------------
// ASN whitelist CRUD
// --------------------------------------------------------------------------

func (s *Store) ListWhitelistASNs() []WhitelistASN {
	rows, err := s.db.Query(`
		SELECT id, asn, COALESCE(name, ''), COALESCE(country, ''), COALESCE(source, ''), enabled, created_at
		FROM asn_whitelist ORDER BY asn`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []WhitelistASN
	for rows.Next() {
		var w WhitelistASN
		if err := rows.Scan(&w.ID, &w.ASN, &w.Name, &w.Country, &w.Source, &w.Enabled, &w.CreatedAt); err != nil {
			continue
		}
		out = append(out, w)
	}
	return out
}

// ListEnabledWhitelistASNs returns the ASNs pushed to agents for auto-accept.
func (s *Store) ListEnabledWhitelistASNs() []uint32 {
	rows, err := s.db.Query(`SELECT asn FROM asn_whitelist WHERE enabled = 1 ORDER BY asn`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []uint32
	for rows.Next() {
		var asn uint32
		if err := rows.Scan(&asn); err == nil {
			out = append(out, asn)
		}
	}
	return out
}

func (s *Store) GetWhitelistASN(asn uint32) (WhitelistASN, bool) {
	var w WhitelistASN
	err := s.db.QueryRow(`
		SELECT id, asn, COALESCE(name, ''), COALESCE(country, ''), COALESCE(source, ''), enabled, created_at
		FROM asn_whitelist WHERE asn = ?`, asn).
		Scan(&w.ID, &w.ASN, &w.Name, &w.Country, &w.Source, &w.Enabled, &w.CreatedAt)
	if err != nil {
		return WhitelistASN{}, false
	}
	return w, true
}

func (s *Store) CreateWhitelistASN(w WhitelistASN) (WhitelistASN, error) {
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now().UTC()
	}
	if w.Country == "" {
		w.Country = "ID"
	}
	if w.Source == "" {
		w.Source = "manual"
	}
	_, err := s.db.Exec(`
		INSERT INTO asn_whitelist (id, asn, name, country, source, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.ASN, w.Name, w.Country, w.Source, w.Enabled, w.CreatedAt,
	)
	if err != nil {
		return WhitelistASN{}, err
	}
	return w, nil
}

// UpsertWhitelistASN inserts the entry when the ASN is not present. It never
// changes an existing entry's enabled state. The boolean reports whether a new
// row was created.
func (s *Store) UpsertWhitelistASN(w WhitelistASN) (bool, error) {
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now().UTC()
	}
	if w.Country == "" {
		w.Country = "ID"
	}
	if w.Source == "" {
		w.Source = "apnic"
	}
	res, err := s.db.Exec(`
		INSERT OR IGNORE INTO asn_whitelist (id, asn, name, country, source, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.ASN, w.Name, w.Country, w.Source, w.Enabled, w.CreatedAt,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) DeleteWhitelistASN(id string) error {
	res, err := s.db.Exec(`DELETE FROM asn_whitelist WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("whitelist entry %q not found", id)
	}
	return nil
}

func (s *Store) DeleteWhitelistASNByASN(asn uint32) error {
	_, err := s.db.Exec(`DELETE FROM asn_whitelist WHERE asn = ?`, asn)
	return err
}

func (s *Store) ToggleWhitelistASN(id string, enabled bool) error {
	res, err := s.db.Exec(`UPDATE asn_whitelist SET enabled = ? WHERE id = ?`, enabled, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("whitelist entry %q not found", id)
	}
	return nil
}

// ReplaceWhitelistBySource removes entries from the given source and inserts
// the supplied entries. Manual entries are never touched.
func (s *Store) ReplaceWhitelistBySource(source string, entries []WhitelistASN) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM asn_whitelist WHERE source = ?`, source); err != nil {
		return 0, err
	}
	created := 0
	for _, w := range entries {
		if w.CreatedAt.IsZero() {
			w.CreatedAt = time.Now().UTC()
		}
		if w.Country == "" {
			w.Country = "ID"
		}
		if w.Source == "" {
			w.Source = source
		}
		res, err := tx.Exec(`
			INSERT OR IGNORE INTO asn_whitelist (id, asn, name, country, source, enabled, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			w.ID, w.ASN, w.Name, w.Country, w.Source, w.Enabled, w.CreatedAt,
		)
		if err != nil {
			return created, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			created++
		}
	}
	if err := tx.Commit(); err != nil {
		return created, err
	}
	return created, nil
}

func (s *Store) CountWhitelistASNs() int {
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM asn_whitelist`).Scan(&n)
	return n
}

// --------------------------------------------------------------------------
// BGP telemetry
// --------------------------------------------------------------------------

// ReplaceAgentBGPSessions atomically replaces the reported sessions for an
// agent and records the engine name.
func (s *Store) ReplaceAgentBGPSessions(agentID, engine string, sessions []BGPSession) error {
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO agent_bgp_status (agent_id, engine, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET engine = excluded.engine, updated_at = excluded.updated_at`,
		agentID, engine, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM agent_bgp_sessions WHERE agent_id = ?`, agentID); err != nil {
		return err
	}
	for _, sess := range sessions {
		if sess.Peer == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO agent_bgp_sessions (agent_id, peer, name, remote_as, local_as, state, passive, uptime_sec, prefixes_in, last_error, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			agentID, sess.Peer, sess.Name, sess.RemoteAS, sess.LocalAS, sess.State,
			sess.Passive, sess.UptimeSec, sess.PrefixesIn, sess.LastError, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListAgentBGPSessions() []BGPSession {
	rows, err := s.db.Query(`
		SELECT b.agent_id, COALESCE(a.name, ''), b.peer, COALESCE(b.name, ''), b.remote_as, b.local_as,
		       COALESCE(b.state, ''), b.passive, b.uptime_sec, b.prefixes_in, COALESCE(b.last_error, ''), b.updated_at
		FROM agent_bgp_sessions b
		LEFT JOIN agents a ON a.id = b.agent_id
		ORDER BY a.name, b.peer`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []BGPSession
	for rows.Next() {
		var sess BGPSession
		if err := rows.Scan(&sess.AgentID, &sess.AgentName, &sess.Peer, &sess.Name, &sess.RemoteAS, &sess.LocalAS,
			&sess.State, &sess.Passive, &sess.UptimeSec, &sess.PrefixesIn, &sess.LastError, &sess.UpdatedAt); err != nil {
			continue
		}
		out = append(out, sess)
	}
	return out
}

// AgentBGPEngines returns agentID -> engine name and last report time.
func (s *Store) AgentBGPEngines() map[string]string {
	rows, err := s.db.Query(`SELECT agent_id, COALESCE(engine, '') FROM agent_bgp_status`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var id, engine string
		if err := rows.Scan(&id, &engine); err == nil {
			out[id] = engine
		}
	}
	return out
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
// LookupIP - query IP/CIDR against sources, manual prefixes, and excludes
// --------------------------------------------------------------------------

func (s *Store) LookupIP(queries []string) ([]LookupResult, error) {
	if len(queries) == 0 {
		return []LookupResult{}, nil
	}

	results := make([]LookupResult, 0, len(queries))

	for _, query := range queries {
		// Detect CIDR format before normalization (normalizeCIDR converts bare IPs to /32)
		isQueryCIDR := strings.Contains(query, "/")

		normalized, err := normalizeCIDR(query)
		if err != nil {
			return nil, fmt.Errorf("invalid query '%s': %w", query, err)
		}

		var queryAddr netip.Addr
		var queryPrefix netip.Prefix

		if p, err := netip.ParsePrefix(normalized); err == nil {
			queryPrefix = p
			queryAddr = p.Addr()
		} else if addr, err := netip.ParseAddr(normalized); err == nil {
			queryAddr = addr
		} else {
			return nil, fmt.Errorf("invalid query '%s': %w", query, err)
		}

		result := LookupResult{
			Query:   query,
			Matches: []LookupMatch{},
		}

		var matches []LookupMatch

		sourceRows, err := s.db.Query(`
			SELECT s.id, s.name, sp.cidr, s.created_at
			FROM source_prefixes sp
			INNER JOIN sources s ON s.id = sp.source_id
			WHERE s.enabled = 1`)
		if err != nil {
			return nil, fmt.Errorf("failed to query sources: %w", err)
		}
		for sourceRows.Next() {
			var sourceID, sourceName, cidr string
			var createdAt time.Time
			if err := sourceRows.Scan(&sourceID, &sourceName, &cidr, &createdAt); err != nil {
				continue
			}
			if storedPrefix, err := netip.ParsePrefix(cidr); err == nil {
				var matched bool
				if isQueryCIDR {
					// Query is CIDR: check if stored prefix is a sub-prefix of query prefix
					// i.e., query prefix contains stored prefix's network
					matched = queryPrefix.Bits() <= storedPrefix.Bits() &&
						queryPrefix.Contains(storedPrefix.Addr())
				} else {
					// Query is bare IP: check if stored prefix contains the IP
					matched = storedPrefix.Contains(queryAddr)
				}
				if matched {
					matches = append(matches, LookupMatch{
						Type:       "source",
						SourceID:   sourceID,
						SourceName: sourceName,
						CIDR:       cidr,
						CreatedAt:  createdAt,
					})
				}
			}
		}
		sourceRows.Close()

		prefixRows, err := s.db.Query(`
			SELECT cidr, note, created_at
			FROM prefixes
			WHERE enabled = 1`)
		if err != nil {
			return nil, fmt.Errorf("failed to query prefixes: %w", err)
		}
		for prefixRows.Next() {
			var cidr, note string
			var createdAt time.Time
			if err := prefixRows.Scan(&cidr, &note, &createdAt); err != nil {
				continue
			}
			if storedPrefix, err := netip.ParsePrefix(cidr); err == nil {
				var matched bool
				if isQueryCIDR {
					// Query is CIDR: check if stored prefix is a sub-prefix of query prefix
					matched = queryPrefix.Bits() <= storedPrefix.Bits() &&
						queryPrefix.Contains(storedPrefix.Addr())
				} else {
					// Query is bare IP: check if stored prefix contains the IP
					matched = storedPrefix.Contains(queryAddr)
				}
				if matched {
					matches = append(matches, LookupMatch{
						Type:      "manual",
						CIDR:      cidr,
						Note:      note,
						CreatedAt: createdAt,
					})
				}
			}
		}
		prefixRows.Close()

		excludeSet := make(map[string]bool)
		var excludeParsed []netip.Prefix

		excludeRows, err := s.db.Query(`SELECT cidr, note, created_at FROM excludes`)
		if err == nil {
			for excludeRows.Next() {
				var cidr, note string
				var createdAt time.Time
				if err := excludeRows.Scan(&cidr, &note, &createdAt); err == nil {
					excludeSet[cidr] = true
					if pref, err := netip.ParsePrefix(cidr); err == nil {
						excludeParsed = append(excludeParsed, pref.Masked())

						var matched bool
						if isQueryCIDR {
							matched = queryPrefix.Bits() <= pref.Bits() &&
								queryPrefix.Contains(pref.Addr())
						} else {
							matched = pref.Contains(queryAddr)
						}
						if matched {
							matches = append(matches, LookupMatch{
								Type:      "exclude",
								CIDR:      cidr,
								Note:      note,
								CreatedAt: createdAt,
							})
						}
					}
				}
			}
			excludeRows.Close()
		}

		excludeSourceRows, err := s.db.Query(`
			SELECT esp.cidr, es.id, es.name, es.created_at
			FROM exclude_source_prefixes esp
			INNER JOIN exclude_sources es ON es.id = esp.source_id
			WHERE es.enabled = 1`)
		if err == nil {
			for excludeSourceRows.Next() {
				var cidr, sourceID, sourceName string
				var createdAt time.Time
				if err := excludeSourceRows.Scan(&cidr, &sourceID, &sourceName, &createdAt); err == nil {
					excludeSet[cidr] = true
					if pref, err := netip.ParsePrefix(cidr); err == nil {
						excludeParsed = append(excludeParsed, pref.Masked())

						var matched bool
						if isQueryCIDR {
							matched = queryPrefix.Bits() <= pref.Bits() &&
								queryPrefix.Contains(pref.Addr())
						} else {
							matched = pref.Contains(queryAddr)
						}
						if matched {
							matches = append(matches, LookupMatch{
								Type:       "exclude",
								SourceID:   sourceID,
								SourceName: sourceName,
								CIDR:       cidr,
								CreatedAt:  createdAt,
							})
						}
					}
				}
			}
			excludeSourceRows.Close()
		}

		filteredMatches := make([]LookupMatch, 0, len(matches))
		for _, match := range matches {
			if match.Type == "exclude" {
				filteredMatches = append(filteredMatches, match)
				continue
			}
			if excludeSet[match.CIDR] {
				continue
			}
			if pref, err := netip.ParsePrefix(match.CIDR); err == nil {
				if isExcludedByCIDR(pref, excludeParsed) {
					continue
				}
			}
			filteredMatches = append(filteredMatches, match)
		}

		sort.Slice(filteredMatches, func(i, j int) bool {
			var bitsI, bitsJ int
			if pref, err := netip.ParsePrefix(filteredMatches[i].CIDR); err == nil {
				bitsI = pref.Bits()
			}
			if pref, err := netip.ParsePrefix(filteredMatches[j].CIDR); err == nil {
				bitsJ = pref.Bits()
			}
			return bitsI > bitsJ
		})

		result.Matches = filteredMatches
		results = append(results, result)
	}

	return results, nil
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
