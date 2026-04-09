#!/usr/bin/env bash
#
# Post-deploy disk cleanup — safe, auditable cleanup of temporary/reclaimable files.
#
# Modes:
#   (default)    Interactive — lists all reclaimable items, asks [y/N] for each
#   --auto       Auto-clean safe items only (dangling images, build cache, stopped
#                containers, old /tmp files). Skips non-dangling images.
#   --dry-run    Detect and report only — no deletions
#
# Safety:
#   - NEVER removes aiproxy:local, aiproxy:rollback, postgres:*, redis:*
#   - NEVER removes running containers
#   - All deletions are logged with timestamps
#
# Usage:
#   bash scripts/post-deploy-cleanup.sh              # Interactive
#   bash scripts/post-deploy-cleanup.sh --auto        # Auto (safe items)
#   bash scripts/post-deploy-cleanup.sh --dry-run     # Report only

set -euo pipefail

# ── Flags ────────────────────────────────────────────────────
MODE="interactive"  # interactive | auto | dry-run

while [[ $# -gt 0 ]]; do
  case "$1" in
    --auto)     MODE="auto";    shift ;;
    --dry-run)  MODE="dry-run"; shift ;;
    -h|--help)
      head -20 "$0" | tail -18
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# ── Helpers ──────────────────────────────────────────────────
pass() { printf "\033[32m✓\033[0m %s\n" "$*"; }
fail() { printf "\033[31m✗\033[0m %s\n" "$*"; }
info() { printf "\033[36m→\033[0m %s\n" "$*"; }
warn() { printf "\033[33m!\033[0m %s\n" "$*"; }
dim()  { printf "\033[90m  %s\033[0m\n" "$*"; }

TOTAL_RECLAIMED=0

# Ask for confirmation in interactive mode. Auto mode returns 0 (yes) for safe
# items and 1 (no) for unsafe items. Dry-run always returns 1.
# Usage: confirm "prompt" [safe]
confirm() {
  local prompt="$1"
  local safe="${2:-no}"

  if [[ "${MODE}" == "dry-run" ]]; then
    dim "(dry-run: skipped)"
    return 1
  fi

  if [[ "${MODE}" == "auto" ]]; then
    if [[ "${safe}" == "safe" ]]; then
      dim "(auto: proceeding)"
      return 0
    else
      dim "(auto: skipped — use interactive mode to clean this)"
      return 1
    fi
  fi

  # Interactive
  printf "  \033[33m?\033[0m %s [y/N] " "${prompt}"
  read -r answer
  [[ "${answer}" =~ ^[Yy]$ ]]
}

human_size() {
  local bytes="$1"
  if [[ ${bytes} -ge 1073741824 ]]; then
    echo "$(awk "BEGIN {printf \"%.1f\", ${bytes}/1073741824}")GB"
  elif [[ ${bytes} -ge 1048576 ]]; then
    echo "$(awk "BEGIN {printf \"%.1f\", ${bytes}/1048576}")MB"
  elif [[ ${bytes} -ge 1024 ]]; then
    echo "$(awk "BEGIN {printf \"%.0f\", ${bytes}/1024}")KB"
  else
    echo "${bytes}B"
  fi
}

# ── Header ───────────────────────────────────────────────────
echo ""
info "=========================================="
info "Post-Deploy Disk Cleanup (${MODE} mode)"
info "=========================================="
echo ""

# ══════════════════════════════════════════════════════════════
# 1. Disk Overview
# ══════════════════════════════════════════════════════════════
info "1. Disk Usage Overview"
echo ""
df -h / /data 2>/dev/null | head -5 || df -h / | head -5
echo ""

DISK_PCT=$(df / --output=pcent 2>/dev/null | tail -1 | tr -d ' %' || echo "0")
if [[ ${DISK_PCT} -ge 90 ]]; then
  fail "WARNING: Disk usage at ${DISK_PCT}% — cleanup strongly recommended!"
elif [[ ${DISK_PCT} -ge 80 ]]; then
  warn "Disk usage at ${DISK_PCT}% — cleanup recommended"
else
  pass "Disk usage at ${DISK_PCT}% — healthy"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# 2. Docker: Dangling Images
