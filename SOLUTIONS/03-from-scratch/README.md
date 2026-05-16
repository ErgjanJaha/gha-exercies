# Solution — 03 Flask from scratch

## Checklist mapping

| Requirement                                          | Where in the Dockerfile / dockerignore                   |
|------------------------------------------------------|----------------------------------------------------------|
| Slim Python base                                     | `FROM python:3.12-slim` (~45 MB vs ~350 MB for full)    |
| Pinned Python major version                          | `3.12` — not `latest`                                   |
| `WORKDIR`                                            | `WORKDIR /app`                                           |
| Deps in a separate layer from app code               | `COPY requirements.txt` → `RUN pip install` *before* `COPY . .` |
| `pip --no-cache-dir`                                 | yes — no `~/.cache/pip` left in the image                |
| `PYTHONDONTWRITEBYTECODE=1` + `PYTHONUNBUFFERED=1`   | `ENV` block — keeps the image clean and logs flush       |
| Non-root user with real home                         | `useradd --create-home --shell /bin/bash app` + `USER app` |
| Expose 5000                                          | `EXPOSE 5000`                                            |
| Exec-form `CMD`                                      | `CMD ["python", "app.py"]`                               |
| `.dockerignore` covers Python noise                  | `__pycache__`, `*.pyc`, `.pytest_cache`, `.venv`, `.git` |

## One-paragraph reasoning

This is a "checklist Dockerfile" — every line earns its place by addressing a
concrete production concern. `python:3.12-slim` keeps the image small without
going all the way to Alpine (which uses musl and breaks some wheels). The
`PYTHONUNBUFFERED=1` flag is the difference between seeing logs immediately
in Kubernetes and waiting 30 seconds for stdout to flush. Installing
dependencies *before* copying source means a one-line code change rebuilds in
under a second instead of re-installing Flask. Running as a non-root user is
basic container hygiene — a process compromise shouldn't equal root inside the
container — and giving that user a real home with `--create-home` prevents
weird "permission denied on `~/.cache`" errors from libraries that write there.
The `.dockerignore` keeps the build context small and stops `__pycache__`
files from polluting the image with host-tied bytecode.
