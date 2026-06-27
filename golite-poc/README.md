# GoLite — proof of concept

A proof of concept for **GoLite**: a Go-only micro-VM architecture that boots the
*standard* Go runtime directly on a Hyperlight-style KVM/MSHV/WHP partition with
**no guest OS and no syscalls**, talks to a tiny typed host ABI, and gets
sub-millisecond warm cold-starts via **per-tenant snapshot-and-restore**.

This PoC turns the design document into something you can **run and test today**.
It has two parts:

| Part | Path | What it is | Runs here? |
|------|------|------------|------------|
| 1. Lifecycle simulation | `arena/ abi/ guest/ vm/ examples/ cmd/golite/` | Pure-Go, executable model of the GoLite host-side lifecycle and its safety properties | ✅ yes — `go run ./cmd/golite`, `go test ./...` |
| 2. Real Hyperlight host | `hyperlight/` | Rust host harness written against the **real** `hyperlight-host` 0.16 API, mapping the same lifecycle onto actual VMM primitives | ⚙️ compiles; runs only on a KVM/MSHV/WHP host |

> **Why two parts?** A real Hyperlight micro-VM needs a hardware hypervisor, and
> this environment has no `/dev/kvm` (and no VMX/SVM). So the *substrate* can't
> execute here — but the **novel** part of GoLite isn't the hypervisor (that
> exists); it's the **protocol and its safety properties**: a no-syscall guest, a
> narrow typed host ABI, ready-barrier snapshot, restore-per-call, entropy/
> generation reseed for uniqueness, and one-tenant-per-VM isolation. Those are
> hardware-independent, so Part 1 implements and **asserts them with tests**,
> while Part 2 proves the same shape compiles against the real Hyperlight crate.

## Quick start

```bash
cd golite-poc
go test ./...        # asserts uniqueness, isolation, ABI discipline, latency
go run ./cmd/golite  # prints the four demonstrations with real numbers
./scripts/run.sh     # both parts; Part 2 self-gates on /dev/kvm
```

## What Part 1 demonstrates (with assertions)

Running `go run ./cmd/golite` produces, e.g.:

```
── 1. snapshot uniqueness (reseed-on-restore) ──
  reseed ON : 1000/1000 UUIDs distinct
  reseed OFF: 1/1000 UUIDs distinct  <- the bug the reseed step prevents
── 2. cross-invocation isolation (restore-per-call) ──
  restore-per-call : next call sees heap offset -> "clean"
  reused arena     : next call sees heap offset -> "TENANT-A-API-KEY"  <- leak
── 3. typed-ABI discipline (deny by default + schema validation) ──
  net without CapNet -> abi: capability denied: net
  200KB output (limit 64KB) -> host rejected output: ... 65539>65536
── 4. latency & density (snapshot pays the init cost once) ──
  one-time boot-to-ready (init + snapshot): ~460µs
  per-instance linear memory footprint    : 384 KiB
  warm restore (n=2000): mean ~100µs  p50 ~60µs  p99 ~500µs
```

(Exact numbers vary by machine; the *shape* — warm restore ≪ boot — is the point.)

| Design claim | Where it's proven |
|--------------|-------------------|
| Restore-per-call gives unique RNG/UUID state per invocation | `TestUniqueness_ReseedOn`, demo §1 |
| Without reseed, clones share secrets (the bug) | `TestUniqueness_ReseedOffCollides`, demo §1 |
| Generation counter (VmGenId analogue) increases per restore | `TestGenerationMonotonic` |
| No tenant state leaks across invocations | `TestIsolation_NoLeakAcrossCalls`, demo §2 |
| Networking is host-mediated & off by default | `TestABI_NetDeniedByDefault`, demo §3 |
| Host rejects malformed/oversized output (schema validation) | `TestABI_OversizedOutputRejected`, demo §3 |
| Errors surface as typed envelopes | `TestErrorEnvelope` |
| Warm restore ≪ one-time boot | `TestRestoreCheaperThanBoot`, demo §4 |

## How the code maps to the design

```
            DESIGN TERM                         POC ARTIFACT
  ─────────────────────────────────   ───────────────────────────────────
  guest linear memory (no kernel)      arena.Arena         (a []byte + regions)
  typed host ABI / FlatBuffer calls    abi.Host            (gated, size-bounded, tagged)
  GOOS=hyperlight runtime overlay      guest.Ctx + seeded RNG (host-mediated I/O only)
  //go:golite_handler export           guest.Handler signature []byte→([]byte,error)
  Rust VMM lifecycle manager           vm.Function         (Boot→Snapshot→Restore→Invoke)
  ready-barrier snapshot               vm.Boot + arena.Snapshot
  restore-per-call + reseed            vm.Function.Invoke  (crypto/rand seed + gen bump)
  one tenant per VM (discard dirtied)  fresh arena per Invoke, never reused
  sample serverless functions          examples/ (uuidgen, jsontransform, leaky)
```

And onto the **real** substrate (`hyperlight/host/src/main.rs`, against
`hyperlight-host` 0.16):

```
  vm.Boot(image)            ->  UninitializedSandbox::new(GuestBinary, ..).register(..).evolve()
  arena.Snapshot()          ->  MultiUseSandbox::snapshot()        -> Arc<Snapshot>
  Invoke: Restore           ->  MultiUseSandbox::restore(snapshot)
  Invoke: call handler      ->  MultiUseSandbox::call("golite_handle", input)
  abi.Host registered fns   ->  sandbox.register("golite_write_output" | "golite_log", ..)
```

## Honest limitations

- **The unified GoLite system does not exist yet** — this is a PoC of the
  *design*. Part 1 is a faithful, tested model of the host-side protocol, not the
  bare-metal Go runtime port itself.
- **Part 1 is a simulation, not a hypervisor.** "Snapshot/restore" here is a real
  memcpy of the guest's linear-memory slice — which is exactly what it is on
  hardware (copy-on-write of guest pages) — but there is no vCPU and no hardware
  isolation boundary in this process. The hardware boundary is Part 2's job.
- **Part 1 cannot *force* a Go handler to avoid `os`/`syscall`** the way a real
  `GOOS=hyperlight` toolchain would; the example handlers honour the contract by
  construction (ABI-only I/O), and the README documents the real enforcement.
- **The `GOOS=hyperlight` guest port is not built here** — it is the larger
  effort described in `hyperlight/guest/README.md` (start from TamaGo's working
  `GOOS=tamago`, retarget the overlay). No `/dev/kvm` in this container means a
  real guest couldn't run even if built.
- **Latency numbers are this machine's, for a simulated restore.** They show the
  right *regime* and the boot-vs-restore gap, not a hardware SLA.

## Layout

```
golite-poc/
├── arena/        guest linear memory + snapshot/restore (memcpy semantics)
├── abi/          typed, capability-gated, size-bounded host ABI
├── guest/        in-guest runtime overlay: handler ctx + seeded RNG (ABI-only I/O)
├── vm/           host-side lifecycle: Boot→Snapshot→Restore-per-call→Invoke (+ tests)
├── examples/     sample tenant functions (uuidgen, jsontransform, leaky)
├── cmd/golite/   runnable demonstration driver
├── hyperlight/   real-substrate harness
│   ├── host/     Rust host vs hyperlight-host 0.16 (compiles; runs on KVM)
│   └── guest/    GOOS=hyperlight guest design + build recipe
└── scripts/run.sh
```
