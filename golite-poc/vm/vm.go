// Package vm is the host-side lifecycle manager for GoLite micro-VMs: the part
// the Rust VMM would own in a real deployment. It implements the three moves the
// whole design is built on:
//
//  1. Boot once, off the request path, running runtime + package init all the
//     way to a "ready barrier", then take a memory Snapshot.
//  2. Per request, Restore that snapshot into a clean arena — and on every
//     restore reseed entropy and bump a VM generation counter so no two clones
//     share RNG/UUID/secret state (the VmGenId / MADV_WIPEONSUSPEND fix).
//  3. Invoke the handler against the restored arena, collect the result through
//     the typed ABI, then discard the dirtied arena — never reuse it across
//     tenants (one tenant per VM).
//
// There is no hypervisor here (this container has no /dev/kvm); see the README
// for the KVM-gated real-Hyperlight smoke test. What this package proves is the
// *protocol and its safety properties*, which are hardware-independent.
package vm

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/ergjanjaha/gha-exercies/golite-poc/abi"
	"github.com/ergjanjaha/gha-exercies/golite-poc/arena"
	"github.com/ergjanjaha/gha-exercies/golite-poc/guest"
)

// Image bundles everything needed to boot one function: the user's handler plus
// the expensive, idempotent initialisation that runs to the ready barrier. The
// init cost is paid once at Boot and then baked into the snapshot, so warm
// restores never pay it again.
type Image struct {
	Name    string
	Handler guest.Handler
	// Init simulates package-level init: building tables, parsing config,
	// constructing a netstack, warming caches. It runs once, before the
	// snapshot is taken. Make it do real work so the latency benchmark is honest.
	Init func() error
}

// Function is a booted image with its ready-barrier snapshot captured.
type Function struct {
	img      Image
	snapshot []byte // immutable golden image taken at the ready barrier
	gen      uint64 // monotonically increasing across restores

	// BootDuration is how long the one-time boot-to-ready took.
	BootDuration time.Duration
}

// Boot runs the image's init to the ready barrier and snapshots the result.
// This is the expensive, off-the-request-path step.
func Boot(img Image) (*Function, error) {
	start := time.Now()
	a := arena.New()
	if img.Init != nil {
		if err := img.Init(); err != nil {
			return nil, fmt.Errorf("boot %s: init: %w", img.Name, err)
		}
	}
	// Seed the golden image deterministically with zeros; the real per-instance
	// entropy is injected on every restore, NOT baked into the snapshot — that
	// is the entire point of the uniqueness fix.
	a.SetSeed(make([]byte, 32))
	a.SetReady()
	return &Function{
		img:          img,
		snapshot:     a.Snapshot(),
		BootDuration: time.Since(start),
	}, nil
}

// Policy is the per-tenant capability grant applied when an instance is restored.
type Policy struct {
	Caps    []abi.Capability
	Limits  abi.Limits
	LogSink func(string)
	NetSink func([]byte) ([]byte, error)
}

// Result is the outcome of one invocation.
type Result struct {
	Output     []byte
	Generation uint64
	Seed       []byte // the per-instance seed used (for the uniqueness test)
	Restore    time.Duration
	Run        time.Duration
	HostCalls  int
	HostRefuse int
}

// reseed controls whether restore injects fresh entropy. Production is always
// true; the demos flip it to false to show what the mitigation prevents.
type reseedMode bool

const (
	reseedOn  reseedMode = true
	reseedOff reseedMode = false
)

// Invoke performs a full warm cold-start: restore -> reseed -> run -> collect ->
// discard. Each call gets its own fresh arena; nothing leaks between calls.
func (f *Function) Invoke(input []byte, p Policy) (*Result, error) {
	return f.invoke(input, p, reseedOn)
}

// InvokeNoReseed restores WITHOUT injecting fresh entropy or bumping the
// generation counter. It exists only to demonstrate the vulnerability the
// reseed step prevents (cloned VMs sharing RNG/secret state). Never use this in
// production — it is the wrong way, shown on purpose.
func (f *Function) InvokeNoReseed(input []byte, p Policy) (*Result, error) {
	return f.invoke(input, p, reseedOff)
}

func (f *Function) invoke(input []byte, p Policy, mode reseedMode) (*Result, error) {
	// --- restore: clean arena from the golden snapshot ---
	tRestore := time.Now()
	a := arena.New()
	if err := a.Restore(f.snapshot); err != nil {
		return nil, err
	}

	// --- uniqueness: fresh entropy + bumped generation on every restore ---
	var seed []byte
	if mode == reseedOn {
		seed = make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return nil, fmt.Errorf("invoke %s: entropy: %w", f.img.Name, err)
		}
		f.gen++
		a.SetSeed(seed)
		a.SetGeneration(f.gen)
	} else {
		// Broken mode: reuse the snapshot's baked-in (zero) seed and a fixed
		// generation. This is the bug the design exists to avoid.
		seed = a.Seed()
		a.SetGeneration(0)
	}
	restoreDur := time.Since(tRestore)

	if err := a.WriteInput(input); err != nil {
		return nil, err
	}

	// --- run the guest handler against its private arena ---
	host := abi.NewHost(orDefault(p.Limits), p.Caps...)
	host.SetLogSink(p.LogSink)
	host.SetNetSink(p.NetSink)
	ctx := guest.NewCtx(host, a, a.Seed(), a.Generation())

	tRun := time.Now()
	out, herr := f.img.Handler(ctx, a.ReadInput())
	runDur := time.Since(tRun)

	// --- collect result through the typed ABI (host validates before accepting) ---
	var envelope []byte
	if herr != nil {
		envelope = abi.Encode(abi.TagError, []byte(herr.Error()))
	} else {
		envelope = abi.Encode(abi.TagResult, out)
	}
	if err := host.ValidateOutput(envelope); err != nil {
		return nil, fmt.Errorf("invoke %s: host rejected output: %w", f.img.Name, err)
	}
	if err := a.WriteOutput(envelope); err != nil {
		return nil, err
	}

	res := &Result{
		Output:     a.ReadOutput(),
		Generation: a.Generation(),
		Seed:       seed,
		Restore:    restoreDur,
		Run:        runDur,
		HostCalls:  host.Calls,
		HostRefuse: host.Refused,
	}
	// --- discard: the dirtied arena `a` simply goes out of scope and is never
	// reused. The next Invoke restores a fresh copy of the golden snapshot. ---
	return res, herr
}

// InstanceSize reports the linear-memory footprint of a single restored
// instance — the density figure.
func (f *Function) InstanceSize() int { return len(f.snapshot) }

func orDefault(l abi.Limits) abi.Limits {
	if l.MaxOutput == 0 {
		return abi.DefaultLimits()
	}
	return l
}
