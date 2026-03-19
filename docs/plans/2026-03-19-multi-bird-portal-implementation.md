# Multi-Bird Centralized Portal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform RTBHID from a single-instance RTBH controller into a centralized Go portal (gRPC + REST API) that manages multiple Bird 2/3 agents on different servers, with a React 19 + Vite 8 + Tailwind CSS 4 + shadcn/ui 4 + Lucide React frontend.

**Architecture:** Portal binary serves both a REST API (for the React dashboard) and a gRPC API (for remote agents). Agents extend the existing Go binary with a gRPC client that registers with the portal, receives config via server-side streaming, generates Bird config files locally, and reports health via heartbeats. Portal state moves from JSON file to SQLite. React app builds to `web/dist/` and is embedded into the portal binary via `//go:embed`.

**Tech Stack:** Go 1.23 (stdlib + SQLite via modernc.org/sqlite + gRPC via google.golang.org/grpc), React 19.2.4, Vite 8.0.1, Tailwind CSS 4.2.2, shadcn/ui CLI 4.0.8, Lucide React 0.577.0, Protobuf

---

## Phase 1: Project Restructuring

### Task 1: Create directory structure and move existing code

**Files:**
- Create: `cmd/portal/main.go` (placeholder)
- Create: `cmd/agent/main.go` (copy from `cmd/rtbhd/main.go`)
- Create: `internal/shared/types.go` (placeholder)
- Create: `internal/portal/` directory
- Create: `internal/agent/grpcclient/` directory
- Create: `internal/agent/bird/` (move from `internal/bird/`)
- Create: `internal/agent/config/` (move from `internal/config/`)
- Create: `proto/` directory
- Move: `internal/bird/` → `internal/agent/bird/`
- Move: `internal/config/` → `internal/agent/config/`
- Keep: `internal/httpapi/` (will be repurposed for portal later)
- Keep: `internal/state/` (will be replaced by SQLite later)
- Keep: `internal/app/` (will be repurposed for portal later)
- Keep: `internal/syncer/` (stays on portal side)

**Step 1: Create directories**

```bash
mkdir -p cmd/portal cmd/agent internal/portal/httpapi internal/portal/grpcapi internal/portal/state internal/agent/grpcclient internal/agent/bird internal/agent/config internal/shared proto gen/rtbhid web/src/{components/{ui,layout,agents,prefixes,sources,peers,events},pages,lib}
```

**Step 2: Move bird and config packages**

```bash
mv internal/bird internal/agent/bird
mv internal/config internal/agent/config
```

**Step 3: Update all import paths in moved files**

Update `internal/agent/bird/manager.go`: no import changes needed (no internal imports).
Update `internal/agent/config/config.go`: no import changes needed (no internal imports).

**Step 4: Move cmd/rtbhd to cmd/agent**

```bash
cp cmd/rtbhd/main.go cmd/agent/main.go
```

Update `cmd/agent/main.go` imports:
```go
import (
    "rtbhid/internal/agent/bird"
    "rtbhid/internal/agent/config"
    // keep other imports pointing to their current paths
)
```

**Step 5: Create placeholder cmd/portal/main.go**

```go
package main

import "fmt"

func main() {
    fmt.Println("rtbh portal - placeholder")
}
```

**Step 6: Verify build still works**

```bash
cd /Users/billyriantono/Works/rtbhid
go build ./cmd/agent/
go build ./cmd/portal/
```

**Step 7: Commit**

```bash
git add -A
git commit -m "refactor: restructure project for portal/agent split

Move bird and config packages under internal/agent/.
Create cmd/portal and cmd/agent entry points.
Create directory structure for portal gRPC, state, and React frontend."
```

---

## Phase 2: Protobuf & gRPC Setup

### Task 2: Define protobuf schema and generate gRPC code

**Files:**
- Create: `proto/rtbhid.proto`
- Create: `buf.yaml` (buf config for proto management)
- Create: `buf.gen.yaml` (code generation config)
- Generate: `gen/rtbhid/*.go` (generated gRPC code)
- Modify: `go.mod` (add gRPC + protobuf + SQLite dependencies)

**Step 1: Define proto file**

```protobuf
// proto/rtbhid.proto
syntax = "proto3";
package rtbhid;
option go_package = "rtbhid/gen/rtbhid";

service Portal {
    rpc Register(RegisterRequest) returns (RegisterResponse);
    rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
    rpc StreamConfig(ConfigRequest) returns (stream ConfigUpdate);
}

message RegisterRequest {
    string agent_id = 1;
    string name = 2;
    string bird_version = 3;
    string version = 4;
}

message RegisterResponse {
    bool success = 1;
    string message = 2;
    int32 heartbeat_interval = 3;
}

message HeartbeatRequest {
    string agent_id = 1;
    AgentStatus status = 2;
    int32 prefix_count = 3;
    int32 peer_count = 4;
    string bird_status = 5;
}

message HeartbeatResponse {
    bool success = 1;
}

message AgentStatus {
    bool online = 1;
    string bird_version = 2;
    int32 active_prefixes = 3;
    int32 active_peers = 4;
}

message ConfigRequest {
    string agent_id = 1;
}

message ConfigUpdate {
    repeated string prefixes = 1;
    repeated PeerConfig peers = 2;
    BirdConfig bird = 3;
    int64 version = 4;
}

message PeerConfig {
    string name = 1;
    string neighbor_ip = 2;
    uint32 remote_as = 3;
    uint32 local_as = 4;
    uint32 multihop = 5;
    repeated string std_communities = 6;
    repeated string large_communities = 7;
}

message BirdConfig {
    string output_path = 1;
    string peers_output_path = 2;
    bool reload_enabled = 3;
    string reload_command = 4;
}
```

**Step 2: Install buf and generate code**

```bash
# Install buf CLI
go install github.com/bufbuild/buf/cmd/buf@latest

# Create buf.yaml
cat > buf.yaml << 'EOF'
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
EOF

# Create buf.gen.yaml
cat > buf.gen.yaml << 'EOF'
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: rtbhid/gen
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt:
      - paths=source_relative
  - remote: buf.build/grpc/go
    out: gen
    opt:
      - paths=source_relative
EOF

# Generate
buf generate proto
```

**Step 3: Add Go dependencies**

```bash
go get google.golang.org/grpc
go get google.golang.org/protobuf
go get modernc.org/sqlite
```

**Step 4: Verify generated code compiles**

```bash
go build ./gen/...
```

**Step 5: Commit**

```bash
git add proto/ buf.yaml buf.gen.yaml gen/ go.mod go.sum
git commit -m "feat: add protobuf schema and generate gRPC code

Define Portal service with Register, Heartbeat, StreamConfig RPCs.
Set up buf for proto management and code generation."
```

---

## Phase 3: Portal SQLite State Store

### Task 3: Implement SQLite store for portal state

**Files:**
- Create: `internal/portal/state/store.go`
- Create: `internal/portal/state/types.go`
- Create: `internal/portal/state/migrations.go`

**Step 1: Define portal state types**

```go
// internal/portal/state/types.go
package state

import "time"

type Agent struct {
    ID           string     `json:"id"`
    Name         string     `json:"name"`
    Hostname     string     `json:"hostname,omitempty"`
    BirdVersion  string     `json:"bird_version,omitempty"`
    Status       string     `json:"status"`
    LastSeen     *time.Time `json:"last_seen,omitempty"`
    CreatedAt    time.Time  `json:"created_at"`
    UpdatedAt    time.Time  `json:"updated_at"`
}

type Prefix struct {
    ID        int64     `json:"id"`
    CIDR      string    `json:"cidr"`
    Enabled   bool      `json:"enabled"`
    Note      string    `json:"note,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

type Source struct {
    ID              string     `json:"id"`
    Name            string     `json:"name"`
    URL             string     `json:"url"`
    Interval        int        `json:"interval"`
    Enabled         bool       `json:"enabled"`
    LastSync        *time.Time `json:"last_sync,omitempty"`
    LastCount       int        `json:"last_count"`
    CreatedAt       time.Time  `json:"created_at"`
}

