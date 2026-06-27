package vm_test

import (
	"bytes"
	"testing"

	"github.com/ergjanjaha/gha-exercies/golite-poc/abi"
	"github.com/ergjanjaha/gha-exercies/golite-poc/examples"
	"github.com/ergjanjaha/gha-exercies/golite-poc/guest"
	"github.com/ergjanjaha/gha-exercies/golite-poc/vm"
)

func decode(t *testing.T, out []byte) (byte, []byte) {
	t.Helper()
	tag, payload, ok := abi.Decode(out)
	if !ok {
		t.Fatalf("output is not a valid envelope: %x", out)
	}
	return tag, payload
}

// Reseeding on every restore must make N invocations yield N distinct UUIDs.
func TestUniqueness_ReseedOn(t *testing.T) {
	fn, err := vm.Boot(examples.UUIDGen())
	if err != nil {
		t.Fatal(err)
	}
	const N = 5000
	seen := make(map[string]bool, N)
	for i := 0; i < N; i++ {
		res, err := fn.Invoke(nil, vm.Policy{})
		if err != nil {
			t.Fatal(err)
		}
		_, p := decode(t, res.Output)
		if seen[string(p)] {
			t.Fatalf("duplicate UUID at i=%d: %s", i, p)
		}
		seen[string(p)] = true
	}
	if len(seen) != N {
		t.Fatalf("want %d distinct, got %d", N, len(seen))
	}
}

// Without reseeding, every clone shares the snapshot's RNG state and collides —
// the bug the reseed step exists to prevent.
func TestUniqueness_ReseedOffCollides(t *testing.T) {
	fn, err := vm.Boot(examples.UUIDGen())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		res, err := fn.InvokeNoReseed(nil, vm.Policy{})
		if err != nil {
			t.Fatal(err)
		}
		_, p := decode(t, res.Output)
		seen[string(p)] = true
	}
	if len(seen) != 1 {
		t.Fatalf("expected all clones to collide to 1 value, got %d distinct", len(seen))
	}
}

// Generation counter must increase monotonically across restores.
func TestGenerationMonotonic(t *testing.T) {
	fn, err := vm.Boot(examples.UUIDGen())
	if err != nil {
		t.Fatal(err)
	}
	var last uint64
	for i := 0; i < 50; i++ {
		res, err := fn.Invoke(nil, vm.Policy{})
		if err != nil {
			t.Fatal(err)
		}
		if res.Generation <= last {
			t.Fatalf("generation not monotonic: %d after %d", res.Generation, last)
		}
		last = res.Generation
	}
}

// A secret written into guest heap by one invocation must be unreachable by the
// next, because each call restores the clean snapshot.
func TestIsolation_NoLeakAcrossCalls(t *testing.T) {
	fn, err := vm.Boot(examples.Leaky())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fn.Invoke([]byte("stash"), vm.Policy{}); err != nil {
		t.Fatal(err)
	}
	res, err := fn.Invoke([]byte("peek"), vm.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	_, p := decode(t, res.Output)
	if !bytes.Equal(p, []byte("clean")) {
		t.Fatalf("secret leaked across invocations: %q", p)
	}
}

// Networking is denied unless the tenant policy grants CapNet.
func TestABI_NetDeniedByDefault(t *testing.T) {
	img := vm.Image{
		Name: "net",
		Init: func() error { return nil },
		Handler: func(ctx *guest.Ctx, input []byte) ([]byte, error) {
			_, err := ctx.Net([]byte("x"))
			return []byte("ok"), err
		},
	}
	fn, _ := vm.Boot(img)
	if _, err := fn.Invoke(nil, vm.Policy{}); err == nil {
		t.Fatal("expected net denied by default")
	}
	fn2, _ := vm.Boot(img)
	if _, err := fn2.Invoke(nil, vm.Policy{
		Caps:    []abi.Capability{abi.CapNet},
		NetSink: func(b []byte) ([]byte, error) { return []byte("ok"), nil },
	}); err != nil {
		t.Fatalf("granted net should succeed: %v", err)
	}
}

// The host rejects oversized output before accepting it.
func TestABI_OversizedOutputRejected(t *testing.T) {
	img := vm.Image{
		Name: "big",
		Init: func() error { return nil },
		Handler: func(ctx *guest.Ctx, input []byte) ([]byte, error) {
			return bytes.Repeat([]byte("x"), 200<<10), nil
		},
	}
	fn, _ := vm.Boot(img)
	if _, err := fn.Invoke(nil, vm.Policy{}); err == nil {
		t.Fatal("expected oversized output rejected")
	}
}

// A handler error is surfaced as a typed Error envelope, still schema-valid.
func TestErrorEnvelope(t *testing.T) {
	fn, err := vm.Boot(examples.JSONTransform())
	if err != nil {
		t.Fatal(err)
	}
	res, herr := fn.Invoke([]byte("not json"), vm.Policy{})
	if herr == nil {
		t.Fatal("expected handler error on bad json")
	}
	tag, _ := decode(t, res.Output)
	if tag != abi.TagError {
		t.Fatalf("want error tag, got %d", tag)
	}
}

// Warm restore must be cheaper than the one-time boot. (Sanity, not a perf SLA.)
func TestRestoreCheaperThanBoot(t *testing.T) {
	fn, err := vm.Boot(examples.JSONTransform())
	if err != nil {
		t.Fatal(err)
	}
	input := []byte(`{"name":"Acme","amount":1,"currency":"EUR","policyId":"P1"}`)
	res, err := fn.Invoke(input, vm.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Restore >= fn.BootDuration {
		t.Fatalf("warm restore (%s) should be cheaper than boot (%s)", res.Restore, fn.BootDuration)
	}
}
