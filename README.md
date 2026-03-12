# RTBHID - Centralized RTBH Controller

`rtbhd` adalah controller RTBH terpusat berbasis Go (single binary) untuk:

- Manajemen prefix manual (add/delete/enable/disable) untuk blackhole.
- Sinkronisasi prefix dari banyak remote URL.
- Pengaturan interval auto-sync berbeda per source.
- Aktivasi/deaktivasi source tanpa menghapus konfigurasi source.
- Export konfigurasi Bird 2/3 secara otomatis.
- Dashboard web built-in untuk monitoring status, source, dan event.

## Arsitektur Singkat

- **Single binary**: API + scheduler + dashboard.
- **State**: JSON file lokal (`RTBH_STATE_PATH`).
- **BGP/RTBH plane**: Bird 2/3 tetap menjadi route daemon. `rtbhd` hanya generate include file route blackhole dan opsional menjalankan `birdc configure`.

## Menjalankan

```bash
go run ./cmd/rtbhd
```

Default:

- Dashboard/API: `http://127.0.0.1:8080`
- State file: `./data/state.json`
- Bird include output: `./data/rtbh_auto.conf`

## Deploy Dengan systemd

File deployment systemd tersedia di:

- `deploy/systemd/rtbhd.service`
- `deploy/systemd/rtbhd.env.example`
- `deploy/systemd/README.md`
- `scripts/install-systemd.sh`

Quick start:

```bash
go build -o rtbhd ./cmd/rtbhd
sudo ./scripts/install-systemd.sh ./rtbhd
```

Lalu edit `/etc/default/rtbhd` sesuai environment produksi Anda.

## Environment Variables

- `RTBH_LISTEN_ADDR` (default `:8080`)
- `RTBH_STATE_PATH` (default `./data/state.json`)
- `RTBH_BIRD_OUTPUT` (default `./data/rtbh_auto.conf`)
- `RTBH_BIRD_RELOAD` (default `false`)
- `RTBH_BIRD_COMMAND` (default `birdc configure`)
- `RTBH_SYNC_USER_AGENT` (default `rtbhd/1.0`)
- `RTBH_SCHEDULER_TICK_SECONDS` (default `15`)
- `RTBH_AUTH_USER` (default empty, auth disabled)
- `RTBH_AUTH_PASS` (default empty, auth disabled)
- `RTBH_AUTH_SESSION_SECRET` (default empty, random at startup)
- `RTBH_AUTH_SESSION_TTL_HOURS` (default `24`)
- `RTBH_AUTH_COOKIE_SECURE` (default `false`)

Contoh enable auto-reload Bird:

```bash
export RTBH_BIRD_RELOAD=true
export RTBH_BIRD_COMMAND='birdc -s /run/bird/bird.ctl configure'
go run ./cmd/rtbhd
```

## Integrasi Bird 2/3

Tambahkan include pada konfigurasi utama Bird:

```bird
include "/absolute/path/to/data/rtbh_auto.conf";
```

Lalu pastikan ada sesi BGP untuk announce komunitas RTBH sesuai policy jaringan Anda.

Lihat contoh minimal: [examples/bird.conf.snippet](examples/bird.conf.snippet)

## Login Dashboard/API

`rtbhd` mendukung halaman login built-in pada `/login` (session cookie).

Aktifkan dengan set environment:

```bash
export RTBH_AUTH_USER=admin
export RTBH_AUTH_PASS='ganti-password-kuat'
```

Opsional:

- `RTBH_AUTH_SESSION_SECRET` untuk secret statik lintas restart.
- `RTBH_AUTH_COOKIE_SECURE=true` jika service diakses via HTTPS.

## API Ringkas

- `GET /api/v1/health`
- `POST /api/v1/auth/login` body `{ "username": "...", "password": "..." }`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `GET /api/v1/status`
- `GET /api/v1/events?limit=120`
- `GET /api/v1/prefixes`
- `POST /api/v1/prefixes` body `{ "prefix": "203.0.113.1", "enabled": true }`
- `DELETE /api/v1/prefixes/{prefix}`
- `GET /api/v1/sources`
- `POST /api/v1/sources`
- `PATCH /api/v1/sources/{id}`
- `POST /api/v1/sources/{id}/sync`
- `DELETE /api/v1/sources/{id}`

## Format Remote List

List remote dapat berupa teks sederhana, satu prefix/IP per baris, atau dipisah spasi/koma.

Contoh:

```text
# komentar
203.0.113.5
203.0.113.0/24
2001:db8::/32
```

IP tunggal otomatis dinormalisasi menjadi `/32` (IPv4) atau `/128` (IPv6).

## Catatan Produksi

- Jalankan `rtbhd` sebagai service (systemd/supervisor/container).
- Disarankan tetap gunakan reverse proxy (TLS, allow-list, rate limit) di depan dashboard/API.
- Pastikan file output Bird berada di path yang bisa di-include Bird.
- Aktifkan `RTBH_BIRD_RELOAD=true` hanya jika `birdc` tersedia di host.