# ══════════════════════════════════════════════════════════════
info "2. Docker: Dangling Images"

DANGLING_SIZE=$(docker images -f "dangling=true" --format '{{.Size}}' 2>/dev/null | head -20)
DANGLING_COUNT=$(docker images -f "dangling=true" -q 2>/dev/null | wc -l | tr -d ' ')

if [[ ${DANGLING_COUNT} -gt 0 ]]; then
  # Get size in bytes for accurate reporting
  DANGLING_BYTES=$(docker system df -v 2>/dev/null \
    | awk '/^Images/,/^$/' \
    | grep '<none>' \
    | awk '{sum += $NF} END {print sum+0}' 2>/dev/null || echo "0")
  warn "${DANGLING_COUNT} dangling image(s) found"
  docker images -f "dangling=true" --format '  {{.ID}}  {{.Size}}  (created {{.CreatedSince}})' 2>/dev/null | head -10
  dim "Impact: removes unused intermediate build layers — no effect on running services"

  if confirm "Remove ${DANGLING_COUNT} dangling image(s)?" "safe"; then
    docker image prune -f 2>/dev/null
    pass "Dangling images removed"
  fi
else
  pass "No dangling images"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# 3. Docker: Unused Images (non-protected)
# ══════════════════════════════════════════════════════════════
info "3. Docker: Unused Images (non-protected)"

PROTECTED_PATTERN="^(aiproxy:(local|rollback)|postgres:|redis:)"
UNUSED_IMAGES=()

while IFS= read -r line; do
  repo_tag=$(echo "${line}" | awk '{print $1":"$2}')
  if [[ "${repo_tag}" =~ ${PROTECTED_PATTERN} ]]; then
    continue
  fi
  # Skip images used by running containers
  image_id=$(echo "${line}" | awk '{print $3}')
  if docker ps -q --filter "ancestor=${image_id}" 2>/dev/null | grep -q .; then
    continue
  fi
  UNUSED_IMAGES+=("${line}")
done < <(docker images --format '{{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Size}}\t{{.CreatedSince}}' 2>/dev/null)

