# Exercise 02 — Shrink the bloated Go image

**Service:** `quote-engine` (Go, plain net/http)
**Difficulty:** ★★☆☆☆
**Time target:** 10–15 minutes

## The scenario

The current Dockerfile works — it builds and runs — but the resulting image is
**~800 MB** for a single static binary. Your job is to convert it to a
multi-stage build and shrink it dramatically while keeping it functional.

## The goal

Edit `Dockerfile` in this folder until **all of the following are true**:

1. `docker build -t quote-engine-slim .` succeeds
2. `docker run --rm -p 8080:8080 quote-engine-slim` starts the service
3. `curl http://localhost:8080/health` returns ok
4. `docker images quote-engine-slim` is **< 30 MB** (stretch: < 10 MB)
5. The runtime image contains **no Go toolchain and no shell** (verify with
   `docker run --rm quote-engine-slim ls /` — it should error or list very little)
6. The process runs as a **non-root** user

## Hints (peek only if stuck)

- Multi-stage build: one `FROM golang:...` stage to compile, one minimal stage
  to run.
- Distroless or `scratch` for the runtime stage.
- For static binaries, build with `CGO_ENABLED=0`.
- `-ldflags="-s -w"` strips debug info.

## Done looks like

- Image is < 30 MB.
- Health check passes.
- You can explain what each stage does and why distroless/scratch is safer than
  a full base image.

When you're done, compare with `SOLUTIONS/02-optimize-image/Dockerfile`.
