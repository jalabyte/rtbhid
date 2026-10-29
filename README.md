# RTBHID - Remotely Triggered Black Hole Controller

RTBHID adalah sistem kontrol RTBH (Remotely Triggered Black Hole) terpusat untuk mitigasi DDoS. Sistem ini mengelola instance BIRD Internet Routing Daemon (v2/v3) dengan mengontrol prefix IP yang di-blackhole dan BGP peers yang menerima route.

## Arsitektur

RTBHID memiliki tiga mode operasi:

| Mode | Binary | Storage | Deskripsi |
|------|--------|---------|-----------|
| **Portal** | `rtbh-portal` | SQLite (`portal.db`) | Dashboard terpusat yang mengelola banyak agent via gRPC |
| **Agent** | `rtbhd` (dengan `RTBH_MODE=agent`) | None | Terhubung ke portal via gRPC, menerapkan config ke BIRD lokal |
| **Standalone** | `rtbhd` (default) | JSON file (`state.json`) | Mode legacy untuk single BIRD instance |

```
┌─────────────────┐
│   rtbh-portal   │  Portal (Dashboard + gRPC Server)
│   (SQLite DB)   │
└────────┬────────┘
         │ gRPC
    ┌────┴────┬─────────┐
    │         │         │
┌───▼───┐ ┌───▼───┐ ┌───▼───┐
│Agent 1│ │Agent 2│ │Agent N│  Agents (BIRD Config)
│ BIRD  │ │ BIRD  │ │ BIRD  │
└───────┘ └───────┘ └───────┘
```

## Fitur Utama

- ✅ Manajemen prefix manual (add/delete/enable/disable)
- ✅ Sinkronisasi prefix dari remote URL (multiple sources)
- ✅ Exclude list untuk false-positive prevention
- ✅ Manajemen BGP peer terpusat (single/bulk)
- ✅ Embedded BGP server (opsional, pengganti BIRD) dengan auto-accept peer
- ✅ ASN whitelist (gabungan data delegasi APNIC/ARIN/RIPE/LACNIC/AFRINIC untuk country ID, bisa tambah/hapus manual)
- ✅ Monitoring sesi BGP real-time di dashboard + endpoint Prometheus `/metrics`
- ✅ Auto-detect BIRD version (2.x/3.x)
- ✅ Dashboard web modern (React + Tailwind CSS)
- ✅ REST API lengkap
- ✅ gRPC untuk komunikasi portal-agent
- ✅ Authentication dengan session cookie
- ✅ Dokumentasi publik untuk integrasi upstream
- ✅ Linux build targets untuk deployment

---

## Quick Start

### Prerequisites

#### System Requirements

- **Operating System**: Linux (recommended), macOS, atau Windows dengan WSL2
- **Architecture**: amd64 atau arm64
- **Memory**: Minimal 512MB RAM
- **Disk**: Minimal 100MB untuk binary dan database

#### Software Dependencies

##### 1. Go 1.21+

**Linux (Debian/Ubuntu):**
```bash
sudo apt update
sudo apt install -y golang-go
```

**Linux (RHEL/CentOS):**
```bash
sudo yum install -y golang
```

**macOS:**
```bash
brew install go
```

**Verifikasi:**
```bash
go version
# Output: go version go1.21.x linux/amd64
```

##### 2. Node.js 18+ dan npm

**Linux (Debian/Ubuntu):**
```bash
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs
```

**Linux (RHEL/CentOS):**
```bash
curl -fsSL https://rpm.nodesource.com/setup_20.x | sudo bash -
sudo yum install -y nodejs
```

**macOS:**
```bash
brew install node
```

**Verifikasi:**
```bash
node --version
# Output: v20.x.x
npm --version
# Output: 10.x.x
```

##### 3. buf (Opsional - untuk regenerate protobuf)

Hanya diperlukan jika Anda ingin mengubah file `.proto`:

**Linux/macOS:**
```bash
# Using curl
curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-$(uname -s)-$(uname -m)" \
  -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf

# Or using npm
npm install -g @bufbuild/buf
```

**Verifikasi:**
```bash
buf --version
# Output: 1.x.x
```

