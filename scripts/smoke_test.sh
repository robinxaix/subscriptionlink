#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

LISTEN_ADDR="${LISTEN_ADDR:-127.0.0.1:18081}"
BASE_URL="${BASE_URL:-http://$LISTEN_ADDR}"
ADMIN_TOKEN="${ADMIN_TOKEN:-smoke-admin-token}"
GOCACHE_DIR="${GOCACHE_DIR:-$ROOT_DIR/.gocache}"
USER_FILE="data/users.json"
NODE_FILE="data/nodes.json"
COOKIE_JAR="$(mktemp)"

for cmd in go curl base64; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing command: $cmd" >&2
    exit 1
  fi
done

mkdir -p data
mkdir -p "$GOCACHE_DIR"
USER_BAK="$(mktemp)"
NODE_BAK="$(mktemp)"

if [ -f "$USER_FILE" ]; then cp "$USER_FILE" "$USER_BAK"; else echo '[]' > "$USER_BAK"; fi
if [ -f "$NODE_FILE" ]; then cp "$NODE_FILE" "$NODE_BAK"; else echo '[]' > "$NODE_BAK"; fi

SERVER_PID=""
cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  cp "$USER_BAK" "$USER_FILE"
  cp "$NODE_BAK" "$NODE_FILE"
  rm -f "$USER_BAK" "$NODE_BAK" "$COOKIE_JAR"
}
trap cleanup EXIT

echo '[]' > "$USER_FILE"
echo '[]' > "$NODE_FILE"

if curl -sS "$BASE_URL/" >/dev/null 2>&1; then
  echo "base url already reachable: $BASE_URL (port likely in use), stop existing service first" >&2
  exit 1
fi

ADMIN_TOKEN="$ADMIN_TOKEN" LISTEN_ADDR="$LISTEN_ADDR" GOCACHE="$GOCACHE_DIR" go run ./cmd/server >/tmp/subscriptionlink-smoke.log 2>&1 &
SERVER_PID=$!

sleep 0.2
if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
  echo "server process exited early, check /tmp/subscriptionlink-smoke.log" >&2
  exit 1
fi

for _ in $(seq 1 40); do
  if curl -sS "$BASE_URL/" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

if ! curl -sS "$BASE_URL/" >/dev/null 2>&1; then
  echo "server did not start, check /tmp/subscriptionlink-smoke.log" >&2
  exit 1
fi

echo "[1/7] login admin session"
login_json="$(curl -fsS -X POST "$BASE_URL/api/admin/login" \
  -c "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"$ADMIN_TOKEN\"}")"
csrf_token="$(printf '%s' "$login_json" | sed -n 's/.*"csrf_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "$csrf_token" ]; then
  echo "failed to parse csrf_token from response: $login_json" >&2
  exit 1
fi

echo "[2/7] create user"
user_json="$(curl -fsS -X POST "$BASE_URL/api/admin/users" \
  -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf_token" \
  -H 'Content-Type: application/json' \
  -d '{"name":"smoke-user","uuid":"17c2b5b7-5ff8"}')"

token="$(printf '%s' "$user_json" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "$token" ]; then
  echo "failed to parse token from response: $user_json" >&2
  exit 1
fi

echo "[3/7] create node"
curl -fsS -X POST "$BASE_URL/api/admin/nodes" \
  -b "$COOKIE_JAR" \
  -H "X-CSRF-Token: $csrf_token" \
  -H 'Content-Type: application/json' \
  -d '{"name":"node-a","server":"example.com","port":1234,"protocol":"vless","network":"ws","security":"none","path":"/xhttp"}' >/dev/null

echo "[4/7] verify subscription yaml"
subscription_text="$(curl -fsS "$BASE_URL/api/subscription/$token")"
printf '%s' "$subscription_text" | grep -q 'type: vless'
printf '%s' "$subscription_text" | grep -q 'path: /xhttp'

echo "[5/7] verify singbox and v2ray"
singbox_text="$(curl -fsS "$BASE_URL/api/singbox/$token")"
printf '%s' "$singbox_text" | grep -q '^vless://'

v2ray_b64="$(curl -fsS "$BASE_URL/api/v2ray/$token")"
v2ray_text="$(printf '%s' "$v2ray_b64" | base64 -d 2>/dev/null || printf '%s' "$v2ray_b64" | base64 --decode)"
printf '%s' "$v2ray_text" | grep -q '^vless://'

echo "[6/7] verify stats"
stats_json="$(curl -fsS "$BASE_URL/api/admin/stats" -b "$COOKIE_JAR")"
printf '%s' "$stats_json" | grep -q '"request_count"'
printf '%s' "$stats_json" | grep -q '"subscription"'

echo "[7/7] verify admin auth"
status="$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/api/admin/users")"
if [ "$status" != "401" ]; then
  echo "expected 401 without session cookie, got $status" >&2
  exit 1
fi

echo "smoke test passed"