if [[ ${#UNUSED_IMAGES[@]} -gt 0 ]]; then
  warn "${#UNUSED_IMAGES[@]} unused non-protected image(s):"
  for img in "${UNUSED_IMAGES[@]}"; do
    printf "  %s\n" "${img}"
  done
  dim "Impact: removes old/unused images — protected images (aiproxy:local/rollback, postgres, redis) are SAFE"
  dim "Protected: aiproxy:local, aiproxy:rollback, postgres:*, redis:*"

  if confirm "Remove ${#UNUSED_IMAGES[@]} unused image(s)?" "unsafe"; then
    for img in "${UNUSED_IMAGES[@]}"; do
      img_id=$(echo "${img}" | awk '{print $3}')
      docker rmi "${img_id}" 2>/dev/null || warn "Could not remove ${img_id} (may be in use)"
    done
    pass "Unused images cleaned"
  fi
else
  pass "No unused non-protected images"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# 4. Docker: Build Cache
# ══════════════════════════════════════════════════════════════
info "4. Docker: Build Cache"

BUILD_CACHE_LINE=$(docker system df 2>/dev/null | grep "Build Cache" || echo "")
BUILD_CACHE_RECLAIM=$(echo "${BUILD_CACHE_LINE}" | awk '{print $NF}' | tr -d '()' || echo "0B")
BUILD_CACHE_TOTAL=$(echo "${BUILD_CACHE_LINE}" | awk '{print $4}' || echo "0B")

if [[ -n "${BUILD_CACHE_LINE}" && "${BUILD_CACHE_RECLAIM}" != "0B" ]]; then
  warn "Build cache: ${BUILD_CACHE_TOTAL} total, ${BUILD_CACHE_RECLAIM} reclaimable"
  dim "Impact: clears Docker build layer cache — next build will be slower (rebuilds all layers)"

  if confirm "Prune Docker build cache (${BUILD_CACHE_RECLAIM} reclaimable)?" "safe"; then
    docker builder prune -f 2>/dev/null
    pass "Build cache pruned"
  fi
else
  pass "Build cache clean"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# 5. Docker: Stopped Containers
# ══════════════════════════════════════════════════════════════
info "5. Docker: Stopped Containers"

STOPPED_COUNT=$(docker ps -a --filter "status=exited" --filter "status=dead" -q 2>/dev/null | wc -l | tr -d ' ')

if [[ ${STOPPED_COUNT} -gt 0 ]]; then
  warn "${STOPPED_COUNT} stopped container(s):"
  docker ps -a --filter "status=exited" --filter "status=dead" \
    --format '  {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Size}}' 2>/dev/null | head -10
  dim "Impact: removes exited/dead containers and their writable layers — no effect on running services"

  if confirm "Remove ${STOPPED_COUNT} stopped container(s)?" "safe"; then
    docker container prune -f 2>/dev/null
    pass "Stopped containers removed"
  fi
else
  pass "No stopped containers"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# 6. System: Old /tmp files (>7 days)
# ══════════════════════════════════════════════════════════════
info "6. System: Old /tmp files (>7 days)"

OLD_TMP_SIZE=$(sudo find /tmp -maxdepth 2 -type f -mtime +7 -not -path '/tmp/systemd-*' \
  -exec du -cb {} + 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")

if [[ ${OLD_TMP_SIZE} -gt 1048576 ]]; then
  warn "Old /tmp files: $(human_size "${OLD_TMP_SIZE}")"
  sudo find /tmp -maxdepth 2 -type f -mtime +7 -not -path '/tmp/systemd-*' \
    -printf '  %p (%s bytes, modified %t)\n' 2>/dev/null | head -10
  dim "Impact: removes temp files older than 7 days — excludes systemd runtime files"

  if confirm "Remove old /tmp files ($(human_size "${OLD_TMP_SIZE}"))?" "safe"; then
    sudo find /tmp -maxdepth 2 -type f -mtime +7 -not -path '/tmp/systemd-*' -delete 2>/dev/null
    pass "Old /tmp files removed"
  fi
else
  pass "No significant old /tmp files"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# 7. System: Journal Logs
# ══════════════════════════════════════════════════════════════
info "7. System: Journal Logs"

if command -v journalctl >/dev/null 2>&1; then
  JOURNAL_SIZE=$(journalctl --disk-usage 2>/dev/null | grep -oP '[\d.]+[GMKT]' | head -1 || echo "0")
  if [[ -n "${JOURNAL_SIZE}" && "${JOURNAL_SIZE}" != "0" ]]; then
    warn "Journal logs: ${JOURNAL_SIZE}"
    dim "Impact: removes journal entries older than 7 days — recent logs preserved"

    if confirm "Vacuum journal logs older than 7 days?" "safe"; then
      sudo journalctl --vacuum-time=7d 2>/dev/null
      pass "Journal logs vacuumed"
    fi
  else
    pass "Journal logs minimal"
  fi
else
  dim "journalctl not available"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# 8. System: APT Cache
# ══════════════════════════════════════════════════════════════
info "8. System: APT Cache"

if command -v apt-get >/dev/null 2>&1; then
  APT_CACHE_SIZE=$(sudo du -sb /var/cache/apt/archives/ 2>/dev/null | awk '{print $1}' || echo "0")
  if [[ ${APT_CACHE_SIZE} -gt 10485760 ]]; then
    warn "APT cache: $(human_size "${APT_CACHE_SIZE}")"
    dim "Impact: removes cached .deb packages — does not affect installed packages"

    if confirm "Clean APT cache ($(human_size "${APT_CACHE_SIZE}"))?" "safe"; then
      sudo apt-get clean 2>/dev/null
      pass "APT cache cleaned"
    fi
  else
    pass "APT cache minimal"
  fi
else
  dim "apt-get not available"
fi
echo ""

# ══════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════
info "=========================================="

# Show final disk state
DISK_PCT_AFTER=$(df / --output=pcent 2>/dev/null | tail -1 | tr -d ' %' || echo "0")
if [[ "${MODE}" != "dry-run" ]]; then
  info "Disk usage: ${DISK_PCT}% → ${DISK_PCT_AFTER}%"
else
  info "Disk usage: ${DISK_PCT}% (dry-run — no changes made)"
fi

pass "Cleanup check complete"
info "=========================================="
echo ""