type Exclude struct {
    ID        int64     `json:"id"`
    CIDR      string    `json:"cidr"`
    Note      string    `json:"note,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

type Peer struct {
    ID               string   `json:"id"`
    Name             string   `json:"name"`
    NeighborIP       string   `json:"neighbor_ip"`
    RemoteAS         int      `json:"remote_as"`
    LocalAS          int      `json:"local_as"`
    Multihop         int      `json:"multihop"`
    Enabled          bool     `json:"enabled"`
    StdCommunities   []string `json:"std_communities,omitempty"`
    LargeCommunities []string `json:"large_communities,omitempty"`
    CreatedAt        time.Time `json:"created_at"`
}

type Event struct {
    ID        int64     `json:"id"`
    Type      string    `json:"type"`
    Message   string    `json:"message"`
    AgentID   string    `json:"agent_id,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}
```

**Step 2: Create SQLite store with schema migrations**

```go
// internal/portal/state/migrations.go
package state

const schema = `
CREATE TABLE IF NOT EXISTS agents (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    hostname      TEXT DEFAULT '',
    bird_version  TEXT DEFAULT '',
    status        TEXT DEFAULT 'offline',
    last_seen     DATETIME,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS prefixes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cidr        TEXT NOT NULL UNIQUE,
    enabled     INTEGER DEFAULT 1,
    note        TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sources (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    url         TEXT NOT NULL,
    interval    INTEGER DEFAULT 3600,
    enabled     INTEGER DEFAULT 1,
    last_sync   DATETIME,
    last_count  INTEGER DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS source_prefixes (
    source_id   TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    cidr        TEXT NOT NULL,
    PRIMARY KEY (source_id, cidr)
);

CREATE TABLE IF NOT EXISTS excludes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cidr        TEXT NOT NULL UNIQUE,
    note        TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS peers (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    neighbor_ip   TEXT NOT NULL,
    remote_as     INTEGER NOT NULL,
    local_as      INTEGER DEFAULT 65500,
    multihop      INTEGER DEFAULT 1,
    enabled       INTEGER DEFAULT 1,
    std_communities   TEXT DEFAULT '[]',
    large_communities TEXT DEFAULT '[]',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    type        TEXT NOT NULL,
    message     TEXT NOT NULL,
    agent_id    TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
`
```

**Step 3: Implement Store struct and constructor**

```go
// internal/portal/state/store.go
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
    db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)")
    if err != nil {
        return nil, fmt.Errorf("open sqlite: %w", err)
    }
    if _, err := db.Exec(schema); err != nil {
        db.Close()
        return nil, fmt.Errorf("run schema: %w", err)
    }
    return &Store{db: db}, nil
}

func (s *Store) Close() error {
    return s.db.Close()
}
```

**Step 4: Implement agent CRUD methods**

```go
// Add to store.go

func (s *Store) UpsertAgent(a Agent) error {
    now := time.Now().UTC()
    _, err := s.db.Exec(`
        INSERT INTO agents (id, name, hostname, bird_version, status, last_seen, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            name=excluded.name, hostname=excluded.hostname,
            bird_version=excluded.bird_version, status=excluded.status,
            last_seen=excluded.last_seen, updated_at=excluded.updated_at
    `, a.ID, a.Name, a.Hostname, a.BirdVersion, a.Status, a.LastSeen, now, now)
    return err
}

func (s *Store) GetAgent(id string) (Agent, bool) {
    var a Agent
    var lastSeen sql.NullTime
    err := s.db.QueryRow("SELECT id, name, hostname, bird_version, status, last_seen, created_at, updated_at FROM agents WHERE id = ?", id).
        Scan(&a.ID, &a.Name, &a.Hostname, &a.BirdVersion, &a.Status, &lastSeen, &a.CreatedAt, &a.UpdatedAt)
    if err != nil {
        return Agent{}, false
    }
    if lastSeen.Valid {
        a.LastSeen = &lastSeen.Time
    }
    return a, true
}

func (s *Store) ListAgents() []Agent {
    rows, err := s.db.Query("SELECT id, name, hostname, bird_version, status, last_seen, created_at, updated_at FROM agents ORDER BY name")
    if err != nil {
        return nil
    }
    defer rows.Close()
    var agents []Agent
    for rows.Next() {
        var a Agent
        var lastSeen sql.NullTime
        if err := rows.Scan(&a.ID, &a.Name, &a.Hostname, &a.BirdVersion, &a.Status, &lastSeen, &a.CreatedAt, &a.UpdatedAt); err != nil {
            continue
        }
        if lastSeen.Valid {
            a.LastSeen = &lastSeen.Time
        }
        agents = append(agents, a)
    }
    return agents
}

func (s *Store) DeleteAgent(id string) error {
    _, err := s.db.Exec("DELETE FROM agents WHERE id = ?", id)
    return err
}

func (s *Store) UpdateAgentHeartbeat(id string, status string, prefixCount, peerCount int, birdStatus string) error {
    now := time.Now().UTC()
    _, err := s.db.Exec("UPDATE agents SET status=?, last_seen=? WHERE id=?", "online", now, id)
    return err
}
```

**Step 5: Implement prefix CRUD methods**

```go
// Add to store.go

func (s *Store) AddPrefix(cidr string, enabled bool, note string) (Prefix, error) {
    norm, err := normalizeCIDR(cidr)
    if err != nil {
        return Prefix{}, err
    }
    now := time.Now().UTC()
    res, err := s.db.Exec("INSERT OR IGNORE INTO prefixes (cidr, enabled, note, created_at) VALUES (?, ?, ?, ?)",
        norm, enabled, note, now)
    if err != nil {
        return Prefix{}, err
    }
    id, _ := res.LastInsertId()
    // If INSERT was ignored (duplicate), fetch existing
    if id == 0 {
        return s.GetPrefixByCIDR(norm)
    }
    return Prefix{ID: id, CIDR: norm, Enabled: enabled, Note: note, CreatedAt: now}, nil
}

func (s *Store) GetPrefixByCIDR(cidr string) (Prefix, error) {
    var p Prefix
    err := s.db.QueryRow("SELECT id, cidr, enabled, note, created_at FROM prefixes WHERE cidr = ?", cidr).
        Scan(&p.ID, &p.CIDR, &p.Enabled, &p.Note, &p.CreatedAt)
    return p, err
}

func (s *Store) ListPrefixes() []Prefix {
    rows, err := s.db.Query("SELECT id, cidr, enabled, note, created_at FROM prefixes ORDER BY cidr")
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
    _, err := s.db.Exec("DELETE FROM prefixes WHERE id = ?", id)
    return err
}

func (s *Store) TogglePrefix(id int64, enabled bool) error {
    _, err := s.db.Exec("UPDATE prefixes SET enabled = ? WHERE id = ?", enabled, id)
    return err
}
```

**Step 6: Implement source CRUD methods**

```go
// Add to store.go

func (s *Store) CreateSource(src Source) error {
    _, err := s.db.Exec("INSERT INTO sources (id, name, url, interval, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)",
        src.ID, src.Name, src.URL, src.Interval, src.Enabled, time.Now().UTC())
    return err
}

func (s *Store) GetSource(id string) (Source, bool) {
    var src Source
    var lastSync sql.NullTime
    err := s.db.QueryRow("SELECT id, name, url, interval, enabled, last_sync, last_count, created_at FROM sources WHERE id = ?", id).
        Scan(&src.ID, &src.Name, &src.URL, &src.Interval, &src.Enabled, &lastSync, &src.LastCount, &src.CreatedAt)
    if err != nil {
        return Source{}, false
    }
    if lastSync.Valid {
        src.LastSync = &lastSync.Time
    }
    return src, true
}

func (s *Store) ListSources() []Source {
    rows, err := s.db.Query("SELECT id, name, url, interval, enabled, last_sync, last_count, created_at FROM sources ORDER BY name")
    if err != nil {
        return nil
    }
    defer rows.Close()
    var out []Source
    for rows.Next() {
        var src Source
        var lastSync sql.NullTime
        if err := rows.Scan(&src.ID, &src.Name, &src.URL, &src.Interval, &src.Enabled, &lastSync, &src.LastCount, &src.CreatedAt); err != nil {
            continue
        }
        if lastSync.Valid {
            src.LastSync = &lastSync.Time
        }
        out = append(out, src)
    }
    return out
}

func (s *Store) UpdateSource(src Source) error {
    _, err := s.db.Exec("UPDATE sources SET name=?, url=?, interval=?, enabled=? WHERE id=?",
        src.Name, src.URL, src.Interval, src.Enabled, src.ID)
    return err
}

func (s *Store) DeleteSource(id string) error {
    _, err := s.db.Exec("DELETE FROM sources WHERE id = ?", id)
    return err
}

func (s *Store) SetSourcePrefixes(sourceID string, prefixes []string) error {
    tx, err := s.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    if _, err := tx.Exec("DELETE FROM source_prefixes WHERE source_id = ?", sourceID); err != nil {
        return err
    }
    stmt, err := tx.Prepare("INSERT OR IGNORE INTO source_prefixes (source_id, cidr) VALUES (?, ?)")
    if err != nil {
        return err
    }
    defer stmt.Close()
    for _, p := range prefixes {
        if _, err := stmt.Exec(sourceID, p); err != nil {
            return err
        }
    }
    if _, err := tx.Exec("UPDATE sources SET last_sync=?, last_count=? WHERE id=?",
        time.Now().UTC(), len(prefixes), sourceID); err != nil {
        return err
    }
    return tx.Commit()
}
```

**Step 7: Implement exclude and peer CRUD methods**

```go
// Add to store.go

func (s *Store) AddExclude(cidr string, note string) (Exclude, error) {
    norm, err := normalizeCIDR(cidr)
    if err != nil {
        return Exclude{}, err
    }
    now := time.Now().UTC()
    res, err := s.db.Exec("INSERT OR IGNORE INTO excludes (cidr, note, created_at) VALUES (?, ?, ?)", norm, note, now)
    if err != nil {
        return Exclude{}, err
    }
    id, _ := res.LastInsertId()
    if id == 0 {
        var ex Exclude
        err = s.db.QueryRow("SELECT id, cidr, note, created_at FROM excludes WHERE cidr = ?", norm).
            Scan(&ex.ID, &ex.CIDR, &ex.Note, &ex.CreatedAt)
        return ex, err
    }
    return Exclude{ID: id, CIDR: norm, Note: note, CreatedAt: now}, nil
}