##### 4. BIRD 2.x atau 3.x (Hanya untuk Agent)

BIRD hanya diperlukan jika Anda menjalankan **agent mode**.

**Linux (Debian/Ubuntu):**
```bash
# BIRD 2
sudo apt install -y bird2

# Atau BIRD 3 (jika tersedia di repository)
sudo apt install -y bird
```

**Linux (RHEL/CentOS):**
```bash
sudo yum install -y bird
```

**Build dari source (untuk versi terbaru):**
```bash
# Download dari https://bird.network.cz/
wget https://bird.network.cz/download/bird-3.0.0.tar.gz
tar xzf bird-3.0.0.tar.gz
cd bird-3.0.0
./configure
make
sudo make install
```

**Verifikasi:**
```bash
bird --version
# Output: BIRD version 3.0.0

birdc show status
# Output: BIRD 3.0.0 ready.
```

#### npm Install Step

**PENTING:** Anda perlu menjalankan `npm install` untuk menginstall dependencies frontend **sebelum** build pertama kali.

```bash
# Masuk ke direktori web
cd web

# Install dependencies
npm install

# Kembali ke root project
cd ..
```

Ini akan menginstall semua dependencies yang diperlukan untuk build frontend, termasuk:
- React
- Vite
- Tailwind CSS
- shadcn/ui components
- TypeScript

**Catatan:** `npm install` hanya perlu dijalankan sekali, atau ketika ada perubahan di `package.json`.

#### Summary Checklist

Sebelum mulai build, pastikan semua prerequisites sudah terinstall:

- [ ] Go 1.21+ (`go version`)
- [ ] Node.js 18+ (`node --version`)
- [ ] npm (`npm --version`)
- [ ] (Opsional) buf (`buf --version`) - hanya jika regenerate protobuf
- [ ] (Agent only) BIRD (`bird --version`)
- [ ] Frontend dependencies sudah diinstall (`cd web && npm install`)

### Build Portal

```bash
# Clone repository
git clone https://github.com/your-org/rtbhid.git
cd rtbhid

# Build portal (frontend + backend)
make portal

# Output: bin/rtbh-portal
```

### Build Agent

```bash
# Build agent
make agent

# Output: bin/rtbhd
```

### Build untuk Linux

```bash
# Build portal untuk Linux amd64
make portal-linux
# Output: bin/rtbh-portal-linux-amd64

# Build agent untuk Linux amd64
make agent-linux
# Output: bin/rtbhd-linux-amd64

# Build keduanya sekaligus
make all-linux
```

---

## Deployment

### Portal Deployment

Portal adalah dashboard terpusat yang mengelola semua agent.

#### 1. Environment Variables

Buat file `/etc/default/rtbh-portal`:

```bash
# Network
RTBH_LISTEN_ADDR=:8088
RTBH_GRPC_ADDR=:50051

# Database
RTBH_DB_PATH=/var/lib/rtbh-portal/portal.db

# Local ASN (digunakan semua agent)
RTBH_LOCAL_ASN=65500

# Scheduler
RTBH_SCHEDULER_TICK_SECONDS=15

# IP ASN Lookup (opsional)
# Dapatkan token gratis di https://ipinfo.io/signup
RTBH_IPINFO_TOKEN=your-ipinfo-token

# Authentication (opsional)
RTBH_AUTH_USER=admin
RTBH_AUTH_PASS=your-secure-password
RTBH_AUTH_SESSION_SECRET=your-session-secret-key
RTBH_AUTH_SESSION_TTL_HOURS=24
RTBH_AUTH_COOKIE_SECURE=false
```

#### 2. Systemd Service

```bash
# Copy binary
sudo cp bin/rtbh-portal /usr/local/bin/

# Create user dan directory
sudo useradd -r -s /bin/false rtbhd
sudo mkdir -p /var/lib/rtbh-portal
sudo chown rtbhd:rtbhd /var/lib/rtbh-portal

# Install systemd service
sudo cp deploy/systemd/rtbh-portal.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable rtbh-portal
sudo systemctl start rtbh-portal
```

#### 3. Akses Dashboard

