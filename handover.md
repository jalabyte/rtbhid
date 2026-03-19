# RTBHID — Agent Handover Document

## What Is This Project?

RTBHID is a **Remotely Triggered Black Hole (RTBH) controller** for DDoS mitigation. It manages BIRD Internet Routing Daemon instances (v2/v3) by controlling which IP prefixes get blackholed and which BGP peers receive the routes. Originally a standalone single-instance tool, it has been refactored into a **centralized portal** that manages multiple BIRD instances via agents.

---

## Two Modes of Operation

| Mode | Binary | Storage | Description |
|------|--------|---------|-------------|
| **Standalone** | `rtbhd` | JSON file (`state.json`) | Single BIRD instance, no network agents. Legacy mode. |
| **Portal** | `rtbh-portal` | SQLite (`portal.db`) | Central dashboard managing multiple agents over gRPC. |
| **Agent** | `rtbhd` (with `RTBH_MODE=agent`) | None | Connects to portal via gRPC, applies config to local BIRD. |

The `cmd/agent/main.go` binary supports both standalone and agent mode via the `RTBH_MODE` env var.

---

## Directory Structure

```
rtbhid/
├── cmd/
│   ├── portal/main.go          # Portal entry point (HTTP + gRPC servers)
│   └── agent/main.go           # Standalone/Agent entry point
├── internal/
│   ├── portal/
│   │   ├── httpapi/
│   │   │   ├── server.go       # Portal REST API (all /api/v1/ routes)
│   │   │   ├── auth.go         # Auth manager (HMAC-SHA256 sessions)
│   │   │   └── login.html      # Embedded login page (go:embed)
│   │   ├── grpcapi/server.go   # Portal gRPC server (agent communication)
│   │   ├── service.go          # Portal business logic (CRUD, sync, config push)
│   │   └── state/
│   │       ├── store.go        # SQLite store (all DB operations)
│   │       ├── types.go        # Data model types (Agent, Prefix, Source, Exclude, Peer, Event)
│   │       └── migrations.go   # Schema (CREATE TABLE IF NOT EXISTS)
│   ├── agent/
│   │   ├── config/config.go    # Agent config loader
│   │   ├── grpcclient/client.go # gRPC client (connects to portal)
│   │   └── bird/manager.go     # Bird config file writer
│   ├── app/service.go          # Standalone business logic
│   ├── bird/manager.go         # Bird config generation (standalone)
│   ├── config/config.go        # Standalone config loader
│   ├── httpapi/                # Standalone HTTP API + auth
│   ├── state/                  # Standalone JSON state store
│   ├── shared/types.go         # Shared types (mostly empty)
│   └── syncer/syncer.go        # URL fetcher for prefix sources
├── gen/rtbhid/                  # Generated gRPC protobuf code
├── proto/
│   ├── rtbhid.proto            # gRPC service definition
│   └── buf.yaml                # buf config
├── web/                        # React frontend (Vite + Tailwind + shadcn/ui)
│   ├── src/
│   │   ├── App.tsx             # Router setup
│   │   ├── lib/api.ts          # API client (all fetch calls)
│   │   ├── pages/              # Page components
│   │   └── components/
│   │       ├── layout/Sidebar.tsx
│   │       └── ui/             # shadcn/ui components
│   ├── package.json
│   └── vite.config.ts
├── deploy/systemd/             # systemd service files
├── Makefile                    # Build targets
└── data/                       # Runtime data (gitignored)
```

---

## Build & Run