func (s *Store) ListExcludes() []Exclude {
    rows, err := s.db.Query("SELECT id, cidr, note, created_at FROM excludes ORDER BY cidr")
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
    _, err := s.db.Exec("DELETE FROM excludes WHERE id = ?", id)
    return err
}

func (s *Store) CreatePeer(p Peer) error {
    stdJSON, _ := json.Marshal(p.StdCommunities)
    largeJSON, _ := json.Marshal(p.LargeCommunities)
    _, err := s.db.Exec("INSERT INTO peers (id, name, neighbor_ip, remote_as, local_as, multihop, enabled, std_communities, large_communities, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
        p.ID, p.Name, p.NeighborIP, p.RemoteAS, p.LocalAS, p.Multihop, p.Enabled, string(stdJSON), string(largeJSON), time.Now().UTC())
    return err
}

func (s *Store) GetPeer(id string) (Peer, bool) {
    var p Peer
    var stdJSON, largeJSON string
    err := s.db.QueryRow("SELECT id, name, neighbor_ip, remote_as, local_as, multihop, enabled, std_communities, large_communities, created_at FROM peers WHERE id = ?", id).
        Scan(&p.ID, &p.Name, &p.NeighborIP, &p.RemoteAS, &p.LocalAS, &p.Multihop, &p.Enabled, &stdJSON, &largeJSON, &p.CreatedAt)
    if err != nil {
        return Peer{}, false
    }
    json.Unmarshal([]byte(stdJSON), &p.StdCommunities)
    json.Unmarshal([]byte(largeJSON), &p.LargeCommunities)
    return p, true
}

func (s *Store) ListPeers() []Peer {
    rows, err := s.db.Query("SELECT id, name, neighbor_ip, remote_as, local_as, multihop, enabled, std_communities, large_communities, created_at FROM peers ORDER BY name")
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
        json.Unmarshal([]byte(stdJSON), &p.StdCommunities)
        json.Unmarshal([]byte(largeJSON), &p.LargeCommunities)
        out = append(out, p)
    }
    return out
}

func (s *Store) UpdatePeer(p Peer) error {
    stdJSON, _ := json.Marshal(p.StdCommunities)
    largeJSON, _ := json.Marshal(p.LargeCommunities)
    _, err := s.db.Exec("UPDATE peers SET name=?, neighbor_ip=?, remote_as=?, local_as=?, multihop=?, enabled=?, std_communities=?, large_communities=? WHERE id=?",
        p.Name, p.NeighborIP, p.RemoteAS, p.LocalAS, p.Multihop, p.Enabled, string(stdJSON), string(largeJSON), p.ID)
    return err
}

func (s *Store) DeletePeer(id string) error {
    _, err := s.db.Exec("DELETE FROM peers WHERE id = ?", id)
    return err
}
```

**Step 8: Implement events and effective prefix computation**

```go
// Add to store.go

func (s *Store) AddEvent(eventType, message, agentID string) error {
    _, err := s.db.Exec("INSERT INTO events (type, message, agent_id) VALUES (?, ?, ?)", eventType, message, agentID)
    return err
}

func (s *Store) ListEvents(limit int) []Event {
    if limit <= 0 {
        limit = 100
    }
    rows, err := s.db.Query("SELECT id, type, message, agent_id, created_at FROM events ORDER BY created_at DESC LIMIT ?", limit)
    if err != nil {
        return nil
    }
    defer rows.Close()
    var out []Event
    for rows.Next() {
        var e Event
        if err := rows.Scan(&e.ID, &e.Type, &e.Message, &e.AgentID, &e.CreatedAt); err != nil {
            continue
        }
        out = append(out, e)
    }
    return out
}

func (s *Store) EffectivePrefixes() []string {
    // Collect excluded CIDRs
    excluded := map[string]bool{}
    rows, _ := s.db.Query("SELECT cidr FROM excludes")
    if rows != nil {
        for rows.Next() {
            var cidr string
            rows.Scan(&cidr)
            excluded[cidr] = true
        }
        rows.Close()
    }

    // Collect active manual prefixes
    set := map[string]bool{}
    rows, _ = s.db.Query("SELECT cidr FROM prefixes WHERE enabled = 1")
    if rows != nil {
        for rows.Next() {
            var cidr string
            rows.Scan(&cidr)
            if !excluded[cidr] {
                set[cidr] = true
            }
        }
        rows.Close()
    }

    // Collect active source prefixes
    rows, _ = s.db.Query("SELECT sp.cidr FROM source_prefixes sp JOIN sources s ON sp.source_id = s.id WHERE s.enabled = 1")
    if rows != nil {
        for rows.Next() {
            var cidr string
            rows.Scan(&cidr)
            if !excluded[cidr] {
                set[cidr] = true
            }
        }
        rows.Close()
    }

    out := make([]string, 0, len(set))
    for p := range set {
        out = append(out, p)
    }
    sort.Strings(out)
    return out
}

// Helper
func normalizeCIDR(value string) (string, error) {
    trimmed := strings.TrimSpace(value)
    if trimmed == "" {
        return "", fmt.Errorf("empty prefix")
    }
    if p, err := netip.ParsePrefix(trimmed); err == nil {
        return p.Masked().String(), nil
    }
    if addr, err := netip.ParseAddr(trimmed); err == nil {
        bits := 32
        if addr.Is6() {
            bits = 128
        }
        return netip.PrefixFrom(addr, bits).Masked().String(), nil
    }
    return "", fmt.Errorf("invalid IP/CIDR: %s", trimmed)
}
```

**Step 9: Test store compiles**

```bash
go build ./internal/portal/state/
```

**Step 10: Commit**

```bash
git add internal/portal/state/
git commit -m "feat: implement SQLite state store for portal

Replace JSON file state with SQLite database.
Implement CRUD for agents, prefixes, sources, excludes, peers, events.
Add effective prefix computation with exclude support."
```

---

## Phase 4: Portal Service & gRPC Server

### Task 4: Implement portal business logic service

**Files:**
- Create: `internal/portal/service.go`

**Step 1: Implement portal service**

The portal service manages:
- Agent registry
- Source syncing (reuse existing syncer package)
- Prefix/peer/exclude CRUD with event logging
- Reconciliation: compute effective prefixes and push to connected agents
- Config version tracking

```go
// internal/portal/service.go
package portal

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "log"
    "sync"
    "time"

    "rtbhid/internal/portal/state"
    "rtbhid/internal/syncer"
    pb "rtbhid/gen/rtbhid"
)

type ConfigUpdateChan struct {
    ch     chan *pb.ConfigUpdate
    agentID string
}

type Service struct {
    store   *state.Store
    fetcher *syncer.Fetcher
    localASN int

    mu              sync.RWMutex
    configReceivers map[string]*ConfigUpdateChan  // agentID -> channel
    configVersion   int64
    schedulerTick   time.Duration
    runningSyncs    map[string]bool
}

func NewService(store *state.Store, fetcher *syncer.Fetcher, localASN int, tickSec int) *Service {
    if tickSec < 5 {
        tickSec = 5
    }
    return &Service{
        store:           store,
        fetcher:         fetcher,
        localASN:        localASN,
        configReceivers: make(map[string]*ConfigUpdateChan),
        schedulerTick:   time.Duration(tickSec) * time.Second,
        runningSyncs:    make(map[string]bool),
    }
}
```

**Step 2: Implement agent registration and config streaming**

```go
// Add to service.go

func (s *Service) RegisterAgent(agentID, name, birdVersion string) error {
    return s.store.UpsertAgent(state.Agent{
        ID:          agentID,
        Name:        name,
        BirdVersion: birdVersion,
        Status:      "online",
    })
}

func (s *Service) Heartbeat(agentID string, prefixCount, peerCount int, birdStatus string) error {
    now := time.Now().UTC()
    return s.store.UpsertAgent(state.Agent{
        ID:     agentID,
        Status: "online",
        // last_seen updated by UpsertAgent
    })
}

func (s *Service) RegisterConfigReceiver(agentID string) chan *pb.ConfigUpdate {
    ch := make(chan *pb.ConfigUpdate, 16)
    s.mu.Lock()
    s.configReceivers[agentID] = &ConfigUpdateChan{ch: ch, agentID: agentID}
    s.mu.Unlock()

    // Send current config immediately
    go func() {
        update := s.buildConfigUpdate()
        select {
        case ch <- update:
        default:
        }
    }()
    return ch
}

func (s *Service) UnregisterConfigReceiver(agentID string) {
    s.mu.Lock()
    if rc, ok := s.configReceivers[agentID]; ok {
        close(rc.ch)
        delete(s.configReceivers, agentID)
    }
    s.mu.Unlock()
}

func (s *Service) buildConfigUpdate() *pb.ConfigUpdate {
    prefixes := s.store.EffectivePrefixes()
    peers := s.store.ListPeers()

    peerConfigs := make([]*pb.PeerConfig, 0, len(peers))
    for _, p := range peers {
        if !p.Enabled {
            continue
        }
        peerConfigs = append(peerConfigs, &pb.PeerConfig{
            Name:             p.Name,
            NeighborIp:       p.NeighborIP,
            RemoteAs:         uint32(p.RemoteAS),
            LocalAs:          uint32(p.LocalAS),
            Multihop:         uint32(p.Multihop),
            StdCommunities:   p.StdCommunities,
            LargeCommunities: p.LargeCommunities,
        })
    }

    s.mu.Lock()
    s.configVersion++
    ver := s.configVersion
    s.mu.Unlock()

    return &pb.ConfigUpdate{
        Prefixes: prefixes,
        Peers:    peerConfigs,
        Bird:     &pb.BirdConfig{},  // Agent uses local bird config
        Version:  ver,
    }
}