Buka browser: `http://your-server:8088`

Login dengan username/password yang dikonfigurasi (jika auth enabled).

---

### Agent Deployment

Agent berjalan di setiap router BIRD dan terhubung ke portal.

#### 1. Environment Variables

Buat file `/etc/default/rtbhd`:

```bash
# Mode
RTBH_MODE=agent

# Portal connection
RTBH_PORTAL_URL=portal.example.com:50051

# Agent identity
RTBH_AGENT_NAME=router-01

# BIRD configuration
RTBH_BIRD_VERSION=3
RTBH_BIRD_OUTPUT=/etc/bird/rtbh_auto.conf
RTBH_BIRD_PEERS_OUTPUT=/etc/bird/rtbh_peers_auto.conf
RTBH_BIRD_RELOAD=true
RTBH_BIRD_COMMAND=birdc -s /var/run/bird/bird.ctl configure
RTBH_BIRD_CONF_DIR=/etc/bird

# Local ASN (opsional, default dari portal)
RTBH_LOCAL_ASN=65500

# Export communities
RTBH_EXPORT_STD_COMMUNITIES=65535:666
RTBH_EXPORT_LARGE_COMMUNITIES=65535:666:666
```

#### 2. Systemd Service

```bash
# Copy binary
sudo cp bin/rtbhd /usr/local/bin/

# Create user dan group
sudo useradd -r -s /bin/false -G bird rtbhd

# Create directory
sudo mkdir -p /var/lib/rtbhd
sudo chown rtbhd:bird /var/lib/rtbhd

# Install systemd service
sudo cp deploy/systemd/rtbhd.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable rtbhd
sudo systemctl start rtbhd
```

#### 3. BIRD Configuration

Tambahkan include di `/etc/bird/bird.conf`:

```bird
# RTBH routes
include "/etc/bird/rtbh_auto.conf";

# RTBH peers
include "/etc/bird/rtbh_peers_auto.conf";
```

#### 4. Auto-detect BIRD Version

Agent akan otomatis mendeteksi versi BIRD dengan menjalankan:
```bash
birdc -s /var/run/bird/bird.ctl show status
```

Jika berhasil, agent akan melaporkan versi lengkap (mis. "3.2.0").

Fallback: Jika deteksi gagal, gunakan `RTBH_BIRD_VERSION` environment variable.

---

### Standalone Deployment (Legacy)

Untuk single BIRD instance tanpa portal:

```bash
# Mode standalone (default)
RTBH_LISTEN_ADDR=:8080 \
RTBH_STATE_PATH=/var/lib/rtbhd/state.json \
RTBH_BIRD_OUTPUT=/etc/bird/rtbh_auto.conf \
RTBH_BIRD_RELOAD=true \
RTBH_BIRD_COMMAND='birdc -s /var/run/bird/bird.ctl configure' \
./bin/rtbhd
```

---

## Environment Variables Reference

### Portal Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RTBH_LISTEN_ADDR` | `:8088` | HTTP listen address |
| `RTBH_GRPC_ADDR` | `:50051` | gRPC listen address |
| `RTBH_GRPC_MAX_MSG_SIZE` | `0` (4MB default) | gRPC max message size in bytes |
| `RTBH_DB_PATH` | `./data/portal.db` | SQLite database path |
| `RTBH_LOCAL_ASN` | `65500` | Local ASN untuk semua agent |
| `RTBH_SCHEDULER_TICK_SECONDS` | `15` | Interval sync check |
| `RTBH_IPINFO_TOKEN` | *(disabled)* | ipinfo.io token untuk ASN lookup |
| `RTBH_BGP_AUTO_ACCEPT` | `true` | Izinkan agent auto-accept peer dari ASN whitelist |
| `RTBH_WHITELIST_STRICT_ID` | `true` | Tolak manual add ASN yang tidak ada di registry country-ID RIR |
| `RTBH_ID_ASN_SOURCE_URL` | semua RIR | Override satu URL sumber daftar ASN Indonesia |
| `RTBH_ID_ASN_SOURCE_URLS` | semua RIR | Override daftar URL (dipisah koma) sumber ASN Indonesia |
| `RTBH_METRICS_TOKEN` | *(public)* | Wajibkan `Authorization: Bearer <token>` atau `?token=` untuk `/metrics` |
| `RTBH_AUTH_USER` | *(disabled)* | Auth username |
| `RTBH_AUTH_PASS` | *(disabled)* | Auth password |
| `RTBH_AUTH_SESSION_SECRET` | *(random)* | Session signing key |
| `RTBH_AUTH_SESSION_TTL_HOURS` | `24` | Session TTL |
| `RTBH_AUTH_COOKIE_SECURE` | `false` | Set Secure flag |

