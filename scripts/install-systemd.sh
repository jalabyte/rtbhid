#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SERVICE_SRC="$REPO_ROOT/deploy/systemd/rtbhd.service"
ENV_EXAMPLE_SRC="$REPO_ROOT/deploy/systemd/rtbhd.env.example"
SERVICE_DST="/etc/systemd/system/rtbhd.service"
ENV_DST="/etc/default/rtbhd"
BIN_DST="/usr/local/bin/rtbhd"
BIN_SRC="${1:-$REPO_ROOT/rtbhd}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root: sudo $0 [path-to-rtbhd-binary]"
  exit 1
fi

if [[ ! -f "$SERVICE_SRC" ]]; then
  echo "Missing service source: $SERVICE_SRC"
  exit 1
fi

if [[ ! -x "$BIN_SRC" ]]; then
  echo "Binary not found/executable: $BIN_SRC"
  echo "Build first: go build -o rtbhd ./cmd/rtbhd"
  exit 1
fi

NOLOGIN_BIN="$(command -v nologin || true)"
if [[ -z "$NOLOGIN_BIN" ]]; then
  if [[ -x /usr/sbin/nologin ]]; then
    NOLOGIN_BIN="/usr/sbin/nologin"
  elif [[ -x /sbin/nologin ]]; then
    NOLOGIN_BIN="/sbin/nologin"
  else
    NOLOGIN_BIN="/bin/false"
  fi
fi

if ! id -u rtbhd >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/rtbhd --shell "$NOLOGIN_BIN" rtbhd
fi

install -d -m 0750 -o rtbhd -g rtbhd /var/lib/rtbhd
install -m 0755 "$BIN_SRC" "$BIN_DST"
install -m 0644 "$SERVICE_SRC" "$SERVICE_DST"

if [[ ! -f "$ENV_DST" ]]; then
  install -m 0644 "$ENV_EXAMPLE_SRC" "$ENV_DST"
  echo "Created $ENV_DST from template. Review this file before production use."
else
  echo "$ENV_DST already exists. Keeping existing values."
fi

systemctl daemon-reload
systemctl enable --now rtbhd
systemctl status --no-pager rtbhd || true

echo
echo "rtbhd installed. Useful commands:"
echo "  systemctl status rtbhd"
echo "  journalctl -u rtbhd -f"
echo "  systemctl restart rtbhd"
