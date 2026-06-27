# GoLite guest — the `GOOS=hyperlight` side

This directory documents the guest half of GoLite: the Go function compiled to
run **directly on the vCPU** of a Hyperlight partition, with **no OS and no
syscalls** underneath it. The runnable, executable model of this contract lives
in the Go simulation (`../../guest`, `../../examples`); this README explains how
that contract is realised on real hardware and why building it is the larger
engineering effort.

## What the guest is

A single statically-linked image:

```
standard Go runtime  +  runtime/goos overlay (GOOS=hyperlight)  +  guest-ABI shim  +  user code
```

This is exactly the structure TamaGo already ships for `GOOS=tamago`, retargeted
from generic KVM boards onto Hyperlight's guest memory layout and I/O-port
signalling.

## The overlay: bare-metal hooks the runtime needs

The standard Go runtime assumes an OS. The overlay supplies the small set of
hooks that replace it — the same set TamaGo's `runtime/goos` defines:

| Hook | Replaces the syscall | GoLite implementation |
|------|----------------------|------------------------|
| `RamStart` / `RamSize` / `RamStackOffset` | `mmap` of the heap/stack | fixed offsets into the partition's linear memory (the `arena` in the sim) |
| `Nanotime` | `clock_gettime` | host-injected clock via the ABI |
| `GetRandomData` / `InitRNG` | `getrandom` | host-seeded entropy injected on every restore (the sim's reseed step) |
| `Printk` | `write(2, …)` | `golite_log` host function |
| `Idle` | `futex` wait | `hlt` until the next host call |
| `Exit` | `exit_group` | halt the vCPU |

No `clone`, no `epoll`, no `rt_sigaction`: the scheduler runs on the vCPU(s) and
preemption uses the runtime's own mechanism (TamaGo's `WakeG`), not OS signals.

## The handler export: `//go:golite_handler`

The user marks one function, analogous to `//go:wasmexport`:

```go
//go:golite_handler
func Handle(input []byte) ([]byte, error) {
    // pure compute + stdlib; reaches the host only through the ABI shim
}
```

The toolchain emits the entry the host calls as `golite_handle`, and the shim
wires `input`/output through the comms region (see the host harness'
`golite_write_output`).

## The host ABI (guest → host)

The only registered host functions — the guest's entire outside world. These are
the real-Hyperlight counterparts of the Go sim's `abi.Host`:

- `golite_read_input() []byte`
- `golite_write_output([]byte)`
- `golite_log(string)`            (capability-gated)
- `golite_now() int64`            (capability-gated)
- `golite_entropy() []byte`       (host-seeded on restore)
- `golite_net(req []byte) []byte` (capability-gated, allow-listed, off by default)

## Why this isn't built end-to-end here

1. **No hypervisor in this container** (`/dev/kvm` is absent), so a real guest
   could not execute even if built.
2. **A `GOOS=hyperlight` port is a real toolchain effort.** The honest path is to
   start from TamaGo's `GOOS=tamago` (which already runs the *full* Go runtime on
   Cloud Hypervisor / Firecracker / QEMU microvm) and retarget its overlay onto
   Hyperlight's memory layout and I/O-port ABI — moderate work, plus ongoing
   maintenance across Go releases (tracked upstream by the `GOOS=none` proposal,
   golang/go#73608).

The simulation in the parent directory implements and **tests** the same
contract — handler signature, host-ABI-only I/O, snapshot/restore, entropy
reseed, capability gating — so the safety properties are demonstrated and
asserted even though the bare-metal port is out of scope for a PoC.

## Building it for real (the recipe)

```bash
# 1. Get a GOOS=hyperlight toolchain (fork of TamaGo's tamago-go):
#    src/runtime/goos/ overlay retargeted to Hyperlight.
# 2. Compile the user function to a static guest image:
GOOS=hyperlight GOARCH=amd64 go build \
    -ldflags "-T 0x10010000 -R 0x1000" \
    -o golite-guest ./fn
# 3. Run it under the host harness on a KVM-enabled machine:
cargo run --manifest-path ../host/Cargo.toml -- ./golite-guest
```
