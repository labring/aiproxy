#!/usr/bin/env bash
#
# WireGuard tunnel health check for overseas nodes.
# Checks tunnel connectivity and DB reachability, auto-restarts on failure,
# sends Feishu alerts if recovery fails.
#
# Usage:
#   bash scripts/wireguard-health.sh
#
# Crontab (run every minute):
#   */1 * * * * bash /data/aiproxy/scripts/wireguard-health.sh
#
# Environment:
#   PEER_IP           — WireGuard peer IP (default: 10.0.0.1)
#   DB_HOST           — Database host to check (default: same as PEER_IP)
#   DB_USER           — Database user (default: postgres)
#   DB_NAME           — Database name (default: aiproxy)
#   FEISHU_WEBHOOK    — Feishu webhook URL for alerts (optional)
#   WG_INTERFACE      — WireGuard interface name (default: wg0)

set -euo pipefail

# Prevent overlapping runs — the recovery wait loop (up to 70s) can outlast
# the 1-minute cron interval. flock ensures only one instance runs at a time.
LOCK_FILE="/tmp/wireguard-health.lock"
exec 200>"${LOCK_FILE}"
if ! flock -n 200; then
  echo "$(date -Iseconds) Another instance running, skipping" >> /var/log/wireguard-health.log 2>/dev/null || true
  exit 0
fi

PEER_IP="${PEER_IP:-10.0.0.1}"
DB_HOST="${DB_HOST:-${PEER_IP}}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-aiproxy}"
FEISHU_WEBHOOK="${FEISHU_WEBHOOK:-}"
WG_INTERFACE="${WG_INTERFACE:-wg0}"
HOSTNAME=$(hostname)

send_alert() {
  local msg="$1"
  if [[ -n "${FEISHU_WEBHOOK}" ]]; then
    # Escape JSON special characters to prevent malformed payloads
    local escaped="${msg//\\/\\\\}"
    escaped="${escaped//\"/\\\"}"
    escaped="${escaped//$'\n'/\\n}"
    curl -sf -X POST "${FEISHU_WEBHOOK}" \
      -H 'Content-Type: application/json' \
      -d "{\"msg_type\":\"text\",\"content\":{\"text\":\"${escaped}\"}}" \
      >/dev/null 2>&1 || true
  fi
  echo "[ALERT] ${msg}" >&2
}

# ── Check 1: WireGuard tunnel ───────────────────────────────
if ! ping -c 1 -W 3 "${PEER_IP}" >/dev/null 2>&1; then
  echo "WireGuard tunnel down — attempting restart..."
  sudo systemctl restart "wg-quick@${WG_INTERFACE}" 2>/dev/null || true
  sleep 3

  if ! ping -c 1 -W 3 "${PEER_IP}" >/dev/null 2>&1; then
    send_alert "[${HOSTNAME}] WireGuard tunnel to ${PEER_IP} DOWN — auto-restart failed. Manual intervention required."
    exit 1
  fi
  echo "WireGuard tunnel recovered after restart."
fi

# ── Check 2: Database connectivity ──────────────────────────
DB_REACHABLE=1
if command -v psql >/dev/null 2>&1; then
  if ! psql -h "${DB_HOST}" -U "${DB_USER}" -d "${DB_NAME}" -c "SELECT 1" >/dev/null 2>&1; then
    DB_REACHABLE=0
    send_alert "[${HOSTNAME}] Cannot connect to PostgreSQL at ${DB_HOST} — WireGuard is up but DB unreachable."
  fi
else
  # Fallback: TCP check on PostgreSQL port
  if ! timeout 3 bash -c "echo >/dev/tcp/${DB_HOST}/5432" 2>/dev/null; then
    DB_REACHABLE=0
    send_alert "[${HOSTNAME}] PostgreSQL port 5432 unreachable at ${DB_HOST} (TCP check)."
  fi
fi

# ── Check 3: Restart aiproxy if DB was down then recovered ──
# When WireGuard restarts, Go's connection pool may hold dead connections.
# Restart the app container to force fresh DB connections.
RECOVERY_FLAG="/tmp/wireguard-db-down"

if [[ "${DB_REACHABLE}" == "0" ]]; then
  touch "${RECOVERY_FLAG}"
  echo "$(date -Iseconds) DB unreachable" >> /var/log/wireguard-health.log 2>/dev/null || true
  exit 1
fi

if [[ -f "${RECOVERY_FLAG}" ]]; then
  rm -f "${RECOVERY_FLAG}"
  echo "$(date -Iseconds) DB recovered — waiting for connection pool to auto-recover" >> /var/log/wireguard-health.log 2>/dev/null || true

  # GORM uses database/sql connection pool with ConnMaxLifetime=60s.
  # Stale connections are recycled on next use — new connections will reach the recovered DB.
  # /api/health does a real DB ping (unlike /api/status which is always 200).
  # Wait up to 70s (> ConnMaxLifetime) checking every 10s.
  RECOVERED=0
  ACTIVE_PORT=$(cat /data/aiproxy/.active-port 2>/dev/null || echo "3000")
  for _attempt in 1 2 3 4 5 6 7; do
    sleep 10
    if curl -sf "http://localhost:${ACTIVE_PORT}/api/health" >/dev/null 2>&1; then
      RECOVERED=1
      break
    fi
  done

  if [[ "${RECOVERED}" == "1" ]]; then
    echo "$(date -Iseconds) Connection pool auto-recovered, no restart needed" >> /var/log/wireguard-health.log 2>/dev/null || true
    send_alert "[${HOSTNAME}] WireGuard/DB recovered — connection pool auto-recovered, no restart needed."
  else
    echo "$(date -Iseconds) Connection pool did NOT recover after 70s — restarting container" >> /var/log/wireguard-health.log 2>/dev/null || true
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^aiproxy-active$'; then
      send_alert "[${HOSTNAME}] ⚠️ WireGuard/DB recovered but connection pool stuck after 70s — restarting container (will cause ~10s downtime)"
      docker restart aiproxy-active
    fi
  fi
fi

echo "$(date -Iseconds) OK" >> /var/log/wireguard-health.log 2>/dev/null || true
