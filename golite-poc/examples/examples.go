// Package examples holds sample tenant functions — the kind of short-lived Go
// work GoLite targets: transform / validate / route / call-an-API. Each is an
// ordinary Go handler that touches the outside world ONLY through ctx (the typed
// host ABI). None of them import os, net, or syscall.
package examples

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ergjanjaha/gha-exercies/golite-poc/guest"
	"github.com/ergjanjaha/gha-exercies/golite-poc/vm"
)

// UUIDGen returns a fresh UUIDv4 on every invocation. Because the handler's RNG
// is seeded from per-restore entropy, the uniqueness demo can assert that 1000
// invocations yield 1000 distinct UUIDs — and that the broken (no-reseed) path
// collides.
func UUIDGen() vm.Image {
	return vm.Image{
		Name: "uuidgen",
		Init: func() error { return nil },
		Handler: func(ctx *guest.Ctx, input []byte) ([]byte, error) {
			_ = ctx.Log("uuidgen invoked") // gated; no-op unless CapLog granted
			return []byte(ctx.UUIDv4()), nil
		},
	}
}

// JSONTransform validates an incoming JSON object and returns a canonicalised,
// key-sorted form with a request id stamped in. Models a realistic
// validate+transform function and exercises a non-trivial ready-barrier Init.
func JSONTransform() vm.Image {
	// A pretend expensive init: build a lookup table the handler consults. This
	// cost is paid once at Boot and frozen into the snapshot.
	var allow map[string]bool
	return vm.Image{
		Name: "jsontransform",
		Init: func() error {
			allow = make(map[string]bool, 4096)
			for i := 0; i < 4096; i++ {
				allow[fmt.Sprintf("field_%d", i)] = true
			}
			// canonical fields the API accepts
			for _, k := range []string{"name", "amount", "currency", "policyId"} {
				allow[k] = true
			}
			return nil
		},
		Handler: func(ctx *guest.Ctx, input []byte) ([]byte, error) {
			var obj map[string]any
			if err := json.Unmarshal(input, &obj); err != nil {
				return nil, fmt.Errorf("invalid json: %w", err)
			}
			for k := range obj {
				if !allow[k] {
					return nil, fmt.Errorf("rejected unknown field %q", k)
				}
			}
			obj["_reqId"] = ctx.UUIDv4()
			obj["_gen"] = ctx.Generation

			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var b strings.Builder
			b.WriteByte('{')
			for i, k := range keys {
				if i > 0 {
					b.WriteByte(',')
				}
				kv, _ := json.Marshal(obj[k])
				fmt.Fprintf(&b, "%q:%s", k, kv)
			}
			b.WriteByte('}')
			return []byte(b.String()), nil
		},
	}
}

// Leaky is a deliberately adversarial tenant used by the isolation demo. On its
// first call it stashes a "secret" into guest heap; on later calls it reports
// whatever is at that heap offset. With restore-per-call the secret can never be
// observed by a subsequent invocation, because every call starts from the clean
// snapshot. (The heap is reached through ctx-exposed helpers so the demo can
// drive it; a real handler cannot escape its arena either way.)
func Leaky() vm.Image {
	const secretOff = 1024
	return vm.Image{
		Name: "leaky",
		Init: func() error { return nil },
		Handler: func(ctx *guest.Ctx, input []byte) ([]byte, error) {
			switch string(input) {
			case "stash":
				if err := ctx.HeapWrite(secretOff, []byte("TENANT-A-API-KEY")); err != nil {
					return nil, err
				}
				return []byte("stashed"), nil
			case "peek":
				got, err := ctx.HeapRead(secretOff, 16)
				if err != nil {
					return nil, err
				}
				if isZero(got) {
					return []byte("clean"), nil
				}
				return append([]byte("LEAKED:"), got...), nil
			default:
				return nil, errors.New("leaky: want 'stash' or 'peek'")
			}
		},
	}
}

func isZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
