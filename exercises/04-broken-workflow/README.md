# Exercise 04 — Fix the broken GitHub Actions workflow

**Difficulty:** ★★★☆☆
**Time target:** 15–20 minutes

## The scenario

This `ci.yml` was inherited from a contractor and has never run successfully.
Your job is to fix it so it lints, tests, and (eventually) publishes the
`policy-api` image. There are **5 distinct bugs**.

## The goal

Edit `ci.yml` in this folder until **all of the following are true**:

1. The YAML parses (no syntax errors)
2. The workflow runs on both `push` to `main` and any `pull_request`
3. Every job has the source code available (i.e. `actions/checkout` runs first)
4. All action versions are non-deprecated (no `@v1` on actively maintained actions)
5. The `needs:` graph references only jobs that actually exist
6. The job that talks to GHCR has the right `permissions:` block

## How to actually run it

Once you think it's fixed, copy your file into the real workflows directory:

```bash
mkdir -p .github/workflows
cp exercises/04-broken-workflow/ci.yml .github/workflows/ci.yml
git add .github/workflows/ci.yml
git commit -m "ci: enable workflow"
git push
```

Then watch the run at `https://github.com/<you>/<repo>/actions`.

## Hints (peek only if stuck)

Look for:
- A YAML key that's missing a colon
- A `uses:` that's pinned to a major version released years ago
- A job whose first step isn't `actions/checkout`
- A `needs:` that points to a job name that doesn't appear under `jobs:`
- A workflow that pushes to GHCR but doesn't grant `packages: write`

## Done looks like

- Workflow runs green on Actions.
- You can articulate what each bug was (interview talk-out-loud).

When you're done, compare with `SOLUTIONS/04-broken-workflow/ci.yml`.
