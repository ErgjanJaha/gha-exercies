# Solution — 02 optimize image

## Before vs. after

| Aspect            | Single-stage `golang:1.22` | Multi-stage + distroless |
|-------------------|----------------------------|--------------------------|
| Image size        | ~800 MB                    | ~10 MB                   |
| Runtime contents  | Full Go toolchain, shell, package manager, source code | Just the binary + CA certs |
| Attack surface    | Huge — every CVE in the Go base, glibc, shell, etc. | Minimal — no shell, no package manager |
| Build cache reuse | Poor (`COPY . .` busts everything) | Good (`go.mod` copied first → `go mod download` cached) |
| Runs as           | root                       | `nonroot` (uid 65532)    |

## What the solution does

1. **Stage 1 (`build`)** uses `golang:1.22-alpine` purely as a compiler. We
   `COPY go.mod` first and run `go mod download` so the dependency layer caches
   independently of source changes (this project has no external deps, but the
   pattern is what interviewers want to see). Then `CGO_ENABLED=0` produces a
   fully static binary so the runtime stage doesn't need libc. `-ldflags="-s -w"`
   strips the symbol table and DWARF debug info — typically 25–30% smaller.

2. **Stage 2** uses `gcr.io/distroless/static-debian12:nonroot`. "Distroless"
   means *no shell, no package manager, no busybox* — just the binary, CA
   certificates, and tzdata. The `:nonroot` tag pre-creates an unprivileged
   user (`nonroot`, uid 65532). We `COPY --from=build` only the artifact,
   `USER nonroot:nonroot`, and use `ENTRYPOINT` so the binary is PID 1.

## One-paragraph reasoning

The single-stage image ships everything needed to *build* Go programs to every
production node, which is both wasteful (slower pulls, more disk) and unsafe
(every CVE in the Go toolchain is now a CVE in your fleet). Multi-stage builds
let us throw away the build environment entirely — the runtime image only
contains what's required to *execute*. Distroless takes that one step further
than Alpine: no shell means no `docker exec /bin/sh` for an attacker who pops
the process, and no package manager means no opportunistic supply-chain abuse.
For a static Go binary this trade-off is nearly free: you give up `kubectl
exec`-style debugging convenience and gain a ~80x smaller, much safer image.