func (s *Service) PushConfigToAll() {
    update := s.buildConfigUpdate()
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, rc := range s.configReceivers {
        select {
        case rc.ch <- update:
        default:
            log.Printf("config channel full for agent %s, skipping", rc.agentID)
        }
    }
}
```

**Step 3: Implement CRUD methods that trigger config push**

```go
// Add to service.go

func (s *Service) AddPrefix(cidr string, enabled bool, note string) (state.Prefix, error) {
    p, err := s.store.AddPrefix(cidr, enabled, note)
    if err != nil {
        return state.Prefix{}, err
    }
    s.store.AddEvent("info", fmt.Sprintf("prefix added: %s", cidr), "")
    s.PushConfigToAll()
    return p, nil
}

func (s *Service) DeletePrefix(id int64) error {
    if err := s.store.DeletePrefix(id); err != nil {
        return err
    }
    s.store.AddEvent("info", "prefix deleted", "")
    s.PushConfigToAll()
    return nil
}

func (s *Service) TogglePrefix(id int64, enabled bool) error {
    if err := s.store.TogglePrefix(id, enabled); err != nil {
        return err
    }
    s.store.AddEvent("info", "prefix toggled", "")
    s.PushConfigToAll()
    return nil
}

func (s *Service) AddExclude(cidr string, note string) (state.Exclude, error) {
    ex, err := s.store.AddExclude(cidr, note)
    if err != nil {
        return state.Exclude{}, err
    }
    s.store.AddEvent("info", fmt.Sprintf("exclude added: %s", cidr), "")
    s.PushConfigToAll()
    return ex, nil
}

func (s *Service) DeleteExclude(id int64) error {
    if err := s.store.DeleteExclude(id); err != nil {
        return err
    }
    s.store.AddEvent("info", "exclude deleted", "")
    s.PushConfigToAll()
    return nil
}

func (s *Service) CreatePeer(p state.Peer) error {
    if p.ID == "" {
        p.ID = newResourceID("peer")
    }
    if err := s.store.CreatePeer(p); err != nil {
        return err
    }
    s.store.AddEvent("info", fmt.Sprintf("peer created: %s", p.Name), "")
    s.PushConfigToAll()
    return nil
}

func (s *Service) UpdatePeer(p state.Peer) error {
    if err := s.store.UpdatePeer(p); err != nil {
        return err
    }
    s.store.AddEvent("info", fmt.Sprintf("peer updated: %s", p.Name), "")
    s.PushConfigToAll()
    return nil
}

func (s *Service) DeletePeer(id string) error {
    if err := s.store.DeletePeer(id); err != nil {
        return err
    }
    s.store.AddEvent("info", "peer deleted", "")
    s.PushConfigToAll()
    return nil
}

func (s *Service) CreateSource(src state.Source) error {
    if src.ID == "" {
        src.ID = newResourceID("src")
    }
    if err := s.store.CreateSource(src); err != nil {
        return err
    }
    s.store.AddEvent("info", fmt.Sprintf("source created: %s", src.Name), "")
    return nil
}

func (s *Service) DeleteSource(id string) error {
    if err := s.store.DeleteSource(id); err != nil {
        return err
    }
    s.store.AddEvent("info", "source deleted", "")
    s.PushConfigToAll()
    return nil
}

// Pass-through methods
func (s *Service) ListAgents() []state.Agent              { return s.store.ListAgents() }
func (s *Service) GetAgent(id string) (state.Agent, bool) { return s.store.GetAgent(id) }
func (s *Service) DeleteAgent(id string) error            { return s.store.DeleteAgent(id) }
func (s *Service) ListPrefixes() []state.Prefix           { return s.store.ListPrefixes() }
func (s *Service) ListSources() []state.Source            { return s.store.ListSources() }
func (s *Service) ListExcludes() []state.Exclude          { return s.store.ListExcludes() }
func (s *Service) ListPeers() []state.Peer                { return s.store.ListPeers() }
func (s *Service) ListEvents(limit int) []state.Event     { return s.store.ListEvents(limit) }

func newResourceID(kind string) string {
    var b [4]byte
    _, _ = rand.Read(b[:])
    return fmt.Sprintf("%s-%d-%s", kind, time.Now().Unix(), hex.EncodeToString(b[:]))
}
```

**Step 4: Implement scheduler loop for source syncing**

```go
// Add to service.go

func (s *Service) Start(ctx context.Context) {
    go s.schedulerLoop(ctx)
}

func (s *Service) schedulerLoop(ctx context.Context) {
    ticker := time.NewTicker(s.schedulerTick)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            sources := s.store.ListSources()
            for _, src := range sources {
                if !src.Enabled {
                    continue
                }
                interval := time.Duration(src.Interval) * time.Second
                if src.Interval < 10 {
                    interval = 300 * time.Second
                }
                if src.LastSync == nil || time.Since(*src.LastSync) >= interval {
                    s.syncSourceAsync(ctx, src)
                }
            }
        }
    }
}

func (s *Service) syncSourceAsync(ctx context.Context, src state.Source) {
    s.mu.Lock()
    if s.runningSyncs[src.ID] {
        s.mu.Unlock()
        return
    }
    s.runningSyncs[src.ID] = true
    s.mu.Unlock()

    go func() {
        defer func() {
            s.mu.Lock()
            delete(s.runningSyncs, src.ID)
            s.mu.Unlock()
        }()
        prefixes, err := s.fetcher.Fetch(ctx, src.URL, 30)
        if err != nil {
            s.store.AddEvent("error", fmt.Sprintf("source sync failed: %s: %v", src.Name, err), "")
            return
        }
        if err := s.store.SetSourcePrefixes(src.ID, prefixes); err != nil {
            s.store.AddEvent("error", fmt.Sprintf("save source prefixes failed: %v", err), "")
            return
        }
        s.store.AddEvent("info", fmt.Sprintf("source sync success: %s: %d prefixes", src.Name, len(prefixes)), "")
        s.PushConfigToAll()
    }()
}

func (s *Service) TriggerSync(sourceID string) {
    src, ok := s.store.GetSource(sourceID)
    if !ok {
        return
    }
    s.syncSourceAsync(context.Background(), src)
}

// Store delegates
func (s *Service) UpdateSource(src state.Source) error {
    if err := s.store.UpdateSource(src); err != nil {
        return err
    }
    s.store.AddEvent("info", fmt.Sprintf("source updated: %s", src.Name), "")
    return nil
}
```

**Step 5: Verify service compiles**

```bash
go build ./internal/portal/
```

**Step 6: Commit**

```bash
git add internal/portal/service.go
git commit -m "feat: implement portal service with config push

Portal service manages agents, prefixes, sources, excludes, peers.
Pushes config updates to all connected agents on any change.
Includes source sync scheduler."
```

### Task 5: Implement gRPC server

**Files:**
- Create: `internal/portal/grpcapi/server.go`

**Step 1: Implement gRPC server**

```go
// internal/portal/grpcapi/server.go
package grpcapi

import (
    "log"
    "net"
    "time"

    "google.golang.org/grpc"
    pb "rtbhid/gen/rtbhid"
    "rtbhid/internal/portal"
)

type Server struct {
    pb.UnimplementedPortalServer
    svc *portal.Service
    addr string
}

func NewServer(svc *portal.Service, addr string) *Server {
    return &Server{svc: svc, addr: addr}
}

func (s *Server) ListenAndServe() error {
    lis, err := net.Listen("tcp", s.addr)
    if err != nil {
        return err
    }
    gs := grpc.NewServer()
    pb.RegisterPortalServer(gs, s)
    log.Printf("gRPC server listening on %s", s.addr)
    return gs.Serve(lis)
}

func (s *Server) Register(req *pb.RegisterRequest, stream pb.Portal_RegisterServer) error {
    log.Printf("agent registered: %s (%s) bird=%s", req.AgentId, req.Name, req.BirdVersion)
    if err := s.svc.RegisterAgent(req.AgentId, req.Name, req.BirdVersion); err != nil {
        log.Printf("register agent error: %v", err)
        return err
    }

    // This is actually the StreamConfig handler - agents call Register first, then StreamConfig
    return nil
}

func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
    birdStatus := req.BirdStatus
    if birdStatus == "" {
        birdStatus = "unknown"
    }
    if err := s.svc.Heartbeat(req.AgentId, int(req.PrefixCount), int(req.PeerCount), birdStatus); err != nil {
        log.Printf("heartbeat error for %s: %v", req.AgentId, err)
    }
    return &pb.HeartbeatResponse{Success: true}, nil
}

