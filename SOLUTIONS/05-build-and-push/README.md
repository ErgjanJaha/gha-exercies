# Solution — 05 build and push

## Anatomy of the workflow

### Triggers
- `push` to `main` → real publish runs.
- `workflow_dispatch` → manual button in the Actions UI for ad-hoc rebuilds.

### Top-level `permissions:`
- `contents: read` is the minimum the workflow needs to check out code.
- `packages: write` lets `GITHUB_TOKEN` push to GHCR. Without it, the login
  succeeds but the push 403s — a classic gotcha.

### `test` job
- Standalone. Fast feedback. Blocks the publish job.
- Could be parallelized further (lint, unit, integration) — kept minimal here.

### `build-and-push` job
Five steps, each with a specific role:

1. **`actions/checkout@v4`** — the source.
2. **`docker/setup-buildx-action@v3`** — installs BuildKit, which is what
   makes the `cache-from: type=gha` cache backend work. Without buildx you're
   stuck with the legacy `docker build` cache (per-layer, no remote).
3. **`docker/login-action@v3`** — authenticates to GHCR using the
   per-run `GITHUB_TOKEN`. No long-lived PAT, no manually-managed secret.
4. **`docker/metadata-action@v5`** — generates a coherent set of tags and
   OCI labels from the GitHub event:
   - `type=ref,event=branch` → `main`
   - `type=sha,prefix=sha-` → `sha-abc1234` (immutable, traceable)
   - `type=raw,value=latest,enable={{is_default_branch}}` → only put
     `:latest` on main, never on a PR.
   It also emits `org.opencontainers.image.*` labels (source URL, revision,
   created timestamp) for free.
5. **`docker/build-push-action@v6`** — builds and pushes in one go.
   `cache-from`/`cache-to` use the GitHub Actions cache backend (`type=gha`)
   so the next run reuses unchanged layers. `mode=max` caches *all* layers,
   not just the final ones — bigger cache, much faster rebuilds.

## One-paragraph reasoning

This workflow is the textbook answer to "publish a Docker image from CI." The
job split (`test` → `build-and-push`) prevents shipping broken images. The
two-step Docker setup (`setup-buildx` then `login`) reflects how modern
Docker builds work: BuildKit is the engine, the registry login is separate.
The three Docker actions are a deliberate chain — `metadata-action` is the
single source of truth for tags and labels, `build-push-action` consumes its
output via `${{ steps.meta.outputs.tags }}`. Caching with `type=gha,mode=max`
typically cuts subsequent builds from minutes to seconds. The `permissions:`
block is the security story: instead of a long-lived PAT in a repo secret,
each run gets a scoped, ephemeral token that GitHub revokes when the run
ends.

## Fanning out to all 4 services

For an interview "next step", add a matrix to the build job:

```yaml
build-and-push:
  needs: test
  strategy:
    matrix:
      service: [policy-api, claims-service, quote-engine, portal]
  steps:
    ...
    - uses: docker/metadata-action@v5
      with:
        images: ghcr.io/${{ github.repository }}/${{ matrix.service }}
    - uses: docker/build-push-action@v6
      with:
        context: services/${{ matrix.service }}
        ...
```