```bash
# Build everything (web + Go portal binary)
make portal         # outputs bin/rtbh-portal

# Build agent only
make agent          # outputs bin/rtbhd

# Build React web assets only
make web            # builds web/, copies to cmd/portal/web/dist/

# Build for Linux (cross-compile from macOS/Linux)
make portal-linux   # outputs bin/rtbh-portal-linux-amd64
make agent-linux    # outputs bin/rtbhd-linux-amd64
make all-linux      # builds both portal and agent for Linux

# Regenerate protobuf code
make proto          # requires buf

# Run portal
RTBH_LISTEN_ADDR=:8088 \
RTBH_GRPC_ADDR=:50051 \
RTBH_DB_PATH=./data/portal.db \
RTBH_LOCAL_ASN=65500 \
./bin/rtbh-portal

# Run standalone agent
RTBH_MODE=standalone \
RTBH_LISTEN_ADDR=:8088 \
./bin/rtbhd

# Run agent mode (connects to portal)
RTBH_MODE=agent \
RTBH_PORTAL_URL=portal.example.com:50051 \
RTBH_BIRD_VERSION=2 \
RTBH_BIRD_OUTPUT=/etc/bird/rtbh_auto.conf \
RTBH_BIRD_RELOAD=true \
./bin/rtbhd
```

### Web Dev (hot reload)

```bash
cd web
npm install
npm run dev         # Vite dev server (proxies API to :8088)
```

---

## Environment Variables

### Portal

| Variable | Default | Description |
|----------|---------|-------------|
| `RTBH_LISTEN_ADDR` | `:8088` | HTTP listen address |
| `RTBH_GRPC_ADDR` | `:50051` | gRPC listen address |
| `RTBH_DB_PATH` | `./data/portal.db` | SQLite database path |
| `RTBH_LOCAL_ASN` | `65500` | Local AS number |
| `RTBH_SCHEDULER_TICK_SECONDS` | `15` | Source sync interval check |
| `RTBH_AUTH_USER` | *(disabled)* | Auth username (enables auth) |
| `RTBH_AUTH_PASS` | *(disabled)* | Auth password |
| `RTBH_AUTH_SESSION_SECRET` | | HMAC-SHA256 session signing key |
| `RTBH_AUTH_SESSION_TTL_HOURS` | `24` | Session cookie TTL |
| `RTBH_AUTH_COOKIE_SECURE` | `false` | Set Secure flag on cookies |

### Agent / Standalone

| Variable | Default | Description |
|----------|---------|-------------|
| `RTBH_MODE` | `standalone` | `standalone` or `agent` |
| `RTBH_PORTAL_URL` | `localhost:50051` | Portal gRPC address (agent mode) |
| `RTBH_AGENT_NAME` | hostname | Agent display name |
| `RTBH_BIRD_VERSION` | `2` | BIRD version (`2` or `3`) |
| `RTBH_BIRD_OUTPUT` | `./data/rtbh_auto.conf` | BIRD prefix config output |
| `RTBH_BIRD_PEERS_OUTPUT` | `./data/rtbh_peers_auto.conf` | BIRD peers config output |
| `RTBH_BIRD_RELOAD` | `false` | Auto-reload BIRD after config change |
| `RTBH_BIRD_COMMAND` | `birdc configure` | BIRD reload command |
| `RTBH_STATE_PATH` | `./data/state.json` | JSON state file (standalone) |
| `RTBH_EXPORT_STD_COMMUNITIES` | | Standard BGP communities for export |
| `RTBH_EXPORT_LARGE_COMMUNITIES` | | Large BGP communities for export |
| `RTBH_AUTH_USER` / `RTBH_AUTH_PASS` | | Auth (same as portal) |

---

## Database Schema (SQLite — Portal)

8 tables defined in `internal/portal/state/migrations.go`:

| Table | Key | Description |
|-------|-----|-------------|
| `agents` | `id TEXT PK` | Registered agents |
| `prefixes` | `id INTEGER PK` | Manual blackhole prefixes |
| `sources` | `id TEXT PK` | URL-based prefix sources |
| `source_prefixes` | `(source_id, cidr)` | Synced prefixes from sources |
| `excludes` | `id INTEGER PK` | Manual exclude prefixes |
| `exclude_sources` | `id TEXT PK` | URL-based exclude sources |
| `exclude_source_prefixes` | `(source_id, cidr)` | Synced prefixes from exclude sources |
| `peers` | `id TEXT PK` | BGP peer configurations |
| `events` | `id INTEGER PK` | Audit log / event stream |