### Agent Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RTBH_MODE` | `standalone` | `agent` atau `standalone` |
| `RTBH_PORTAL_URL` | `localhost:50051` | Portal gRPC address |
| `RTBH_GRPC_MAX_MSG_SIZE` | `0` (4MB default) | gRPC max message size in bytes |
| `RTBH_AGENT_NAME` | *(hostname)* | Agent display name |
| `RTBH_BIRD_VERSION` | *(auto-detect)* | BIRD version (2/3) |
| `RTBH_BIRD_OUTPUT` | `./data/rtbh_auto.conf` | Routes output file |
| `RTBH_BIRD_PEERS_OUTPUT` | `./data/rtbh_peers_auto.conf` | Peers output file |
| `RTBH_BIRD_RELOAD` | `false` | Auto-reload BIRD |
| `RTBH_BIRD_COMMAND` | `birdc configure` | BIRD reload command |
| `RTBH_BIRD_CONF_DIR` | *(none)* | BIRD config directory |
| `RTBH_LOCAL_ASN` | `65500` | Local ASN (jika berbeda dari portal) |
| `RTBH_EXPORT_STD_COMMUNITIES` | *(none)* | Standard BGP communities |
| `RTBH_EXPORT_LARGE_COMMUNITIES` | *(none)* | Large BGP communities |
| `RTBH_BGP_ENGINE` | `bird` | `bird` atau `custom` (embedded BGP server) |
| `RTBH_BGP_LISTEN_ADDR` | `:179` | Alamat listen embedded BGP server |
| `RTBH_BGP_ROUTER_ID` | *(auto)* | BGP router ID (IPv4) |
| `RTBH_BGP_NEXTHOP_V4` | *(local addr)* | Next hop untuk announcement IPv4 |
| `RTBH_BGP_NEXTHOP_V6` | *(local addr)* | Next hop untuk announcement IPv6 |
| `RTBH_BGP_HOLD_TIME_SEC` | `90` | Hold time BGP |
| `RTBH_BGP_ENABLE_IPV6` | `true` | Advertise capability IPv6 unicast |
| `RTBH_BGP_MAX_PREFIXES` | `1000` | Batas prefix yang diterima dari peer |
| `RTBH_BGP_MAX_DYNAMIC_PEERS` | `256` | Batas sesi inbound auto-accept |

---

## Custom BGP Engine

Agent dapat menjalankan BGP server bawaan (`RTBH_BGP_ENGINE=custom`) sebagai
pengganti BIRD. Engine ini mengumumkan prefix RTBH (blackhole) ke peer statis
dan menerima sesi BGP masuk dari ASN yang ada di whitelist portal.

- Inbound session dari ASN yang tidak di-whitelist ditolak dengan NOTIFICATION.
- Peer statis dari halaman **Peers** tetap dipakai, baik aktif maupun inbound.
- Import ASN Indonesia menggabungkan file delegasi APNIC, ARIN, RIPE, LACNIC,
  dan AFRINIC. Ini penting karena organisasi Indonesia bisa saja mendapat ASN
  dari RIR di luar APNIC. Override dengan `RTBH_ID_ASN_SOURCE_URL` /
  `RTBH_ID_ASN_SOURCE_URLS` atau tombol **Import Indonesian ASNs** di halaman
  **ASN Whitelist**.
- Dengan `RTBH_WHITELIST_STRICT_ID=true` (default), penambahan manual ditolak
  bila ASN tidak ada di cache registry country-ID RIR. Gunakan `allow_non_id`
  pada request API atau centang **Allow non-Indonesian ASN** di UI untuk
  override eksplisit.
