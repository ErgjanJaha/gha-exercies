# Exercise 01 — Fix the broken Dockerfile

**Service:** `policy-api` (Node.js + Express)
**Difficulty:** ★☆☆☆☆ (warm-up)
**Time target:** 5–10 minutes

## The scenario

A junior engineer wrote this Dockerfile in a rush before going on PTO. It builds
*sometimes*, leaks the host user, and the image is 3x larger than it should be.
Make it production-quality.

## The goal

Edit `Dockerfile` in this folder until **all of the following are true**:

1. The image builds successfully: `docker build -t policy-api-fix .`
2. The container starts and stays up: `docker run --rm -p 3000:3000 policy-api-fix`
3. `curl http://localhost:3000/health` returns `{"status":"ok","service":"policy-api"}`
4. Inside the running container, `whoami` is **not** `root`
5. Re-running `docker build` after editing only `server.js` reuses the
   `npm install` layer (no re-install)

## Hints (peek only if stuck)

There are **5 distinct bugs** to find. They span:
- the base image
- working directory
- layer ordering / caching
- the user the process runs as
- the `CMD` instruction

## Done looks like

- All 5 checks above pass.
- You can articulate *why* each change matters (interview talk-out-loud).
- Image size is reasonable (`docker images policy-api-fix` — expect ~150–200 MB).

When you're done, compare with `SOLUTIONS/01-broken-dockerfile/Dockerfile`.
