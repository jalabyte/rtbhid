# Multi-Bird Centralized Portal Design

## Overview

Transform RTBHID from a single-instance RTBH controller into a centralized portal that manages multiple Bird 2/3 instances across different servers. The portal provides a unified React dashboard for managing prefixes, sources, peers, and agents, while lightweight agents on each server handle Bird config generation and reload.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│              Central Portal (Go binary)               │
│                                                       │
│  ┌─────────────────┐  ┌──────────────────────────┐   │
│  │  REST API :8088  │  │  gRPC API :50051         │   │
│  │  (React UI +     │  │  (Agent communication)   │   │
│  │   HTTP handlers) │  │                          │   │
│  └─────────────────┘  └──────────────────────────┘   │
│  ┌─────────────────────────────────────────────────┐  │
│  │  State Store (SQLite: data/portal.db)            │  │
│  │  Syncer (remote blocklist fetch)                 │  │
│  │  Reconciler (effective prefix computation)       │  │
│  └─────────────────────────────────────────────────┘  │
└──────────┬──────────────┬──────────────┬──────────────┘
           │ gRPC         │ gRPC         │ gRPC
     ┌─────┴─────┐  ┌────┴──────┐  ┌───┴──────────┐
     │  Agent 1  │  │  Agent 2  │  │  Agent N     │
     │  (rtbhd)  │  │  (rtbhd)  │  │  (rtbhd)     │
     │  Bird 2   │  │  Bird 3   │  │  Bird 2      │
     └───────────┘  └───────────┘  └──────────────┘