func (s *Server) StreamConfig(req *pb.ConfigRequest, stream pb.Portal_StreamConfigServer) error {
    ch := s.svc.RegisterConfigReceiver(req.AgentId)
    defer s.svc.UnregisterConfigReceiver(req.AgentId)

    log.Printf("config stream opened for agent %s", req.AgentId)

    for {
        select {
        case <-stream.Context().Done():
            log.Printf("config stream closed for agent %s", req.AgentId)
            return nil
        case update, ok := <-ch:
            if !ok {
                return nil
            }
            if err := stream.Send(update); err != nil {
                log.Printf("send config to %s failed: %v", req.AgentId, err)
                return err
            }
        }
    }
}
```

Wait — looking at the proto again, `Register` returns `RegisterResponse` (not a stream), and `StreamConfig` returns a stream. Let me fix the implementation:

**Step 1 (corrected): Implement gRPC server**

```go
// internal/portal/grpcapi/server.go
package grpcapi

import (
    "context"
    "log"
    "net"

    "google.golang.org/grpc"
    pb "rtbhid/gen/rtbhid"
    "rtbhid/internal/portal"
)

type Server struct {
    pb.UnimplementedPortalServer
    svc  *portal.Service
    addr string
}

func NewServer(svc *portal.Service, addr string) *Server {
    return &Server{svc: svc, addr: addr}
}

func (s *Server) ListenAndServe() error {
    lis, err := net.Listen("tcp", s.addr)
    if err != nil {
        return err
    }
    gs := grpc.NewServer()
    pb.RegisterPortalServer(gs, s)
    log.Printf("gRPC server listening on %s", s.addr)
    return gs.Serve(lis)
}

func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
    log.Printf("agent registering: %s (%s) bird=%s", req.AgentId, req.Name, req.BirdVersion)
    if err := s.svc.RegisterAgent(req.AgentId, req.Name, req.BirdVersion); err != nil {
        return &pb.RegisterResponse{Success: false, Message: err.Error()}, nil
    }
    return &pb.RegisterResponse{Success: true, HeartbeatInterval: 30}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
    birdStatus := req.BirdStatus
    if birdStatus == "" {
        birdStatus = "unknown"
    }
    if err := s.svc.Heartbeat(req.AgentId, int(req.PrefixCount), int(req.PeerCount), birdStatus); err != nil {
        log.Printf("heartbeat error for %s: %v", req.AgentId, err)
    }
    return &pb.HeartbeatResponse{Success: true}, nil
}

func (s *Server) StreamConfig(req *pb.ConfigRequest, stream pb.Portal_StreamConfigServer) error {
    ch := s.svc.RegisterConfigReceiver(req.AgentId)
    defer s.svc.UnregisterConfigReceiver(req.AgentId)

    log.Printf("config stream opened for agent %s", req.AgentId)

    for {
        select {
        case <-stream.Context().Done():
            log.Printf("config stream closed for agent %s", req.AgentId)
            return nil
        case update, ok := <-ch:
            if !ok {
                return nil
            }
            if err := stream.Send(update); err != nil {
                log.Printf("send config to %s failed: %v", req.AgentId, err)
                return err
            }
        }
    }
}
```

**Step 2: Verify gRPC server compiles**

```bash
go build ./internal/portal/grpcapi/
```

**Step 3: Commit**

```bash
git add internal/portal/grpcapi/
git commit -m "feat: implement gRPC server for agent communication

gRPC server handles Register, Heartbeat, and StreamConfig.
Agents receive config updates via server-side streaming."
```

---

## Phase 5: Portal REST API & Main Binary

### Task 6: Implement portal REST API

**Files:**
- Create: `internal/portal/httpapi/server.go`

**Step 1: Implement REST API server**

```go
// internal/portal/httpapi/server.go
package httpapi

import (
    "embed"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "io/fs"
    "net/http"
    "strconv"
    "strings"

    "rtbhid/internal/portal"
    "rtbhid/internal/portal/state"
)

//go:embed all:web/dist
var webAssets embed.FS

type Server struct {
    svc *portal.Service
}

func NewServer(svc *portal.Service) *Server {
    return &Server{svc: svc}
}

func (s *Server) Handler() http.Handler {
    mux := http.NewServeMux()

    // API routes
    mux.HandleFunc("/api/v1/health", s.handleHealth)
    mux.HandleFunc("/api/v1/status", s.handleStatus)
    mux.HandleFunc("/api/v1/agents", s.handleAgents)
    mux.HandleFunc("/api/v1/agents/", s.handleAgentByID)
    mux.HandleFunc("/api/v1/prefixes", s.handlePrefixes)
    mux.HandleFunc("/api/v1/prefixes/", s.handlePrefixByID)
    mux.HandleFunc("/api/v1/sources", s.handleSources)
    mux.HandleFunc("/api/v1/sources/", s.handleSourceByID)
    mux.HandleFunc("/api/v1/excludes", s.handleExcludes)
    mux.HandleFunc("/api/v1/excludes/", s.handleExcludeByID)
    mux.HandleFunc("/api/v1/peers", s.handlePeers)
    mux.HandleFunc("/api/v1/peers/", s.handlePeerByID)
    mux.HandleFunc("/api/v1/events", s.handleEvents)

    // Serve React SPA
    subFS, _ := fs.Sub(webAssets, "web/dist")
    fileServer := http.FileServer(http.FS(subFS))
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Try serving static file first
        path := r.URL.Path
        if path == "/" {
            path = "/index.html"
        }
        // Check if file exists
        if f, err := subFS.Open(strings.TrimPrefix(path, "/")); err == nil {
            f.Close()
            fileServer.ServeHTTP(w, r)
            return
        }
        // SPA fallback: serve index.html for all routes
        r.URL.Path = "/"
        fileServer.ServeHTTP(w, r)
    })

    return withCORS(withLogging(mux))
}
```

**Step 2: Implement API handlers**

```go
// Add to server.go

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        methodNotAllowed(w)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        methodNotAllowed(w)
        return
    }
    agents := s.svc.ListAgents()
    online := 0
    for _, a := range agents {
        if a.Status == "online" {
            online++
        }
    }
    writeJSON(w, http.StatusOK, map[string]any{
        "total_agents":      len(agents),
        "online_agents":     online,
        "total_prefixes":    len(s.svc.ListPrefixes()),
        "total_peers":       len(s.svc.ListPeers()),
        "total_excludes":    len(s.svc.ListExcludes()),
        "total_sources":     len(s.svc.ListSources()),
        "effective_prefixes": len(s.svc.ListPrefixes()), // simplified; full calc from store
    })
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        methodNotAllowed(w)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListAgents()})
}

func (s *Server) handleAgentByID(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/v1/agents/")
    if id == "" {
        http.NotFound(w, r)
        return
    }
    switch r.Method {
    case http.MethodGet:
        agent, ok := s.svc.GetAgent(id)
        if !ok {
            writeError(w, http.StatusNotFound, errors.New("agent not found"))
            return
        }
        writeJSON(w, http.StatusOK, agent)
    case http.MethodDelete:
        if err := s.svc.DeleteAgent(id); err != nil {
            writeError(w, http.StatusInternalServerError, err)
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handlePrefixes(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListPrefixes()})
    case http.MethodPost:
        var req struct {
            CIDR    string `json:"cidr"`
            Enabled *bool  `json:"enabled"`
            Note    string `json:"note"`
        }
        if err := decodeJSON(r, &req); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        enabled := true
        if req.Enabled != nil {
            enabled = *req.Enabled
        }
        p, err := s.svc.AddPrefix(req.CIDR, enabled, req.Note)
        if err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusCreated, p)
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handlePrefixByID(w http.ResponseWriter, r *http.Request) {
    idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/prefixes/")
    if idStr == "" {
        http.NotFound(w, r)
        return
    }

    // Handle toggle
    if strings.HasSuffix(idStr, "/toggle") {
        idStr = strings.TrimSuffix(idStr, "/toggle")
        if r.Method != http.MethodPut {
            methodNotAllowed(w)
            return
        }
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            writeError(w, http.StatusBadRequest, errors.New("invalid prefix id"))
            return
        }
        var req struct {
            Enabled bool `json:"enabled"`
        }
        if err := decodeJSON(r, &req); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        if err := s.svc.TogglePrefix(id, req.Enabled); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
        return
    }

    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        writeError(w, http.StatusBadRequest, errors.New("invalid prefix id"))
        return
    }

    switch r.Method {
    case http.MethodDelete:
        if err := s.svc.DeletePrefix(id); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListSources()})
    case http.MethodPost:
        var req struct {
            Name     string `json:"name"`
            URL      string `json:"url"`
            Interval int    `json:"interval"`
            Enabled  bool   `json:"enabled"`
        }
        if err := decodeJSON(r, &req); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        if req.Interval <= 0 {
            req.Interval = 3600
        }
        src := state.Source{
            ID:       fmt.Sprintf("src-%d", timeNow()),
            Name:     req.Name,
            URL:      req.URL,
            Interval: req.Interval,
            Enabled:  req.Enabled,
        }
        if err := s.svc.CreateSource(src); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusCreated, src)
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handleSourceByID(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/sources/"), "/")
    if len(parts) == 0 || parts[0] == "" {
        http.NotFound(w, r)
        return
    }
    id := parts[0]

    if len(parts) == 2 && parts[1] == "sync" {
        if r.Method != http.MethodPost {
            methodNotAllowed(w)
            return
        }
        s.svc.TriggerSync(id)
        writeJSON(w, http.StatusAccepted, map[string]any{"scheduled": true})
        return
    }

    switch r.Method {
    case http.MethodDelete:
        if err := s.svc.DeleteSource(id); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handleExcludes(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListExcludes()})
    case http.MethodPost:
        var req struct {
            CIDR string `json:"cidr"`
            Note string `json:"note"`
        }
        if err := decodeJSON(r, &req); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        ex, err := s.svc.AddExclude(req.CIDR, req.Note)
        if err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusCreated, ex)
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handleExcludeByID(w http.ResponseWriter, r *http.Request) {
    idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/excludes/")
    if idStr == "" {
        http.NotFound(w, r)
        return
    }
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        writeError(w, http.StatusBadRequest, errors.New("invalid exclude id"))
        return
    }
    switch r.Method {
    case http.MethodDelete:
        if err := s.svc.DeleteExclude(id); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        writeJSON(w, http.StatusOK, map[string]any{"items": s.svc.ListPeers()})
    case http.MethodPost:
        var req state.Peer
        if err := decodeJSON(r, &req); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        if err := s.svc.CreatePeer(req); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusCreated, req)
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handlePeerByID(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/v1/peers/")
    if id == "" {
        http.NotFound(w, r)
        return
    }
    switch r.Method {
    case http.MethodDelete:
        if err := s.svc.DeletePeer(id); err != nil {
            writeError(w, http.StatusBadRequest, err)
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
    default:
        methodNotAllowed(w)
    }
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        methodNotAllowed(w)
        return
    }
    limit := 100
    if raw := r.URL.Query().Get("limit"); raw != "" {
        if n, err := strconv.Atoi(raw); err == nil && n > 0 {
            limit = n
        }
    }
    writeJSON(w, http.StatusOK, map[string]any{"events": s.svc.ListEvents(limit)})
}
```

**Step 3: Add shared helpers**

```go
// Add to server.go

