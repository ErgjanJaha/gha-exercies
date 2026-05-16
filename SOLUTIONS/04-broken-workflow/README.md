# Solution — 04 broken workflow

## The 5 bugs

| # | Original                                | Fix                                          | Why                                                                       |
|---|-----------------------------------------|----------------------------------------------|---------------------------------------------------------------------------|
| 1 | `on:\n  push\n    branches: [main]`     | `on:\n  push:\n    branches: [main]`         | Missing colon after `push` → the entire `on:` block is malformed YAML.    |
| 2 | `uses: actions/setup-node@v1`           | `uses: actions/setup-node@v4`                | `@v1` is years deprecated; uses Node 12 internals and breaks on modern runners. |
| 3 | `lint` job has no `actions/checkout`    | First step of `lint` is `actions/checkout@v4`| Without checkout, `services/policy-api/` doesn't exist in the runner workspace. |
| 4 | `needs: build` in the `test` job        | `needs: lint` (the job that does exist)      | The `needs:` graph refers to a job that isn't defined → workflow fails to start. |
| 5 | No `permissions:` block                 | `permissions:\n  contents: read\n  packages: write` | `docker/login-action` against `ghcr.io` with `GITHUB_TOKEN` needs `packages: write`. The default-permissive token was deprecated for new repos. |

## One-paragraph reasoning

GitHub Actions failures cluster into three flavors: **YAML you didn't notice
was malformed**, **action versions that have aged out**, and **assumptions
about implicit state** (the working directory, the available secrets, the
default token's permissions). This workflow had one of each. Adding the
top-level `permissions:` block is the subtle but interview-favorite fix —
GitHub tightened the default `GITHUB_TOKEN` permissions in 2023, so any
repo that pushes to GHCR now has to opt in to `packages: write` explicitly.
Putting it at workflow level (rather than per-job) is fine here because the
publish job is the only one that uses the token's elevated scope; either
works, job-level is slightly more conservative.

## Bonus: what's still imperfect

The solution above is the **minimal fix**. For full production polish you'd
also: add `concurrency:` to cancel superseded runs, pin third-party actions
to a SHA instead of a tag, cache npm via `actions/setup-node`'s `cache: npm`
option, and add a final job dependency on a docker build step before
`publish` actually pushes anything.