```

### Communication Model

- **Agents register** with the portal on startup via `Register()` gRPC call
- **Portal pushes** config updates to agents via `StreamConfig()` server-side streaming
- **Agents report** health via periodic `Heartbeat()` calls
- **React UI** communicates with portal via REST API
- Agents behind NAT/firewall work naturally (outbound gRPC connection)

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Portal Backend | Go 1.23 | stdlib + SQLite + gRPC |
| Frontend Framework | React | 19.2.4 |
| Build Tool | Vite | 8.0.1 |
| CSS Framework | Tailwind CSS | 4.2.2 |
| UI Components | shadcn/ui | CLI 4.0.8 |
| Icons | Lucide React | 0.577.0 |
| Agent | Go 1.23 | Existing rtbhd + gRPC client |
| Database | SQLite | Single file, zero-ops |
| Protocol | gRPC + Protobuf | Agent communication |

## Project Structure

```
rtbhid/
├── cmd/
│   ├── portal/
│   │   └── main.go                  # Portal entry point
│   └── agent/
│       └── main.go                  # Agent entry point
├── internal/
│   ├── shared/
│   │   └── types.go                 # Shared types, constants
│   ├── portal/
│   │   ├── httpapi/
│   │   │   ├── server.go            # REST API for React UI
│   │   │   ├── auth.go              # Session authentication
│   │   │   ├── handlers.go          # API handlers
│   │   │   └── assets.go            # Embedded React build
│   │   ├── grpcapi/
│   │   │   ├── server.go            # gRPC server
│   │   │   └── handlers.go          # gRPC service implementations
│   │   ├── state/
│   │   │   ├── store.go             # SQLite store
│   │   │   ├── migrations.go        # Schema migrations
│   │   │   └── types.go             # DB types
│   │   └── service.go               # Business logic, reconciliation
│   └── agent/
│       ├── grpcclient/
│       │   └── client.go            # gRPC client to portal
│       ├── bird/
│       │   └── manager.go           # Bird config generation (reused)
│       └── config/
│           └── config.go            # Agent config (env vars)
├── proto/
│   └── rtbhid.proto                 # Protobuf definitions
├── gen/
│   └── rtbhid/                      # Generated gRPC code
├── web/                             # React frontend
│   ├── src/
│   │   ├── components/
│   │   │   ├── ui/                  # shadcn/ui components
│   │   │   ├── layout/              # Sidebar, TopBar
│   │   │   ├── agents/              # Agent table, status cards
│   │   │   ├── prefixes/            # Prefix management
│   │   │   ├── sources/             # Source management
│   │   │   ├── peers/               # Peer management
│   │   │   └── events/              # Event log
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Agents.tsx
│   │   │   ├── Prefixes.tsx
│   │   │   ├── Sources.tsx
│   │   │   ├── Excludes.tsx
│   │   │   ├── Peers.tsx
│   │   │   └── Events.tsx
│   │   ├── lib/
│   │   │   ├── api.ts               # REST API client
│   │   │   └── utils.ts
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── package.json
│   ├── vite.config.ts
│   └── index.html
├── data/                            # Runtime data
├── deploy/
│   └── systemd/
└── scripts/
```

## Data Model (SQLite)

```sql
-- Registered agents
CREATE TABLE agents (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    hostname      TEXT,
    bird_version  TEXT,
    listen_addr   TEXT,
    status        TEXT DEFAULT 'offline',
    last_seen     DATETIME,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Manual blackhole prefixes
CREATE TABLE prefixes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cidr        TEXT NOT NULL UNIQUE,
    enabled     BOOLEAN DEFAULT 1,
    note        TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Remote blocklist sources
CREATE TABLE sources (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    url         TEXT NOT NULL,
    interval    INTEGER DEFAULT 3600,
    enabled     BOOLEAN DEFAULT 1,
    last_sync   DATETIME,
    last_count  INTEGER DEFAULT 0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Prefixes from synced sources
CREATE TABLE source_prefixes (
    source_id   TEXT REFERENCES sources(id) ON DELETE CASCADE,
    cidr        TEXT NOT NULL,
    PRIMARY KEY (source_id, cidr)
);

-- Exclude list
CREATE TABLE excludes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    cidr        TEXT NOT NULL UNIQUE,
    note        TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- BGP peers
CREATE TABLE peers (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    neighbor_ip   TEXT NOT NULL,
    remote_as     INTEGER NOT NULL,
    local_as      INTEGER DEFAULT 65500,
    multihop      INTEGER DEFAULT 1,
    enabled       BOOLEAN DEFAULT 1,
    std_communities    TEXT,  -- JSON array
    large_communities  TEXT,  -- JSON array
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Audit events
CREATE TABLE events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    type        TEXT NOT NULL,
    message     TEXT NOT NULL,
    agent_id    TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Effective prefixes** = `(manual_prefixes UNION source_prefixes) MINUS excludes`
Computed on reconcile, same CIDR-aware logic as current `EffectivePrefixes()`.

All agents receive the same effective config (uniform push).

## gRPC API

```protobuf
syntax = "proto3";
package rtbhid;

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

### Agent Flow

1. Agent starts → generate/load persistent `agent_id` (UUID, saved to file)
2. Call `Register()` with agent ID, name, Bird version
3. Open `StreamConfig()` — receive current config, then listen for updates
4. On `ConfigUpdate`: write Bird config via `bird.Manager.Apply()`, reload if configured
5. Periodically call `Heartbeat()` with status, prefix/peer counts, Bird health
6. Portal marks agents offline if no heartbeat for >2x interval

## REST API

```
Auth:
POST   /api/v1/auth/login
POST   /api/v1/auth/logout

Dashboard:
GET    /api/v1/status              # Overview: agent count, prefix count, peer count, events

Agents:
GET    /api/v1/agents              # List agents with status
GET    /api/v1/agents/:id          # Agent detail
DELETE /api/v1/agents/:id          # Deregister

Prefixes:
GET    /api/v1/prefixes            # Paginated list
POST   /api/v1/prefixes            # Add (triggers config push to agents)
DELETE /api/v1/prefixes/:id        # Remove
PUT    /api/v1/prefixes/:id/toggle # Enable/disable

Sources:
GET    /api/v1/sources
POST   /api/v1/sources
PUT    /api/v1/sources/:id
DELETE /api/v1/sources/:id
POST   /api/v1/sources/:id/sync    # Force sync

Excludes:
GET    /api/v1/excludes
POST   /api/v1/excludes
DELETE /api/v1/excludes/:id

Peers:
GET    /api/v1/peers
POST   /api/v1/peers
PUT    /api/v1/peers/:id
DELETE /api/v1/peers/:id

Events:
GET    /api/v1/events              # Paginated, filterable by type/agent
```

### Write Propagation

When any write operation occurs (add prefix, modify peer, etc.):
1. Portal updates SQLite
2. Recomputes effective prefixes
3. Pushes new `ConfigUpdate` to all connected agents via gRPC streams
4. Logs an event

## React Frontend

### Pages

| Page | Description | Key Components |
|------|-------------|----------------|
| Dashboard | Overview KPIs | Status cards, recent events, agent status grid |
| Agents | Agent management | Agent table with status badges, Bird version, last seen |
| Prefixes | Manual prefix management | CIDR table, add form, bulk import, enable/disable toggle |
| Sources | Blocklist sources | Source table, add/edit form, sync button, interval config |
| Excludes | Exclude management | CIDR table, add form, CIDR-aware display |
| Peers | BGP peer management | Peer table, add/edit form, community tags |
| Events | Audit log | Filterable event list with timestamps |

### Layout

- Left sidebar with navigation (Lucide icons)
- Top bar with portal status, user menu, logout
- Responsive design (collapsible sidebar on mobile)

### Real-time Updates

- REST API polling every 10s for dashboard status (same as current)
- Agent status shown with online/offline badges based on last heartbeat

### Build & Embedding

- React app builds to `web/dist/` via `npm run build`
- Portal binary embeds `web/dist/` via `//go:embed web/dist`
- Portal serves static assets and falls back to `index.html` for SPA routing
- Single binary deployment: portal binary contains everything

## Agent Changes

### New Agent Mode

```bash
# Agent mode (new)
rtbhd agent --portal-url=https://portal.example.com:50051 \
            --agent-name=server-jkt-01 \
            --bird-version=3

# Standalone mode (existing, backward compatible)
rtbhd standalone
```

### Agent Config (Environment Variables)

```
RTBH_MODE=agent|standalone          # Operating mode
RTBH_PORTAL_URL=grpc://host:50051   # Portal gRPC address
RTBH_AGENT_NAME=server-jkt-01       # Human-readable name
RTBH_AGENT_ID_PATH=./data/agent_id  # Persistent agent ID file
RTBH_BIRD_OUTPUT=/etc/bird/rtbh_auto.conf
RTBH_BIRD_PEERS_OUTPUT=/etc/bird/rtbh_peers_auto.conf
RTBH_BIRD_RELOAD=true
RTBH_BIRD_COMMAND=birdc configure
RTBH_LOCAL_ASN=65500
```

### What Changes

- Add `grpcclient/` package for portal communication
- Agent business logic simplified: just apply config from portal, generate Bird files, report status
- Remove `state/` (JSON store), `syncer/` (blocklist fetcher), scheduler from agent — portal handles all that
- Keep `bird/manager.go` unchanged — still generates Bird config files

### What Stays the Same

- Bird config generation (`renderRoutes()`, `renderPeers()`)
- Bird reload mechanism
- Standalone mode for backward compatibility

## Implementation Phases

### Phase 1: Portal Backend Foundation
- Set up SQLite schema and migrations
- Implement portal state store
- Build REST API handlers
- Set up gRPC server
- Implement agent registration and heartbeat

### Phase 2: Agent Refactoring
- Add gRPC client package
- Implement agent mode alongside standalone mode
- Wire config streaming and Bird config application
- Test agent ↔ portal communication

### Phase 3: React Frontend
- Scaffold React project (Vite + Tailwind + shadcn/ui)
- Build all pages (Dashboard, Agents, Prefixes, Sources, Excludes, Peers, Events)
- Integrate REST API client
- Embed into portal binary

### Phase 4: Integration & Polish
- End-to-end testing with multiple agents
- Error handling and reconnection logic
- Documentation and deployment guides
- Migrate existing data from state.json to SQLite
