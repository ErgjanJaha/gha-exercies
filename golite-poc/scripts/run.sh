#!/usr/bin/env bash
# GoLite PoC runner: runs the executable Go simulation, then attempts the real
# Hyperlight host harness (which self-gates on hardware virtualization).
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==================================================================="
echo " GoLite PoC — part 1: executable lifecycle simulation (pure Go)"
echo "==================================================================="
go test ./...
echo
go run ./cmd/golite

echo
echo "==================================================================="
echo " GoLite PoC — part 2: real Hyperlight host (KVM/MSHV/WHP gated)"
echo "==================================================================="
if [[ -e /dev/kvm ]]; then
  echo "/dev/kvm present — building and running the real host harness."
  cargo run --manifest-path hyperlight/host/Cargo.toml -- "${1:-}"
else
  echo "/dev/kvm absent in this environment — the micro-VM cannot be created here."
  echo "Type-checking the host against the real hyperlight-host 0.16 crate instead:"
  cargo build --manifest-path hyperlight/host/Cargo.toml
  echo "OK: host harness compiles against the real Hyperlight API."
fi
