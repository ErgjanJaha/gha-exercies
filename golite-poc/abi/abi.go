// Package abi defines the *only* channel between a GoLite guest and the outside
// world. It stands in for Hyperlight's host↔guest interface: typed function
// calls whose arguments cross the boundary as schema-validated blobs in shared
// memory, with the host rejecting anything that does not match.
//
// The design point being demonstrated: a GoLite guest makes NO syscalls. Where
// ordinary Go would call mmap/epoll/socket/clock_gettime/getrandom, a GoLite
// guest calls one of the small, explicitly-registered host functions below.
// That tiny registered set — not a kernel — is the host's attack surface, so we
// keep it minimal, typed, size-bounded, and capability-gated.
package abi

import (
	"errors"
	"fmt"
)

// Capability is an allow-listed permission granted to an instance at restore
// time. Everything is denied by default; a function only gets what its tenant
// policy explicitly grants.
type Capability string

const (
	CapLog Capability = "log" // write to host stderr/log
	CapNet Capability = "net" // host-mediated socket send/recv
	CapClk Capability = "clock"
)

// Limits bound a single boundary crossing. Oversized or malformed messages are
// refused before the host ever looks at the payload — a DoS and confused-deputy
// guard, mirroring Hyperlight's FlatBuffer type validation + execution timeouts.
type Limits struct {
	MaxOutput int
	MaxLogMsg int
	MaxNetMsg int
}

func DefaultLimits() Limits {
	return Limits{MaxOutput: 64 << 10, MaxLogMsg: 4 << 10, MaxNetMsg: 16 << 10}
}

var (
	ErrDenied      = errors.New("abi: capability denied")
	ErrTooLarge    = errors.New("abi: payload exceeds registered limit")
	ErrBadEnvelope = errors.New("abi: malformed message envelope")
	ErrUnknownCall = errors.New("abi: unknown host function")
)

// Host is the set of registered host functions. In Hyperlight terms this is the
// host's function table; a guest can call nothing that is not in it. The host
// owns all real I/O — the guest only ever hands it validated blobs.
type Host struct {
	limits Limits
	caps   map[Capability]bool

	// now and entropy are injected by the host. Crucially, the guest does NOT
	// read the clock or the RNG itself (no clock_gettime, no getrandom syscall);
	// the host mediates both, which is also how snapshot uniqueness is enforced.
	logSink func(string)
	netSink func([]byte) ([]byte, error)

	// Counters let the demos and tests assert how the boundary was exercised.
	Calls   int
	Refused int
}

// NewHost builds a host with the given granted capabilities. Anything not in
// caps is denied.
func NewHost(limits Limits, granted ...Capability) *Host {
	h := &Host{limits: limits, caps: map[Capability]bool{}}
	for _, c := range granted {
		h.caps[c] = true
	}
	return h
}

func (h *Host) SetLogSink(fn func(string))                 { h.logSink = fn }
func (h *Host) SetNetSink(fn func([]byte) ([]byte, error)) { h.netSink = fn }

func (h *Host) has(c Capability) bool { return h.caps[c] }

// --- Registered host functions ---------------------------------------------
//
// Each function is the equivalent of one entry in Hyperlight's host function
// table. They all validate first, act second.

// Log is the guest's only path to stderr. Gated by CapLog and size-bounded.
func (h *Host) Log(msg string) error {
	h.Calls++
	if !h.has(CapLog) {
		h.Refused++
		return fmt.Errorf("%w: log", ErrDenied)
	}
	if len(msg) > h.limits.MaxLogMsg {
		h.Refused++
		return fmt.Errorf("%w: log %d>%d", ErrTooLarge, len(msg), h.limits.MaxLogMsg)
	}
	if h.logSink != nil {
		h.logSink(msg)
	}
	return nil
}

// Net is the guest's only path to the network: a host-mediated, allow-listed
// socket. Off by default (no CapNet) — exactly the "networking is host-mediated
// and allow-listed, off by default" sacrifice in the design.
func (h *Host) Net(req []byte) ([]byte, error) {
	h.Calls++
	if !h.has(CapNet) {
		h.Refused++
		return nil, fmt.Errorf("%w: net", ErrDenied)
	}
	if len(req) > h.limits.MaxNetMsg {
		h.Refused++
		return nil, fmt.Errorf("%w: net %d>%d", ErrTooLarge, len(req), h.limits.MaxNetMsg)
	}
	if h.netSink == nil {
		return nil, errors.New("abi: net granted but no host socket wired")
	}
	return h.netSink(req)
}

// ValidateOutput is what the host runs on the guest's result before accepting
// it: a stand-in for FlatBuffer schema validation. Here the "schema" is: a
// non-empty, length-bounded blob carrying a recognised envelope tag.
func (h *Host) ValidateOutput(out []byte) error {
	if len(out) > h.limits.MaxOutput {
		h.Refused++
		return fmt.Errorf("%w: output %d>%d", ErrTooLarge, len(out), h.limits.MaxOutput)
	}
	if !ValidEnvelope(out) {
		h.Refused++
		return ErrBadEnvelope
	}
	return nil
}

// --- Message envelope -------------------------------------------------------
//
// A deliberately tiny typed framing so the host can reject "anything not
// matching the strongly typed schema" without trusting the guest. Real GoLite
// would use FlatBuffers; the shape of the check is identical.

const (
	tagMagic byte = 0xA7
	// Envelope: [magic][tag][len:2][payload...]
	TagResult byte = 0x01
	TagError  byte = 0x02
)

// Encode frames a payload with a type tag. Used by the guest runtime when it
// publishes a result.
func Encode(tag byte, payload []byte) []byte {
	if len(payload) > 0xFFFF {
		payload = payload[:0xFFFF]
	}
	out := make([]byte, 4+len(payload))
	out[0] = tagMagic
	out[1] = tag
	out[2] = byte(len(payload) >> 8)
	out[3] = byte(len(payload))
	copy(out[4:], payload)
	return out
}

// Decode validates and unwraps an envelope.
func Decode(b []byte) (tag byte, payload []byte, ok bool) {
	if len(b) < 4 || b[0] != tagMagic {
		return 0, nil, false
	}
	n := int(b[2])<<8 | int(b[3])
	if 4+n != len(b) {
		return 0, nil, false
	}
	if b[1] != TagResult && b[1] != TagError {
		return 0, nil, false
	}
	return b[1], b[4:], true
}

// ValidEnvelope reports whether b is a well-formed, recognised envelope.
func ValidEnvelope(b []byte) bool {
	_, _, ok := Decode(b)
	return ok
}
