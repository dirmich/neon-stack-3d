#!/bin/sh
set -e

# 1. Referee (Rust) 백그라운드 실행 (포트 8081)
PORT=8081 /usr/local/bin/neon-referee &
REFEREE_PID=$!

cleanup() {
    kill -TERM "$REFEREE_PID" "$GATEWAY_PID" 2>/dev/null || true
}
trap cleanup EXIT TERM INT

# 2. Gateway (Go) 실행 (기본 포트 8000)
export REFEREE_URL="${REFEREE_URL:-http://127.0.0.1:8081}"
export PORT="${PORT:-8000}"
/usr/local/bin/gateway &
GATEWAY_PID=$!

wait "$GATEWAY_PID"
