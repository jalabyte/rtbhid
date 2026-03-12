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
include "/var/lib/rtbhd/rtbh_peers_auto.conf";
```

Pastikan Bird bisa membaca file include tersebut.

Jika muncul error `Permission denied` saat `bird` membaca include file:

```bash
sudo chown rtbhd:bird /var/lib/rtbhd
sudo chmod 2750 /var/lib/rtbhd
sudo chgrp bird /var/lib/rtbhd/rtbh_auto.conf /var/lib/rtbhd/rtbh_peers_auto.conf
sudo chmod 0640 /var/lib/rtbhd/rtbh_auto.conf /var/lib/rtbhd/rtbh_peers_auto.conf
sudo systemctl restart rtbhd
sudo birdc configure
```

Jika `rtbhd` tidak bisa akses socket `bird.ctl`, cek path socket yang benar:

```bash
ls -l /run/bird/bird.ctl /run/bird.ctl 2>/dev/null
```

Lalu sesuaikan `RTBH_BIRD_COMMAND` di `/etc/default/rtbhd`, contoh:

```bash
RTBH_BIRD_COMMAND=birdc -s /run/bird/bird.ctl configure
# atau
RTBH_BIRD_COMMAND=birdc -s /run/bird.ctl configure
```

Verifikasi dari sisi user service:

```bash
sudo -u rtbhd birdc -s /run/bird/bird.ctl show status
```

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
