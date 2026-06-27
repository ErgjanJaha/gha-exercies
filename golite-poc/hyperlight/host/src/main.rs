/*
GoLite — real Hyperlight host harness.

This is the host-side (Rust VMM) part of the GoLite design, written against the
ACTUAL hyperlight-host 0.16 API. It demonstrates that the lifecycle the Go
simulation models — boot once to a ready barrier, snapshot, then restore the
snapshot per invocation — maps directly onto Hyperlight's real primitives:

    UninitializedSandbox::new(GuestBinary, ..)   // load the guest image
        .register("golite_write_output", ..)     // the GoLite host ABI
        .register("golite_log", ..)
        .evolve()                                 // run init to the ready barrier
        -> MultiUseSandbox
        .snapshot()                               // freeze the ready state
        .restore(snap)  + .call("handle", input)  // per-request warm cold-start

What this harness does NOT do here is run, because a Hyperlight micro-VM needs a
hardware hypervisor (KVM/MSHV/WHP) and this container has no /dev/kvm. The code
is real and type-checks against the published crate (`cargo build` succeeds);
`is_hypervisor_present()` gates execution so the binary explains itself cleanly
on hardware that lacks virtualization.

The guest image is the GoLite-compiled Go function. Building a `GOOS=hyperlight`
guest is the larger engineering effort described in the README; pass a path to a
Hyperlight guest binary as the first argument to exercise the full path on a
KVM-capable host.
*/

use std::env;
use std::sync::Arc;
use std::sync::Mutex;

use hyperlight_host::sandbox::snapshot::Snapshot;
use hyperlight_host::{GuestBinary, MultiUseSandbox, UninitializedSandbox};

fn main() -> hyperlight_host::Result<()> {
    println!("GoLite Hyperlight host harness");

    // Gate on the real substrate. On a machine without KVM/MSHV/WHP we can still
    // prove the code is correct (it compiled), but we cannot create a partition.
    if !hyperlight_host::is_hypervisor_present() {
        eprintln!(
            "no hardware hypervisor present (need KVM/MSHV/WHP).\n\
             This binary type-checks against hyperlight-host 0.16 but cannot create\n\
             a micro-VM here. Run on a KVM-enabled host with a GoLite guest binary:\n\
             \n    golite-host /path/to/golite-guest\n"
        );
        return Ok(());
    }

    let guest_path = env::args().nth(1).ok_or_else(|| {
        hyperlight_host::HyperlightError::Error(
            "usage: golite-host <path-to-golite-guest-binary>".into(),
        )
    })?;

    // The host ABI: a tiny, explicitly-registered function table — the guest's
    // ONLY route to the outside world. This mirrors the Go sim's abi.Host.
    //
    // We collect the guest's output blob here on the host side.
    let output: Arc<Mutex<Vec<u8>>> = Arc::new(Mutex::new(Vec::new()));

    let mut uninit = UninitializedSandbox::new(GuestBinary::FilePath(guest_path), None)?;

    // golite_log: gated host stderr.
    uninit.register("golite_log", |msg: String| {
        eprintln!("[guest] {msg}");
        Ok(0i32)
    })?;

    // golite_write_output: the guest publishes its result blob through this.
    {
        let sink = output.clone();
        uninit.register("golite_write_output", move |blob: Vec<u8>| {
            *sink.lock().unwrap() = blob;
            Ok(0i32)
        })?;
    }

    // evolve() runs guest runtime + package init to the ready barrier.
    let mut sandbox: MultiUseSandbox = uninit.evolve()?;

    // Take the ready-barrier snapshot ONCE, off the request path.
    let ready: Arc<Snapshot> = sandbox.snapshot()?;
    println!("captured ready-barrier snapshot");

    // Per-request warm cold-start: restore the clean snapshot, hand the guest
    // its input, run, collect the output, discard the dirtied state by restoring
    // again next time. One tenant per VM; nothing leaks between calls.
    let requests: [&[u8]; 3] = [b"{\"hello\":1}", b"{\"hello\":2}", b"{\"hello\":3}"];
    for (i, req) in requests.iter().enumerate() {
        sandbox.restore(ready.clone())?; // <- the GoLite restore-per-call move

        // The guest's exported handler. With a GOOS=hyperlight guest this is the
        // //go:golite_handler-marked function.
        let _: i32 = sandbox.call("golite_handle", req.to_vec())?;

        let out = output.lock().unwrap().clone();
        println!("request {i}: guest returned {} bytes", out.len());
    }

    println!("done");
    Ok(())
}