### Effective Prefixes Logic

`EffectivePrefixes()` in `store.go:492` computes the final prefix list:
1. Collect enabled manual prefixes
2. Collect enabled source prefixes (joined from `source_prefixes`)
3. Build exclusion set from manual excludes + exclude source prefixes
4. Merge: manual + source - excludes (CIDR-aware: `10.0.0.0/8` excludes `10.1.0.0/16`)
5. Sort and return

---

## REST API Endpoints

All under `/api/v1/` (except auth/login). Full list from `server.go`:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/status` | Agent/source/peer counts + effective_prefixes |
| GET/DELETE | `/agents/:id` | List/delete agents |
| GET/POST | `/prefixes` | List/create prefixes |
| PUT | `/prefixes/:id/toggle` | Enable/disable prefix |
| DELETE | `/prefixes/:id` | Delete prefix |
| GET/POST | `/sources` | List/create prefix sources |
| PUT | `/sources/:id/toggle` | Enable/disable source |
| DELETE | `/sources/:id` | Delete source |
| POST | `/sources/:id/sync` | Force source sync |
| GET/POST | `/excludes` | List/create excludes |
| DELETE | `/excludes/:id` | Delete exclude |
| GET/POST | `/exclude-sources` | List/create exclude sources |
| PUT | `/exclude-sources/:id/toggle` | Enable/disable exclude source |
| DELETE | `/exclude-sources/:id` | Delete exclude source |
| POST | `/exclude-sources/:id/sync` | Force exclude source sync |
| GET/POST | `/peers` | List/create peers |
| PUT/DELETE | `/peers/:id` | Update/delete peer |
| GET | `/events?limit=N` | Recent events (default 100) |
| POST | `/register` | Agent registration |
| POST | `/heartbeat` | Agent heartbeat |
| GET | `/config?agentID=X` | Long-poll config updates |
| GET | `/auth/me` | Auth status |
| POST | `/auth/login` | Login |
| POST | `/auth/logout` | Logout |

---

## Auth System

HMAC-SHA256 signed session cookies. Works identically in standalone and portal modes.

- Cookie name: `rtbhd_session`
- Session token format: `base64(username:expiry)` + `:` + `hmac-sha256-hex(secret, token)`
- `requireAuth()` middleware wraps the entire handler — redirects browser requests to `/login`, returns 401 for API requests
- Public paths (no auth required): `/login`, `/health`, `/auth/login`, `/auth/logout`, `/auth/me`
- Enabled when `RTBH_AUTH_USER` is set (portal) or `RTBH_AUTH_USER` + `RTBH_AUTH_PASS` are set (standalone)
- Login page is a standalone HTML page (not React) — embedded via `//go:embed login.html`

---

## gRPC Protocol

Defined in `proto/rtbhid.proto`. Three RPCs:

```
service Portal {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc StreamConfig(ConfigRequest) returns (stream ConfigUpdate);
}
```

`StreamConfig` is server-side streaming — the portal pushes config updates (prefixes + peers) to agents whenever anything changes. Agents receive updates via the gRPC stream or fall back to REST long-poll at `/api/v1/config`.

---

## Frontend Stack

- React 19.2.4 + Vite 8.0.1
- Tailwind CSS 4.2.2 (via `@tailwindcss/vite`)
- shadcn/ui components (Button, Card, Dialog, Table, etc.)
- react-router 7.13.1 (BrowserRouter)
- lucide-react icons
- TypeScript 5.9.3

### Pages