func decodeJSON(r *http.Request, out any) error {
    defer r.Body.Close()
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    if err := dec.Decode(out); err != nil {
        return fmt.Errorf("invalid json: %w", err)
    }
    if err := dec.Decode(&struct{}{}); err != io.EOF {
        return errors.New("only one JSON object is allowed")
    }
    return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
    writeJSON(w, status, map[string]any{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
    writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
}

func withCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func withLogging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fmt.Printf("%s %s\n", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

func timeNow() int64 {
    return time.Now().Unix()
}
```

Need to add `time` import.

**Step 4: Note on embed path**

The `//go:embed all:web/dist` in `internal/portal/httpapi/server.go` refers to `web/dist` relative to the package directory. We need the React build output to land at `internal/portal/httpapi/web/dist` OR we adjust the embed path. Since we want `web/` at the project root, the embed should be in a file at project root level. Better approach: move the embed to `cmd/portal/main.go` or create a dedicated assets file.

Actually, Go embed paths are relative to the source file's directory. So `internal/portal/httpapi/server.go` with `//go:embed all:web/dist` would look for `internal/portal/httpapi/web/dist`. We should instead:

Option A: Embed from `cmd/portal/` where we have access to the project root's `web/dist`.
Option B: Create an assets package at project root level.

Let's use Option A — the embed happens in `cmd/portal/main.go` and gets passed to the HTTP server.

**Step 5: Verify REST API compiles**

```bash
go build ./internal/portal/httpapi/
```

**Step 6: Commit**

```bash
git add internal/portal/httpapi/
git commit -m "feat: implement portal REST API

REST API for React dashboard with CRUD for agents, prefixes,
sources, excludes, peers, and events."
```

### Task 7: Wire up portal main binary

**Files:**
- Create: `cmd/portal/main.go`
- Modify: `internal/portal/httpapi/server.go` (move embed to main)

**Step 1: Create portal main.go**

```go
// cmd/portal/main.go
package main

import (
    "context"
    "embed"
    "fmt"
    "io/fs"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "rtbhid/internal/portal"
    "rtbhid/internal/portal/grpcapi"
    "rtbhid/internal/portal/httpapi"
    "rtbhid/internal/portal/state"
    "rtbhid/internal/syncer"
)

//go:embed all:web/dist
var webAssets embed.FS

func main() {
    listenAddr := getEnv("RTBH_LISTEN_ADDR", ":8088")
    grpcAddr := getEnv("RTBH_GRPC_ADDR", ":50051")
    dbPath := getEnv("RTBH_DB_PATH", "./data/portal.db")
    localASN := getEnvInt("RTBH_LOCAL_ASN", 65500)
    tickSec := getEnvInt("RTBH_SCHEDULER_TICK_SECONDS", 15)

    store, err := state.New(dbPath)
    if err != nil {
        log.Fatalf("init state: %v", err)
    }
    defer store.Close()

    fetcher := syncer.NewFetcher("rtbhid-portal/1.0")
    svc := portal.NewService(store, fetcher, localASN, tickSec)

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    svc.Start(ctx)

    // gRPC server
    grpcServer := grpcapi.NewServer(svc, grpcAddr)
    go func() {
        if err := grpcServer.ListenAndServe(); err != nil {
            log.Fatalf("grpc server: %v", err)
        }
    }()

    // HTTP server with embedded React assets
    httpServer := httpapi.NewServer(svc, webAssets)
    srv := &http.Server{
        Addr:              listenAddr,
        Handler:           httpServer.Handler(),
        ReadHeaderTimeout: 10 * time.Second,
    }

    go func() {
        <-ctx.Done()
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer shutdownCancel()
        _ = srv.Shutdown(shutdownCtx)
    }()

    fmt.Printf("portal HTTP listening on %s\n", listenAddr)
    fmt.Printf("portal gRPC listening on %s\n", grpcAddr)
    fmt.Printf("database: %s\n", dbPath)
    fmt.Printf("local ASN: %d\n", localASN)

    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        fmt.Fprintf(os.Stderr, "server error: %v\n", err)
        os.Exit(1)
    }
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func getEnvInt(key string, fallback int) int {
    v := os.Getenv(key)
    if v == "" {
        return fallback
    }
    n := 0
    fmt.Sscanf(v, "%d", &n)
    if n <= 0 {
        return fallback
    }
    return n
}
```

**Step 2: Update httpapi server to accept embed.FS**

```go
// internal/portal/httpapi/server.go - update constructor
type Server struct {
    svc  *portal.Service
    webAssets embed.FS
}

func NewServer(svc *portal.Service, webAssets embed.FS) *Server {
    return &Server{svc: svc, webAssets: webAssets}
}

// Update Handler() to use s.webAssets instead of the package-level embed
```

Remove the `//go:embed` directive from `internal/portal/httpapi/server.go` and use `s.webAssets` in the Handler method.

**Step 3: Create placeholder web/dist for initial build**

```bash
mkdir -p cmd/portal/web/dist
echo '<html><body><h1>RTBH Portal</h1><p>React app not built yet.</p></body></html>' > cmd/portal/web/dist/index.html
```

**Step 4: Verify portal builds**

```bash
go build ./cmd/portal/
```

**Step 5: Commit**

```bash
git add cmd/portal/ internal/portal/httpapi/server.go
git commit -m "feat: wire up portal main binary

Portal binary serves HTTP (React dashboard) and gRPC (agent comm).
Config via env vars: RTBH_LISTEN_ADDR, RTBH_GRPC_ADDR, RTBH_DB_PATH."
```

---

## Phase 6: Agent gRPC Client

### Task 8: Implement agent gRPC client

**Files:**
- Create: `internal/agent/grpcclient/client.go`

**Step 1: Implement gRPC client**

```go
// internal/agent/grpcclient/client.go
package grpcclient

import (
    "context"
    "io"
    "log"
    "os"
    "sync"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    pb "rtbhid/gen/rtbhid"
    "rtbhid/internal/agent/bird"
)

type Config struct {
    PortalURL  string
    AgentID    string
    AgentName  string
    BirdVersion string
    AgentVersion string

    BirdOutputPath   string
    BirdPeersOutput  string
    BirdReloadEnabled bool
    BirdCommand      []string
    LocalASN         int
    ExportStdComms   string
    ExportLargeComms string
}

type Client struct {
    cfg        Config
    conn       *grpc.ClientConn
    portal     pb.PortalClient
    birdMgr    *bird.Manager
    mu         sync.Mutex
    lastConfig *pb.ConfigUpdate
}

func New(cfg Config) *Client {
    exportPolicy, _ := bird.ParseExportPolicy(cfg.ExportStdComms, cfg.ExportLargeComms)
    birdMgr := bird.NewManager(
        cfg.BirdOutputPath,
        cfg.BirdPeersOutput,
        cfg.BirdReloadEnabled,
        cfg.BirdCommand,
        exportPolicy,
    )
    return &Client{cfg: cfg, birdMgr: birdMgr}
}
```

**Step 2: Implement Run method (main agent loop)**

```go
// Add to client.go

func (c *Client) Run(ctx context.Context) error {
    // Load or generate agent ID
    agentID, err := c.loadOrGenerateID()
    if err != nil {
        return err
    }
    c.cfg.AgentID = agentID

    // Connect to portal
    conn, err := grpc.NewClient(c.cfg.PortalURL,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        return err
    }
    c.conn = conn
    c.portal = pb.NewPortalClient(conn)

    // Register
    regResp, err := c.portal.Register(ctx, &pb.RegisterRequest{
        AgentId:     c.cfg.AgentID,
        Name:        c.cfg.AgentName,
        BirdVersion: c.cfg.BirdVersion,
        Version:     c.cfg.AgentVersion,
    })
    if err != nil {
        return err
    }
    if !regResp.Success {
        return fmt.Errorf("registration failed: %s", regResp.Message)
    }
    log.Printf("registered with portal, heartbeat interval: %ds", regResp.HeartbeatInterval)

    heartbeatInterval := time.Duration(regResp.HeartbeatInterval) * time.Second

    // Start heartbeat
    go c.heartbeatLoop(ctx, heartbeatInterval)

    // Start config stream
    return c.configStreamLoop(ctx)
}

func (c *Client) heartbeatLoop(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.mu.Lock()
            prefixCount := 0
            peerCount := 0
            if c.lastConfig != nil {
                prefixCount = len(c.lastConfig.Prefixes)
                peerCount = len(c.lastConfig.Peers)
            }
            c.mu.Unlock()

            _, err := c.portal.Heartbeat(ctx, &pb.HeartbeatRequest{
                AgentId:     c.cfg.AgentID,
                PrefixCount: int32(prefixCount),
                PeerCount:   int32(peerCount),
                BirdStatus:  "running",
            })
            if err != nil {
                log.Printf("heartbeat error: %v", err)
            }
        }
    }
}

func (c *Client) configStreamLoop(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        stream, err := c.portal.StreamConfig(ctx, &pb.ConfigRequest{AgentId: c.cfg.AgentID})
        if err != nil {
            log.Printf("stream config error: %v, retrying in 5s", err)
            time.Sleep(5 * time.Second)
            continue
        }

        for {
            update, err := stream.Recv()
            if err == io.EOF {
                log.Println("config stream ended, reconnecting")
                break
            }
            if err != nil {
                log.Printf("config stream error: %v, reconnecting in 5s", err)
                time.Sleep(5 * time.Second)
                break
            }
            c.applyConfig(update)
        }
    }
}
```

**Step 3: Implement config application**

```go
// Add to client.go

func (c *Client) applyConfig(update *pb.ConfigUpdate) {
    c.mu.Lock()
    c.lastConfig = update
    c.mu.Unlock()

    log.Printf("applying config version %d: %d prefixes, %d peers", update.Version, len(update.Prefixes), len(update.Peers))

    // Convert peers
    peers := make([]bird.Peer, 0, len(update.Peers))
    for _, p := range update.Peers {
        peers = append(peers, bird.Peer{
            ID:          p.Name,
            Name:        p.Name,
            Neighbor:    p.NeighborIp,
            NeighborASN: int(p.RemoteAs),
            Enabled:     true,
            Multihop:    int(p.Multihop),
        })
    }

    result, err := c.birdMgr.Apply(update.Prefixes, peers, c.cfg.LocalASN)
    if err != nil {
        log.Printf("bird apply error: %v", err)
        return
    }
    log.Printf("bird config applied: %d routes, %d peers, reload=%v", result.RouteCount, result.PeerCount, result.ReloadRan)
}
```

**Step 4: Implement agent ID persistence**

```go
// Add to client.go

func (c *Client) loadOrGenerateID() (string, error) {
    idPath := "./data/agent_id"
    if data, err := os.ReadFile(idPath); err == nil {
        id := string(data)
        if len(id) > 0 {
            return id, nil
        }
    }

    // Generate new UUID-like ID
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    id := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])

    os.MkdirAll("./data", 0o755)
    if err := os.WriteFile(idPath, []byte(id), 0o644); err != nil {
        return "", err
    }
    log.Printf("generated new agent ID: %s", id)
    return id, nil
}
```

Add `crypto/rand` and `fmt` imports.

**Step 5: Verify client compiles**

```bash
go build ./internal/agent/grpcclient/
```

**Step 6: Commit**

```bash
git add internal/agent/grpcclient/
git commit -m "feat: implement agent gRPC client

Agent connects to portal, registers, receives config via streaming,
applies Bird config locally, and sends heartbeats."
```

### Task 9: Update agent main binary

**Files:**
- Modify: `cmd/agent/main.go`

**Step 1: Rewrite agent main.go to support both modes**

```go
// cmd/agent/main.go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    "rtbhid/internal/agent/bird"
    "rtbhid/internal/agent/config"
    "rtbhid/internal/agent/grpcclient"
    "rtbhid/internal/app"
    "rtbhid/internal/httpapi"
    "rtbhid/internal/state"
    "rtbhid/internal/syncer"
)

func main() {
    mode := getEnv("RTBH_MODE", "standalone")

    if mode == "agent" {
        runAgent()
    } else {
        runStandalone()
    }
}

func runAgent() {
    portalURL := getEnv("RTBH_PORTAL_URL", "localhost:50051")
    agentName := getEnv("RTBH_AGENT_NAME", hostname())
    birdVersion := getEnv("RTBH_BIRD_VERSION", "2")
    birdOutput := getEnv("RTBH_BIRD_OUTPUT", "./data/rtbh_auto.conf")
    birdPeersOutput := getEnv("RTBH_BIRD_PEERS_OUTPUT", "./data/rtbh_peers_auto.conf")
    birdReload := getEnvBool("RTBH_BIRD_RELOAD", false)
    birdCmd := strings.Fields(getEnv("RTBH_BIRD_COMMAND", "birdc configure"))
    localASN := getEnvInt("RTBH_LOCAL_ASN", 65500)
    exportStd := getEnv("RTBH_EXPORT_STD_COMMUNITIES", "")
    exportLarge := getEnv("RTBH_EXPORT_LARGE_COMMUNITIES", "")

    client := grpcclient.New(grpcclient.Config{
        PortalURL:        portalURL,
        AgentName:        agentName,
        BirdVersion:      birdVersion,
        AgentVersion:     "2.0.0",
        BirdOutputPath:   birdOutput,
        BirdPeersOutput:  birdPeersOutput,
        BirdReloadEnabled: birdReload,
        BirdCommand:      birdCmd,
        LocalASN:         localASN,
        ExportStdComms:   exportStd,
        ExportLargeComms: exportLarge,
    })

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    fmt.Printf("rtbhd agent mode\n")
    fmt.Printf("portal: %s\n", portalURL)
    fmt.Printf("agent name: %s\n", agentName)
    fmt.Printf("bird version: %s\n", birdVersion)
    fmt.Printf("bird routes output: %s\n", birdOutput)
    fmt.Printf("bird peers output: %s\n", birdPeersOutput)
    fmt.Printf("bird reload: %v\n", birdReload)
    fmt.Printf("local ASN: %d\n", localASN)

    if err := client.Run(ctx); err != nil {
        log.Fatalf("agent error: %v", err)
    }
}

func runStandalone() {
    // Existing standalone mode - unchanged
    cfg := config.Load()
    store, err := state.New(cfg.StatePath)
    if err != nil {
        log.Fatalf("init state: %v", err)
    }
    exportPolicy, err := bird.ParseExportPolicy(cfg.ExportStdComms, cfg.ExportLargeComms)
    if err != nil {
        log.Fatalf("parse export communities: %v", err)
    }
    fetcher := syncer.NewFetcher(cfg.SyncUserAgent)
    birdManager := bird.NewManager(cfg.BirdOutputPath, cfg.BirdPeersOutput, cfg.BirdReloadEnabled, cfg.BirdCommand, exportPolicy)
    service := app.NewService(store, fetcher, birdManager, cfg.SchedulerTickSec, cfg.LocalASN)
    httpServer, err := httpapi.NewServer(service, httpapi.AuthConfig{
        Enabled:       cfg.AuthEnabled,
        Username:      cfg.AuthUsername,
        Password:      cfg.AuthPassword,
        SessionSecret: cfg.AuthSessionSecret,
        SessionTTL:    time.Duration(cfg.AuthSessionTTLHrs) * time.Hour,
        CookieSecure:  cfg.AuthCookieSecure,
    })
    if err != nil {
        log.Fatalf("init http auth: %v", err)
    }

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    service.Start(ctx)

    srv := &http.Server{
        Addr:              cfg.ListenAddr,
        Handler:           httpServer.Handler(),
        ReadHeaderTimeout: 10 * time.Second,
    }

    go func() {
        <-ctx.Done()
        shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer shutdownCancel()
        _ = srv.Shutdown(shutdownCtx)
    }()

    fmt.Printf("rtbhd standalone on %s\n", cfg.ListenAddr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        fmt.Fprintf(os.Stderr, "server error: %v\n", err)
        os.Exit(1)
    }
}

func hostname() string {
    h, _ := os.Hostname()
    if h == "" {
        return "agent"
    }
    return h
}

// Helper functions (same as config package)
func getEnv(key, fallback string) string { /* same as before */ }
func getEnvInt(key string, fallback int) int { /* same as before */ }
func getEnvBool(key string, fallback bool) bool { /* same as before */ }
```

**Step 2: Verify agent builds**

```bash
go build ./cmd/agent/
```

**Step 3: Commit**

```bash
git add cmd/agent/main.go
git commit -m "feat: add agent mode to rtbhd binary

RTBH_MODE=agent connects to portal via gRPC.
RTBH_MODE=standandalone uses existing behavior (default)."
```

---

## Phase 7: React Frontend

### Task 10: Scaffold React project

**Files:**
- Create: `web/` directory with Vite + React + TypeScript + Tailwind + shadcn/ui

**Step 1: Scaffold with Vite**

```bash
cd /Users/billyriantono/Works/rtbhid
npm create vite@latest web -- --template react-ts
cd web
```

**Step 2: Install dependencies**

```bash
npm install
npm install tailwindcss @tailwindcss/vite lucide-react
npm install -D @types/node
npx shadcn@latest init -t vite
```

**Step 3: Add shadcn/ui components**

```bash
npx shadcn@latest add button card table badge input dialog label select separator sheet sidebar tabs tooltip dropdown-menu
```

**Step 4: Configure Vite proxy for API**

```typescript
// web/vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8088',
    },
  },
})
```

**Step 5: Build React app and copy to portal embed location**

```bash
# After building:
mkdir -p ../cmd/portal/web/dist
cp -r dist/* ../cmd/portal/web/dist/
```

Add build script to root package.json or Makefile.

**Step 6: Commit**

```bash
git add web/
git commit -m "feat: scaffold React frontend with Vite + Tailwind + shadcn/ui

React 19 + Vite 8 + Tailwind CSS 4.2 + shadcn/ui 4.0 + Lucide React 0.577."
```

### Task 11: Build React pages

**Files:**
- Create: `web/src/pages/Dashboard.tsx`
- Create: `web/src/pages/Agents.tsx`
- Create: `web/src/pages/Prefixes.tsx`
- Create: `web/src/pages/Sources.tsx`
- Create: `web/src/pages/Excludes.tsx`
- Create: `web/src/pages/Peers.tsx`
- Create: `web/src/pages/Events.tsx`
- Create: `web/src/lib/api.ts`
- Create: `web/src/App.tsx` (router setup)
- Create: `web/src/components/layout/Sidebar.tsx`

Due to the large scope, I'll outline each page's responsibility and the API client. Each page follows the same pattern: fetch data, display in shadcn Table, add/delete actions.

**Step 1: API client**

```typescript
// web/src/lib/api.ts
const BASE = '/api/v1'

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(`${BASE}${url}`)
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

async function postJSON<T>(url: string, data: any): Promise<T> {
  const res = await fetch(`${BASE}${url}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

async function deleteJSON<T>(url: string): Promise<T> {
  const res = await fetch(`${BASE}${url}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export const api = {
  getStatus: () => fetchJSON('/status'),
  getAgents: () => fetchJSON<{items: any[]}>('/agents'),
  deleteAgent: (id: string) => deleteJSON(`/agents/${id}`),
  getPrefixes: () => fetchJSON<{items: any[]}>('/prefixes'),
  addPrefix: (data: any) => postJSON('/prefixes', data),
  deletePrefix: (id: number) => deleteJSON(`/prefixes/${id}`),
  togglePrefix: (id: number, enabled: boolean) =>
    fetch(`${BASE}/prefixes/${id}/toggle`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    }),
  getSources: () => fetchJSON<{items: any[]}>('/sources'),
  addSource: (data: any) => postJSON('/sources', data),
  deleteSource: (id: string) => deleteJSON(`/sources/${id}`),
  syncSource: (id: string) => fetch(`${BASE}/sources/${id}/sync`, { method: 'POST' }),
  getExcludes: () => fetchJSON<{items: any[]}>('/excludes'),
  addExclude: (data: any) => postJSON('/excludes', data),
  deleteExclude: (id: number) => deleteJSON(`/excludes/${id}`),
  getPeers: () => fetchJSON<{items: any[]}>('/peers'),
  addPeer: (data: any) => postJSON('/peers', data),
  deletePeer: (id: string) => deleteJSON(`/peers/${id}`),
  getEvents: (limit = 100) => fetchJSON<{events: any[]}>(`/events?limit=${limit}`),
}
```

**Step 2: Sidebar layout**

```typescript
// web/src/components/layout/Sidebar.tsx
import { LayoutDashboard, Server, List, Globe, Shield, Network, ScrollText } from 'lucide-react'
import { NavLink } from 'react-router'

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/agents', icon: Server, label: 'Agents' },
  { to: '/prefixes', icon: List, label: 'Prefixes' },
  { to: '/sources', icon: Globe, label: 'Sources' },
  { to: '/excludes', icon: Shield, label: 'Excludes' },
  { to: '/peers', icon: Network, label: 'Peers' },
  { to: '/events', icon: ScrollText, label: 'Events' },
]

export function Sidebar() {
  return (
    <aside className="w-64 border-r bg-muted/40 p-4">
      <h1 className="text-lg font-bold mb-6">RTBH Portal</h1>
      <nav className="space-y-1">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-3 px-3 py-2 rounded-md text-sm ${
                isActive ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'
              }`
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
```

**Step 3: App with router**

```typescript
// web/src/App.tsx
import { BrowserRouter, Routes, Route } from 'react-router'
import { Sidebar } from './components/layout/Sidebar'
import Dashboard from './pages/Dashboard'
import Agents from './pages/Agents'
import Prefixes from './pages/Prefixes'
import Sources from './pages/Sources'
import Excludes from './pages/Excludes'
import Peers from './pages/Peers'
import Events from './pages/Events'

export default function App() {
  return (
    <BrowserRouter>
      <div className="flex h-screen">
        <Sidebar />
        <main className="flex-1 overflow-auto p-6">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/agents" element={<Agents />} />
            <Route path="/prefixes" element={<Prefixes />} />
            <Route path="/sources" element={<Sources />} />
            <Route path="/excludes" element={<Excludes />} />
            <Route path="/peers" element={<Peers />} />
            <Route path="/events" element={<Events />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}
```

**Step 4: Implement each page** (each follows the same pattern)

Dashboard: Status cards with agent count, prefix count, peer count
Agents: Table with name, status badge, Bird version, last seen
Prefixes: Table with CIDR, enabled badge, add form, delete button
Sources: Table with name, URL, interval, last sync, sync button
Excludes: Table with CIDR, note, add form, delete button
Peers: Table with name, neighbor IP, ASN, multihop, add/delete
Events: Table with timestamp, type badge, message

Each page uses:
- `useEffect` + `useState` to fetch data on mount
- shadcn `Table`, `Button`, `Input`, `Dialog` components
- Lucide React icons for actions
- Auto-refresh every 10s for Dashboard

**Step 5: Build and copy to portal**

```bash
cd web && npm run build
cp -r dist/* ../cmd/portal/web/dist/
cd .. && go build ./cmd/portal/
```

**Step 6: Commit**

```bash
git add web/src/
git commit -m "feat: implement React dashboard pages

Dashboard, Agents, Prefixes, Sources, Excludes, Peers, Events.
Sidebar navigation with Lucide icons. Auto-refresh on dashboard."
```

---

## Phase 8: Integration & Build

### Task 12: Create Makefile for build pipeline

**Files:**
- Create: `Makefile`

```makefile
.PHONY: all portal agent web clean

all: portal agent

web:
	cd web && npm run build
	rm -rf cmd/portal/web/dist
	mkdir -p cmd/portal/web/dist
	cp -r web/dist/* cmd/portal/web/dist/

portal: web
	go build -o bin/rtbh-portal ./cmd/portal/

agent:
	go build -o bin/rtbhd ./cmd/agent/

proto:
	buf generate proto

clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf cmd/portal/web/dist/

test:
	go test ./...
```

**Step 1: Create Makefile and verify build**

```bash
make all
```

**Step 2: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile for portal and agent builds

make portal - builds React frontend + portal Go binary
make agent - builds agent Go binary
make proto - regenerates gRPC code"
```

### Task 13: Create systemd deployment files

**Files:**
- Create: `deploy/systemd/rtbh-portal.service`
- Create: `deploy/systemd/rtbh-portal.env.example`

**Step 1: Portal systemd service**

```ini
# deploy/systemd/rtbh-portal.service
[Unit]
Description=RTBH Portal
After=network.target

[Service]
Type=simple
User=rtbhd
Group=bird
EnvironmentFile=/etc/default/rtbh-portal
ExecStart=/usr/local/bin/rtbh-portal
Restart=on-failure
RestartSec=5
WorkingDirectory=/var/lib/rtbh-portal

[Install]
WantedBy=multi-user.target
```

**Step 2: Portal env example**

```
# deploy/systemd/rtbh-portal.env.example
RTBH_LISTEN_ADDR=:8088
RTBH_GRPC_ADDR=:50051
RTBH_DB_PATH=/var/lib/rtbh-portal/data/portal.db
RTBH_LOCAL_ASN=65500
RTBH_SCHEDULER_TICK_SECONDS=15
```

**Step 3: Commit**

```bash
git add deploy/systemd/
git commit -m "deploy: add systemd files for portal service"
```

---

## Summary

| Phase | Task | Description |
|-------|------|-------------|
| 1 | Restructure | Move code to portal/agent split |
| 2 | Protobuf | Define gRPC schema, generate code |
| 3 | SQLite Store | Portal state in SQLite |
| 4 | Portal Service | Business logic + gRPC server |
| 5 | Portal REST API | HTTP API + main binary |
| 6 | Agent Client | gRPC client for agents |
| 7 | React Frontend | Vite + Tailwind + shadcn/ui dashboard |
| 8 | Build & Deploy | Makefile + systemd |