- Monitoring sesi BGP tersedia di halaman **BGP Monitor** pada portal dan
  endpoint Prometheus `GET /metrics`.

Catatan deployment:

1. Hentikan dan disable `bird.service` sebelum memakai engine custom karena
   keduanya memakai TCP/179.
2. Unit systemd `rtbhd.service` sudah menyertakan
   `AmbientCapabilities=CAP_NET_BIND_SERVICE` agar agent non-root bisa bind
   port 179.
3. Telemetri sesi dikirim lewat heartbeat gRPC tiap 15 detik.

Batasan keamanan engine custom:

1. **ASN pada OPEN belum diverifikasi kriptografis.** Auto-accept memakai ASN
   yang diklaim peer. Gunakan firewall/allowlist IP, IRR/RPKI filtering, atau
   pertimbangkan MD5/GTSM jika lingkungan tidak tepercaya.
2. **TCP MD5 dan GTSM belum didukung.** Field password peer diabaikan (agent
   menulis warning di log). Untuk peer yang mewajibkan MD5/TTL security,
   gunakan engine `bird`.
3. **Multihop** tidak mengubah TTL; session tetap mengikuti routing sistem.
   Untuk TTL security eksplisit gunakan BIRD.
4. Batasi akses TCP/179 ke jaringan peering yang sah.

```bash
RTBH_MODE=agent
RTBH_PORTAL_URL=portal.example.com:50051
RTBH_BGP_ENGINE=custom
RTBH_BGP_LISTEN_ADDR=:179
RTBH_BGP_ROUTER_ID=192.0.2.1
RTBH_BGP_NEXTHOP_V4=192.0.2.1
RTBH_LOCAL_ASN=65500
```

---

## REST API Reference

Portal menyediakan REST API di `/api/v1/`.

### Authentication

```http
POST /api/v1/auth/login
Content-Type: application/json

{"username": "admin", "password": "password"}
```

### Status

```http
GET /api/v1/status
```

Response:
```json
{
  "agents": 3,
  "prefixes": 150,
  "sources": 5,
  "excludes": 10,
  "peers": 8,
  "whitelist": 512,
  "auto_accept": true,
  "bgp_sessions": 8,
  "bgp_established": 7,
  "effective_prefixes": 140,
  "local_asn": 65500
}
```

### ASN Whitelist

```http
GET    /api/v1/whitelist
POST   /api/v1/whitelist                  {"asn": 7713, "name": "ISP", "allow_non_id": false}
DELETE /api/v1/whitelist/:id
PUT    /api/v1/whitelist/:id/toggle       {"enabled": true}
PUT    /api/v1/whitelist/auto-accept      {"enabled": true}
POST   /api/v1/whitelist/import           # import daftar ASN Indonesia (APNIC)
```

### BGP Monitoring

```http
GET /api/v1/bgp       # ringkasan + daftar sesi dari semua agent
GET /metrics          # format Prometheus (publik kecuali RTBH_METRICS_TOKEN di-set)
```

Metrik yang tersedia: `rtbhid_bgp_agents`, `rtbhid_bgp_sessions_total`,
`rtbhid_bgp_sessions_established`, `rtbhid_bgp_prefixes_received`,
`rtbhid_bgp_sessions_stale`, `rtbhid_agent_bgp_sessions{agent,engine}`,
`rtbhid_agent_bgp_sessions_established{agent,engine}`,
`rtbhid_agent_bgp_prefixes_received{agent,engine}`,
`rtbhid_bgp_session_up{agent,peer,remote_as,passive}`, dan
`rtbhid_bgp_session_stale{agent,peer}`.

### Prefixes

```http
GET    /api/v1/prefixes
POST   /api/v1/prefixes
PUT    /api/v1/prefixes/:id/toggle
DELETE /api/v1/prefixes/:id
```

### Sources

```http
GET    /api/v1/sources
POST   /api/v1/sources
PUT    /api/v1/sources/:id/toggle
DELETE /api/v1/sources/:id
POST   /api/v1/sources/:id/sync
```

