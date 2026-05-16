#!/usr/bin/env bash
#
# check.sh — build each service image and verify its /health endpoint.
#
# Usage:  ./check.sh
#
set -euo pipefail

# service -> host port
SERVICES=(
  "policy-api:3000"
  "claims-service:5000"
  "quote-engine:8080"
  "portal:8000"
)

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "$ROOT"

echo "==> Stopping any previous run"
docker compose down --remove-orphans >/dev/null 2>&1 || true

echo "==> Building all images"
for entry in "${SERVICES[@]}"; do
  svc="${entry%%:*}"
  echo "    - $svc"
  docker build -q -t "devops-practice/$svc" "./services/$svc" >/dev/null
done

echo "==> Bringing services up via docker compose"
docker compose up -d --build

trap 'echo; echo "==> Tearing down"; docker compose down' EXIT

echo "==> Waiting 4s for services to start"
sleep 4

echo "==> Checking /health endpoints"
fail=0
for entry in "${SERVICES[@]}"; do
  svc="${entry%%:*}"
  port="${entry##*:}"
  url="http://localhost:${port}/health"
  printf "    %-16s %s  " "$svc" "$url"
  if curl -fsS --max-time 5 "$url" >/tmp/check-body 2>/dev/null; then
    body="$(cat /tmp/check-body)"
    echo "OK  $body"
  else
    echo "FAIL"
    fail=1
  fi
done

if [[ $fail -ne 0 ]]; then
  echo
  echo "==> One or more health checks failed. Tail logs with:"
  echo "    docker compose logs"
  exit 1
fi

echo
echo "All four /health endpoints returned OK."