| Path | Component | File |
|------|-----------|------|
| `/` | Dashboard | `src/pages/Dashboard.tsx` |
| `/agents` | Agents | `src/pages/Agents.tsx` |
| `/prefixes` | Prefixes | `src/pages/Prefixes.tsx` |
| `/sources` | Sources | `src/pages/Sources.tsx` |
| `/excludes` | Excludes | `src/pages/Excludes.tsx` |
| `/peers` | Peers | `src/pages/Peers.tsx` |
| `/events` | Events | `src/pages/Events.tsx` |

All API calls go through `src/lib/api.ts` which exports an `api` object with typed methods for every endpoint.

---

## Deployment (systemd)

Two service files in `deploy/systemd/`:

- `rtbh-portal.service` — runs as `User=rtbhd, Group=bird`, env from `/etc/default/rtbh-portal`, working dir `/var/lib/rtbh-portal`
- `rtbhd.service` — runs as `User=rtbhd, Group=bird`, env from `/etc/default/rtbhd`, working dir `/var/lib/rtbhd`

---

## Key Architectural Patterns

1. **Config push on mutation** — Every CRUD operation on prefixes, peers, excludes, or sources calls `PushConfigToAll()` which sends updated config to all connected agents.

2. **Source sync scheduler** — `syncScheduler()` in `service.go` ticks every N seconds, iterates all enabled sources (both prefix and exclude), and syncs those whose interval has elapsed.

3. **Long-poll config delivery** — REST endpoint `/api/v1/config?agentID=X` blocks until a config update is available, then returns it. Agents can use this as fallback instead of gRPC streaming.

4. **Source ID generation** — `newResourceID("src")` for prefix sources, `newResourceID("xsrc")` for exclude sources. Format: `{prefix}_{8 hex chars}`.

5. **CIDR normalization** — All CIDRs go through `normalizeCIDR()` which uses `netip.ParsePrefix` to mask and validate. Bare IPs get `/32` or `/128` appended.

6. **SPA fallback** — The portal HTTP server embeds the React build. Unknown non-API paths serve `index.html` for client-side routing.

---

## Recent Changes (Completed)

- **Exclude sources** — Added remote URL-based exclude lists with sync scheduler, mirroring the existing source pattern. Tables: `exclude_sources`, `exclude_source_prefixes`.
- **Portal auth** — Ported auth system from standalone to portal. Login page, session cookies, auth middleware.
- **Source ID auto-generation** — Backend generates IDs; frontend removed manual ID input field.
- **Sidebar logout** — React sidebar checks auth status and shows logout button when auth is enabled.
- **Effective prefixes stats** — Dashboard now shows effective prefix count (manual + sources - excludes) via `/status` endpoint. Added `GetEffectivePrefixCount()` to service layer.
- **Linux build targets** — Makefile now includes `portal-linux`, `agent-linux`, and `all-linux` targets for cross-compiling to Linux amd64 from macOS/Linux.
- **Source enable/disable** — Added toggle functionality for sources and exclude sources. Backend: `PUT /sources/:id/toggle`, `PUT /exclude-sources/:id/toggle`. Frontend: Status badge and Power button to activate/deactivate. Sources are now enabled by default when created.

---

## Common Tasks for Next Agent

1. **Adding a new API endpoint** — Add handler in `internal/portal/httpapi/server.go`, register route in `Handler()`, add service method in `internal/portal/service.go`, add store method in `internal/portal/state/store.go`, add frontend method in `web/src/lib/api.ts`, create/update page in `web/src/pages/`.

2. **Adding a new DB table** — Add CREATE TABLE to `migrations.go`, add type to `types.go`, add CRUD in `store.go`.

3. **Modifying the gRPC protocol** — Edit `proto/rtbhid.proto`, run `make proto` to regenerate `gen/rtbhid/`.

4. **Adding a new React page** — Create `web/src/pages/NewPage.tsx`, add route in `web/src/App.tsx`, add nav item in `web/src/components/layout/Sidebar.tsx`.

5. **Building for deployment** — Run `make portal` (builds web + Go binary), copy `bin/rtbh-portal` to server, set up systemd service with env file.