### Excludes

```http
GET    /api/v1/excludes
POST   /api/v1/excludes
DELETE /api/v1/excludes/:id
GET    /api/v1/exclude-sources
POST   /api/v1/exclude-sources
PUT    /api/v1/exclude-sources/:id/toggle
DELETE /api/v1/exclude-sources/:id
POST   /api/v1/exclude-sources/:id/sync
```

### Peers

```http
GET    /api/v1/peers
POST   /api/v1/peers
POST   /api/v1/peers/bulk
PUT    /api/v1/peers/:id/toggle
DELETE /api/v1/peers/:id
```

### Events

```http
GET /api/v1/events?limit=100
```

---

## Development

### Setup Development Environment

```bash
# Backend
go mod download

# Frontend
cd web
npm install
```

### Run Development Server

```bash
# Terminal 1: Portal backend
make portal
./bin/rtbh-portal

# Terminal 2: Frontend dev server (hot reload)
cd web
npm run dev
```

Frontend dev server berjalan di `http://localhost:5173` dengan proxy ke backend di `:8088`.

### Regenerate Protobuf

```bash
# Requires buf (https://buf.build)
make proto
```

---

## Documentation

Portal menyediakan halaman dokumentasi publik di `/docs` yang berisi:

- Konfigurasi untuk upstream providers
- Contoh konfigurasi BGP untuk berbagai platform:
  - BIRD 2/3
  - Juniper Junos
  - Cisco IOS/IOS-XE
  - Arista EOS
  - Huawei VRP
  - MikroTik RouterOS
- BGP communities untuk RTBH
- Security considerations

Akses: `http://your-portal:8088/docs`

---

## Security Considerations

### Portal

1. **Enable Authentication**: Set `RTBH_AUTH_USER` dan `RTBH_AUTH_PASS`
2. **Use Strong Password**: Minimal 16 karakter
3. **Session Security**: Set `RTBH_AUTH_SESSION_SECRET` dengan nilai random yang panjang
4. **HTTPS**: Gunakan reverse proxy dengan TLS
5. **Firewall**: Batasi akses ke port 8088 dan 50051

### Agent

1. **TLS**: Gunakan TLS untuk koneksi gRPC (jika diperlukan)
2. **Firewall**: Batasi akses ke port portal
3. **BIRD Socket Permission**: User `rtbhd` harus punya akses ke BIRD control socket
4. **File Permissions**: Pastikan file output BIRD bisa di-read oleh user `bird`

---

## Troubleshooting

### Agent tidak bisa connect ke portal

```
Error: connection refused
```

**Solusi:**
1. Cek `RTBH_PORTAL_URL` sudah benar
2. Cek firewall membuka port gRPC (default 50051)
3. Cek portal sedang berjalan: `systemctl status rtbh-portal`

### Agent tidak bisa menjalankan birdc

```
Error: permission denied
```

**Solusi:**
1. Tambahkan user `rtbhd` ke group `bird`:
   ```bash
   sudo usermod -a -G bird rtbhd
   ```
2. Restart agent service

### BIRD version tidak terdeteksi

```
birdc detection failed, using env fallback
```

**Solusi:**
1. Cek `RTBH_BIRD_COMMAND` sudah benar dengan socket path
2. Cek user `rtbhd` bisa menjalankan `birdc`:
   ```bash
   sudo -u rtbhd birdc -s /var/run/bird/bird.ctl show status
   ```
3. Set manual: `RTBH_BIRD_VERSION=3.2.0`

### Prefix tidak muncul di BIRD

**Solusi:**
1. Cek file output sudah di-include di `bird.conf`
2. Cek BIRD sudah reload:
   ```bash
   birdc configure
   ```
3. Cek logs agent: `journalctl -u rtbhd -f`

---

## Support

- **Documentation**: `/docs` page on your portal
- **Issues**: GitHub Issues
- **Email**: support@jalabyte.com

---

## License

Copyright © 2026 PT Jala Informatika Nusantara

Coded by [PT Jala Informatika Nusantara](https://jalabyte.com) • Idea from: Rifqi Arief Pamungkas (Rapdodge)
