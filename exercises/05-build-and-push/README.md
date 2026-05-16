# Exercise 05 — Write a build-and-push workflow from scratch

**Difficulty:** ★★★★☆ (the big one — most interview-relevant)
**Time target:** 25–35 minutes

## The scenario

The platform team wants every service to publish a Docker image to **GitHub
Container Registry (GHCR)** on every push to `main`. You're starting with a
blank `.github/workflows/` directory. Write the workflow.

## The goal

Create `.github/workflows/build-and-push.yml` (or copy your draft from this
folder once it's ready) that satisfies **all** of these requirements:

1. Triggers:
   - On push to `main`
   - On manual dispatch (`workflow_dispatch`)
2. A `test` job that:
   - Checks out the repo
   - Sets up Node 20
   - Installs deps and runs `npm test` inside `services/policy-api`
3. A `build-and-push` job that:
   - `needs:` the test job (don't ship untested code)
   - Sets up Docker Buildx
   - Logs into `ghcr.io` using `${{ secrets.GITHUB_TOKEN }}`
   - Generates tags + labels with `docker/metadata-action@v5`
     (include a `sha` tag and a `latest` tag for the default branch)
   - Builds and pushes with `docker/build-push-action@v6`
   - Uses **GitHub Actions cache for buildx** (`cache-from: type=gha`,
     `cache-to: type=gha,mode=max`)
4. Grants the right `permissions:` block (`contents: read`, `packages: write`)
5. Targets `services/policy-api/` as the build context

## How to verify

```bash
mkdir -p .github/workflows
cp exercises/05-build-and-push/build-and-push.yml .github/workflows/build-and-push.yml
git add .github/workflows/build-and-push.yml
git commit -m "ci: add build-and-push workflow"
git push
```

After a successful run, your image should appear under your GitHub profile →
**Packages** tab (or `https://github.com/<you>?tab=packages`).

## Talk-out-loud points (interviewers love these)

- Why split `test` and `build-and-push` into two jobs?
- Why use `metadata-action` instead of hand-rolling tag strings?
- Why `type=gha` cache vs `type=registry`?
- What's the difference between `permissions:` at workflow level vs job level?
- How would you fan this out to push images for **all four** services?

## Done looks like

- Workflow runs green on push to `main`.
- The image appears at `ghcr.io/<you>/<repo>/policy-api:latest`.

When you're done, compare with `SOLUTIONS/05-build-and-push/build-and-push.yml`.
