// Package guest is the in-guest side of GoLite: the thin runtime overlay and
// shim that a user's Go function is linked against.
//
// It plays the role of TamaGo's runtime/goos overlay plus the guest-ABI shim.
// A real port would supply the bare-metal hooks (RamStart/RamSize, Nanotime,
// GetRandomData/InitRNG, Printk, …) and a //go:golite_handler export directive.
// Here the same contract is expressed in ordinary Go: the user registers a
// handler with the signature []byte -> ([]byte, error), and inside the handler
// the ONLY way to reach the outside world is through Ctx, which routes every
// call through the typed host ABI. There is deliberately no os, no net, no
// syscall reachable from a handler.
package guest

import (
	"encoding/binary"

	"github.com/ergjanjaha/gha-exercies/golite-poc/abi"
	"github.com/ergjanjaha/gha-exercies/golite-poc/arena"
)

// Handler is the user's function. Analogous to a func marked //go:golite_handler.
type Handler func(ctx *Ctx, input []byte) ([]byte, error)

// Ctx is what a handler is handed. It is the guest's whole world: a request
// blob, a host it may call (only through registered functions), and a private
// RNG seeded from the arena's per-instance seed.
type Ctx struct {
	host *abi.Host
	rng  *splitmix64
	mem  *arena.Arena // this instance's private linear memory

	// generation is the VM generation counter at the time of this invocation.
	// A correctly written handler that needs a unique identity derives it from
	// (seed, generation), never from a baked-in constant — the snapshot
	// uniqueness guarantee depends on this.
	Generation uint64
}

// NewCtx wires a handler context from the host plus the entropy the host seeded
// into the arena for this restore.
func NewCtx(host *abi.Host, mem *arena.Arena, seed []byte, generation uint64) *Ctx {
	return &Ctx{
		host:       host,
		rng:        newSplitmix64(seed),
		mem:        mem,
		Generation: generation,
	}
}

// HeapWrite / HeapRead give a handler access to its own per-instance scratch
// memory. The point of the isolation demo: anything written here is gone on the
// next invocation because that invocation restores a clean snapshot.
func (c *Ctx) HeapWrite(off int, b []byte) error   { return c.mem.HeapWrite(off, b) }
func (c *Ctx) HeapRead(off, n int) ([]byte, error) { return c.mem.HeapRead(off, n) }

// Log routes to the host's gated log function.
func (c *Ctx) Log(msg string) error { return c.host.Log(msg) }

// Net routes to the host's gated, allow-listed socket. Denied unless the tenant
// policy granted CapNet.
func (c *Ctx) Net(req []byte) ([]byte, error) { return c.host.Net(req) }

// Random returns n bytes from the per-instance RNG. Because the seed is fresh on
// every restore, two invocations never produce the same stream — this is what
// makes UUIDs/nonces unique across cloned VMs.
func (c *Ctx) Random(n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i += 8 {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], c.rng.next())
		copy(out[i:], b[:])
	}
	return out
}

// UUIDv4 is a convenience over Random for the uniqueness demo.
func (c *Ctx) UUIDv4() string {
	b := c.Random(16)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	const hex = "0123456789abcdef"
	var s [36]byte
	j := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			s[j] = '-'
			j++
		}
		s[j] = hex[v>>4]
		s[j+1] = hex[v&0x0f]
		j += 2
	}
	return string(s[:])
}

// --- splitmix64: a tiny deterministic RNG seeded purely from the arena seed ---
//
// Deterministic-given-seed is the whole point: it lets the tests prove that
// "same seed => same stream" and therefore that uniqueness comes ENTIRELY from
// the host reseeding on restore, not from any ambient entropy the guest sneaks
// in. (A guest has no getrandom syscall to sneak in with.)

type splitmix64 struct{ state uint64 }

func newSplitmix64(seed []byte) *splitmix64 {
	// Fold the 32-byte seed into 64 bits deterministically (no ambient entropy:
	// a fixed FNV-style mixing constant, never an OS RNG the guest cannot reach).
	var s uint64 = 0x9e3779b97f4a7c15
	for _, b := range seed {
		s ^= uint64(b)
		s *= 0x100000001b3
	}
	return &splitmix64{state: s}
}

func (r *splitmix64) next() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
