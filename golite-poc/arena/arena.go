// Package arena models the *guest linear memory* of a GoLite micro-VM.
//
// In a real Hyperlight-style partition the guest is nothing more than a flat
// slice of physical memory plus a vCPU: there is no guest kernel and no virtual
// devices. Hyperlight organises that slice into code / stack / heap / comms
// regions with guard pages, and the host validates the layout before running
// the vCPU.
//
// We model exactly that flat slice here as a []byte with a fixed region layout.
// Doing so makes the two operations the whole design hinges on — *snapshot* and
// *restore* — literally a memcpy of this slice, which is precisely what they are
// on real hardware (copy-on-write of the guest's pages). Nothing in this package
// pretends to be a hypervisor; it is the honest data structure that a hypervisor
// snapshots.
package arena

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Region offsets within the linear memory slice. A real port would derive these
// from the linker layout (TamaGo uses -ldflags "-T 0x10010000 -R 0x1000"); here
// we fix them so snapshot/restore and the host ABI agree on where things live.
const (
	HeaderSize = 128 // [0, 128)            runtime state header
	InputCap   = 64 << 10
	OutputCap  = 64 << 10
	HeapCap    = 256 << 10

	inputOff  = HeaderSize
	outputOff = inputOff + InputCap
	heapOff   = outputOff + OutputCap
	totalSize = heapOff + HeapCap
)

// Header field offsets inside the [0, HeaderSize) region.
const (
	offMagic      = 0  // 8 bytes: identifies a GoLite arena
	offGeneration = 8  // 8 bytes: VM generation counter (VmGenId/SysGenId analogue)
	offSeed       = 16 // 32 bytes: per-instance RNG seed (reseeded on every restore)
	offInputLen   = 48 // 4 bytes
	offOutputLen  = 52 // 4 bytes
	offReady      = 56 // 1 byte: ready-barrier reached
	seedLen       = 32
)

const magic = 0x474F4C49544556_31 // "GOLITEV1"

var (
	ErrInputTooLarge  = errors.New("arena: input exceeds comms-IN capacity")
	ErrOutputTooLarge = errors.New("arena: output exceeds comms-OUT capacity")
	ErrHeapRange      = errors.New("arena: heap access out of range")
	ErrBadMagic       = errors.New("arena: bad magic (not a GoLite arena)")
)

// Arena is one guest's linear memory.
type Arena struct {
	mem []byte
}

// New allocates a fresh, zeroed arena and stamps its header.
func New() *Arena {
	a := &Arena{mem: make([]byte, totalSize)}
	binary.LittleEndian.PutUint64(a.mem[offMagic:], magic)
	return a
}

// Size reports the total linear-memory footprint of one instance. This is the
// number that drives density: it is the Go binary's data + heap, with no OS
// image underneath it.
func (a *Arena) Size() int { return len(a.mem) }

// Snapshot returns an independent copy of the entire linear memory. On real
// hardware this is the host capturing the guest's pages once the ready barrier
// is reached; here it is the same memcpy.
func (a *Arena) Snapshot() []byte {
	return append([]byte(nil), a.mem...)
}

// Restore overwrites this arena's memory with a previously taken snapshot.
// Cheap and constant-work: it is the reason warm starts are sub-millisecond.
func (a *Arena) Restore(snap []byte) error {
	if len(snap) != totalSize {
		return fmt.Errorf("arena: snapshot size %d != %d", len(snap), totalSize)
	}
	if binary.LittleEndian.Uint64(snap[offMagic:]) != magic {
		return ErrBadMagic
	}
	copy(a.mem, snap)
	return nil
}

// --- Header accessors -------------------------------------------------------

func (a *Arena) Generation() uint64 {
	return binary.LittleEndian.Uint64(a.mem[offGeneration:])
}

func (a *Arena) SetGeneration(g uint64) {
	binary.LittleEndian.PutUint64(a.mem[offGeneration:], g)
}

// Seed returns a copy of the per-instance RNG seed. The guest runtime derives
// all randomness (UUIDs, map seeds, crypto nonces) from this, so reseeding it on
// every restore is what guarantees two restored clones never share RNG state.
func (a *Arena) Seed() []byte {
	return append([]byte(nil), a.mem[offSeed:offSeed+seedLen]...)
}

func (a *Arena) SetSeed(seed []byte) {
	if len(seed) != seedLen {
		panic("arena: seed must be 32 bytes")
	}
	copy(a.mem[offSeed:offSeed+seedLen], seed)
}

func (a *Arena) Ready() bool { return a.mem[offReady] == 1 }

func (a *Arena) SetReady() { a.mem[offReady] = 1 }

// --- Comms regions ----------------------------------------------------------

// WriteInput places the request blob into the comms-IN region. The host does
// this just before running the vCPU.
func (a *Arena) WriteInput(b []byte) error {
	if len(b) > InputCap {
		return ErrInputTooLarge
	}
	binary.LittleEndian.PutUint32(a.mem[offInputLen:], uint32(len(b)))
	copy(a.mem[inputOff:inputOff+InputCap], make([]byte, InputCap)) // clear stale
	copy(a.mem[inputOff:], b)
	return nil
}

// ReadInput is called from inside the guest to fetch its request blob.
func (a *Arena) ReadInput() []byte {
	n := binary.LittleEndian.Uint32(a.mem[offInputLen:])
	return append([]byte(nil), a.mem[inputOff:inputOff+int(n)]...)
}

// WriteOutput is called from inside the guest to publish its result blob.
func (a *Arena) WriteOutput(b []byte) error {
	if len(b) > OutputCap {
		return ErrOutputTooLarge
	}
	binary.LittleEndian.PutUint32(a.mem[offOutputLen:], uint32(len(b)))
	copy(a.mem[outputOff:], b)
	return nil
}

// ReadOutput is called by the host after the vCPU halts to collect the result.
func (a *Arena) ReadOutput() []byte {
	n := binary.LittleEndian.Uint32(a.mem[offOutputLen:])
	return append([]byte(nil), a.mem[outputOff:outputOff+int(n)]...)
}

// --- Heap (per-instance mutable scratch) -----------------------------------

// HeapWrite stores bytes at a guest heap offset. We expose this so the isolation
// demo can show that data written by one invocation does NOT survive into the
// next once the arena is restored from the clean snapshot.
func (a *Arena) HeapWrite(off int, b []byte) error {
	if off < 0 || off+len(b) > HeapCap {
		return ErrHeapRange
	}
	copy(a.mem[heapOff+off:], b)
	return nil
}

func (a *Arena) HeapRead(off, n int) ([]byte, error) {
	if off < 0 || off+n > HeapCap {
		return nil, ErrHeapRange
	}
	return append([]byte(nil), a.mem[heapOff+off:heapOff+off+n]...), nil
}
