# Exercise 03 — Write a Dockerfile from scratch

**Service:** `claims-service` (Python + Flask)
**Difficulty:** ★★★☆☆
**Time target:** 15–20 minutes

## The scenario

The `claims-service` runs fine locally with `pip install -r requirements.txt &&
python app.py`, but nobody has containerized it yet. The platform team is
adopting a checklist for new services. Your Dockerfile must satisfy it.

## The goal

Create a `Dockerfile` (and a `.dockerignore`) in this folder that satisfies
**every** item on the checklist:

- [ ] Uses a **slim or distroless** Python base (not full `python:3.12`)
- [ ] Pins a major Python version (no `python:latest`)
- [ ] Has a `WORKDIR`
- [ ] Installs dependencies in a separate layer from app code (so editing
      `app.py` doesn't reinstall packages)
- [ ] Uses `--no-cache-dir` for pip
- [ ] Sets `PYTHONDONTWRITEBYTECODE=1` and `PYTHONUNBUFFERED=1`
- [ ] Runs as a **non-root user** with a real home directory
- [ ] Exposes port 5000
- [ ] `CMD` uses exec form (JSON array, not shell)
- [ ] `.dockerignore` excludes `__pycache__`, `.git`, `.venv`, `*.pyc`,
      `.pytest_cache`, and the Dockerfile itself

## Verify

```bash
docker build -t claims-fix .
docker run --rm -p 5000:5000 claims-fix
# in another terminal:
curl http://localhost:5000/health   # {"status":"ok","service":"claims-service"}
curl http://localhost:5000/claims/C-2001
```

Then verify it's running as non-root:
```bash
docker run --rm --entrypoint id claims-fix
# uid should NOT be 0
```

## Done looks like

- All checklist items met.
- Health check passes.
- You can explain why each item is on the list (interview talk-out-loud).

When you're done, compare with `SOLUTIONS/03-from-scratch/`.
