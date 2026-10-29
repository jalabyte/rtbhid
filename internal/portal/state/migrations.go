package state

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS agents (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		hostname      TEXT,
		bird_version  TEXT,
		listen_addr   TEXT,
		status        TEXT DEFAULT 'offline',
		last_seen     DATETIME,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS prefixes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		cidr        TEXT NOT NULL UNIQUE,
		enabled     BOOLEAN DEFAULT 1,
		note        TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS sources (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		url         TEXT NOT NULL,
		interval    INTEGER DEFAULT 3600,
		enabled     BOOLEAN DEFAULT 1,
		last_sync   DATETIME,
		last_count  INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS source_prefixes (
		source_id   TEXT REFERENCES sources(id) ON DELETE CASCADE,
		cidr        TEXT NOT NULL,
		PRIMARY KEY (source_id, cidr)
	)`,
	`CREATE TABLE IF NOT EXISTS excludes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		cidr        TEXT NOT NULL UNIQUE,
		note        TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS peers (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		neighbor_ip   TEXT NOT NULL,
		remote_as     INTEGER NOT NULL,
		local_as      INTEGER DEFAULT 65500,
		multihop      INTEGER DEFAULT 1,
		enabled       BOOLEAN DEFAULT 1,
		std_communities    TEXT,
		large_communities  TEXT,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS exclude_sources (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		url         TEXT NOT NULL,
		interval    INTEGER DEFAULT 3600,
		enabled     BOOLEAN DEFAULT 1,
		last_sync   DATETIME,
		last_count  INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS exclude_source_prefixes (
		source_id   TEXT REFERENCES exclude_sources(id) ON DELETE CASCADE,
		cidr        TEXT NOT NULL,
		PRIMARY KEY (source_id, cidr)
	)`,
	`CREATE TABLE IF NOT EXISTS asn_whitelist (
		id          TEXT PRIMARY KEY,
		asn         INTEGER NOT NULL UNIQUE,
		name        TEXT,
		country     TEXT DEFAULT 'ID',
		source      TEXT DEFAULT 'manual',
		enabled     BOOLEAN DEFAULT 1,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_asn_whitelist_enabled ON asn_whitelist(enabled)`,
	`CREATE TABLE IF NOT EXISTS events (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		type        TEXT NOT NULL,
		message     TEXT NOT NULL,
		agent_id    TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS id_asn_registry (
		asn         INTEGER PRIMARY KEY,
		updated_at  DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS agent_bgp_status (
		agent_id    TEXT PRIMARY KEY,
		engine      TEXT,
		updated_at  DATETIME
	)`,
	`CREATE TABLE IF NOT EXISTS agent_bgp_sessions (
		agent_id    TEXT NOT NULL,
		peer        TEXT NOT NULL,
		name        TEXT,
		remote_as   INTEGER DEFAULT 0,
		local_as    INTEGER DEFAULT 0,
		state       TEXT,
		passive     BOOLEAN DEFAULT 0,
		uptime_sec  INTEGER DEFAULT 0,
		prefixes_in INTEGER DEFAULT 0,
		last_error  TEXT,
		updated_at  DATETIME,
		PRIMARY KEY (agent_id, peer)
	)`,
	`UPDATE peers SET id = 'peer_' || lower(hex(randomblob(4))) || lower(hex(randomblob(2))) || lower(hex(randomblob(2))) || lower(hex(randomblob(2))) || lower(hex(randomblob(6))) WHERE id IS NULL OR id = ''`,
}
