#!/usr/bin/env bash
# ci-run.sh — build/test a local repo on a disposable instabox CI box, pull the artifacts back.
#
# Usage:
#   ci-run.sh <repo-path> [build-command] [artifact-path-in-box]
#
# Examples:
#   ci-run.sh .                                        # push, no build, no pull
#   ci-run.sh . 'go test ./... && go build -o /data/work/dist/server ./cmd/server' /data/work/dist
#   ci-run.sh ../myproj 'npm ci && npm run build' /data/work/repo/dist
#
# Required env:
#   IBK / INSTABOX_API_KEY   upstream instabox API key (ibk_...) of the box-owning account
#   MONKEY                   monitor API key (ibmon_...) that owns the ci-box setup
# Optional env:
#   MONITOR (default https://instabox.hainh.io.vn)
#   ACCOUNT (monitor account id; default: the first account with a ci-box setup)
#   WORKDIR (default /data/work), STOP=1 to stop the box when finished
#
# Flow: reuse the running box if there is one (or start it) -> push repo ->
# run command over ssh -> pull artifact. The box STAYS ALIVE afterwards so the
# next iteration skips the ~70s boot — pass STOP=1 to end the session, or just
# press Stop in the dashboard. The box dies on its own when the ~7h40 job budget
# runs out.

set -euo pipefail

MONITOR="${MONITOR:-https://instabox.hainh.io.vn}"
WORKDIR="${WORKDIR:-/data/work}"
REPO="${1:-.}"
BUILD_CMD="${2:-}"
ARTIFACT="${3:-}"

IBK="${IBK:-${INSTABOX_API_KEY:-}}"
: "${MONKEY:?set MONKEY to a monitor API key (ibmon_...)}"
: "${IBK:?set IBK or INSTABOX_API_KEY to the upstream instabox API key (ibk_...)}"
export INSTABOX_API_KEY="$IBK"

REPO="$(cd "$REPO" && pwd)"
NAME="$(basename "$REPO")"
log() { printf '\033[36m[ci-run]\033[0m %s\n' "$*"; }
die() { printf '\033[31m[ci-run] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }
command -v instabox >/dev/null || die "instabox CLI not found (install: npx instabox@latest)"
command -v jq >/dev/null || die "jq not found"
command -v curl >/dev/null || die "curl not found"

api() { curl -fsS -H "Authorization: Bearer $MONKEY" "$@"; }

# --- locate the ci-box account -------------------------------------------------
if [ -z "${ACCOUNT:-}" ]; then
  ACCOUNT="$(api "$MONITOR/api/ci-boxes" | jq -r '.ciBoxes[0].accountId // empty')"
  [ -n "$ACCOUNT" ] || die "no ci-box setup found — configure one in the monitor dashboard first"
fi
log "monitor account: $ACCOUNT"

# --- state helpers --------------------------------------------------------------
cibox_state() {
  api "$MONITOR/api/ci-boxes" | jq -r --arg acc "$ACCOUNT" \
    '.ciBoxes[] | select(.accountId==$acc) | "\(.state) \(.machineId // "")"' | head -1
}

MACHINE=""
cleanup() {
  if [ -n "$MACHINE" ] && [ "${STOP:-0}" = "1" ]; then
    log "STOP=1 — stopping box (job cancel)..."
    curl -sS -X POST -H "Authorization: Bearer $MONKEY" \
      "$MONITOR/api/accounts/$ACCOUNT/ci-box/stop" >/dev/null 2>&1 || true
  elif [ -n "$MACHINE" ]; then
    log "box stays alive for the next iteration: $MACHINE (STOP=1 or dashboard Stop to end it)"
  fi
}
trap cleanup EXIT

# --- 1. reuse a running box, or start one -------------------------------------------
read -r STATE MACHINE < <(cibox_state) || true
if [ "$STATE" = "running" ] && [ -n "$MACHINE" ]; then
  log "reusing running box: $MACHINE (no 70s boot wait)"
else
  log "starting ci box..."
  curl -fsS -X POST -H "Authorization: Bearer $MONKEY" \
    "$MONITOR/api/accounts/$ACCOUNT/ci-box/start" >/dev/null

  MACHINE=""
  for i in $(seq 1 20); do
    sleep 10
    read -r STATE MACHINE < <(cibox_state) || true
    log "  [$((i*10))s] state=$STATE"
    [ "$STATE" = "running" ] && break
    [ "$STATE" = "error" ] && die "ci box entered error state"
    [ "$i" = "20" ] && die "timed out waiting for the box (200s)"
  done
fi
[ -n "$MACHINE" ] || die "box is running but no machine id yet — retry in a few seconds"
log "machine up: $MACHINE"

# --- 2. push the repo --------------------------------------------------------------
log "preparing workdir and pushing $NAME..."
timeout 60 instabox ssh "$MACHINE" --no-paste -- \
  "sudo mkdir -p '$WORKDIR' && sudo chown user:user '$WORKDIR'" >/dev/null 2>&1

START=$(date +%s)
instabox cp -r -p "$REPO" "$MACHINE:$WORKDIR/" 2>&1 | grep -v outdated | tail -1
log "push took $(( $(date +%s) - START ))s"

# --- 3. build/test in the box --------------------------------------------------------
RC=0
if [ -n "$BUILD_CMD" ]; then
  log "running in box: $BUILD_CMD"
  START=$(date +%s)
  set +e
  timeout 1800 instabox ssh "$MACHINE" --no-paste -- \
    "cd '$WORKDIR/$NAME' && $BUILD_CMD" 2>&1 | grep -v outdated
  CODES=("${PIPESTATUS[@]}")
  set -e
  RC=${CODES[0]}
  log "build/test took $(( $(date +%s) - START ))s (rc=$RC)"
fi

# --- 4. pull artifacts ---------------------------------------------------------------
if [ -n "$ARTIFACT" ]; then
  if [ $RC -eq 0 ]; then
    OUT="./ci-artifacts/$(date +%Y%m%d-%H%M%S)"
    mkdir -p "$OUT"
    log "pulling $ARTIFACT -> $OUT"
    instabox cp -r "$MACHINE:$ARTIFACT" "$OUT/" 2>&1 | grep -v outdated | tail -1
    log "artifacts in $OUT"
  else
    log "build failed — skipping artifact pull (box output above)"
  fi
fi

exit $RC
