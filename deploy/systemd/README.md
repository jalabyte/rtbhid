# systemd Deployment

Panduan ini untuk host Linux dengan `systemd`.

## 1) Build binary

```bash
go build -o rtbhd ./cmd/rtbhd
```

## 2) Install service

```bash
sudo ./scripts/install-systemd.sh ./rtbhd
```

Script akan:

- membuat user system `rtbhd` (jika belum ada)
- membuat `/var/lib/rtbhd`
- menyalin binary ke `/usr/local/bin/rtbhd`
- menyalin unit file ke `/etc/systemd/system/rtbhd.service`
- membuat `/etc/default/rtbhd` dari template (jika belum ada)
- enable + start service

## 3) Edit environment

Edit `/etc/default/rtbhd` lalu restart:

```bash
sudo systemctl restart rtbhd
```

Penting:

- set `RTBH_AUTH_USER` dan `RTBH_AUTH_PASS` untuk mengaktifkan login dashboard/API.
- jika endpoint dipublish HTTPS langsung, set `RTBH_AUTH_COOKIE_SECURE=true`.

## 4) Konfigurasi Bird

Tambahkan pada `bird.conf`:

```bird
include "/var/lib/rtbhd/rtbh_auto.conf";
```

Pastikan Bird bisa membaca file include tersebut.

## Operasional Harian

```bash
sudo systemctl status rtbhd
sudo journalctl -u rtbhd -f
sudo systemctl restart rtbhd
```

## Upgrade Binary

```bash
go build -o rtbhd ./cmd/rtbhd
sudo install -m 0755 ./rtbhd /usr/local/bin/rtbhd
sudo systemctl restart rtbhd
```
