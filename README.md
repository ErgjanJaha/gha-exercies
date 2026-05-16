# DevOps practice — Dockerfile & GitHub Actions drills

A sandbox repo for drilling Dockerfile and GitHub Actions skills ahead of a
live DevOps interview. Themed as an insurtech: four insurance microservices
(`policy-api`, `claims-service`, `quote-engine`, `portal`) and five
exercises with planted bugs / missing pieces to fix under pressure.

## Layout

```
.
├── services/                 # four working services — run via docker-compose
│   ├── policy-api/           # Node.js + Express                 (port 3000)
│   ├── claims-service/       # Python + Flask                    (port 5000)
│   ├── quote-engine/         # Go, plain net/http                (port 8080)
│   └── portal/               # Static HTML/JS served by nginx    (port 8000)
├── exercises/                # YOUR practice scenarios
│   ├── 01-broken-dockerfile/ # fix planted bugs (Node)
│   ├── 02-optimize-image/    # convert bloated single-stage → multi-stage (Go)
│   ├── 03-from-scratch/      # write a Flask Dockerfile from a checklist
│   ├── 04-broken-workflow/   # fix planted bugs in a GH Actions workflow
│   └── 05-build-and-push/    # write a build-and-push workflow from scratch
├── SOLUTIONS/                # peek AFTER you try — one folder per exercise
├── docker-compose.yml        # run all four services together
└── check.sh                  # build each image and curl /health
```

## Recommended order

Do them in numeric order — difficulty climbs and concepts stack.

| #  | Exercise              | Difficulty   | Time   | What it drills                              |
|----|-----------------------|--------------|--------|---------------------------------------------|
| 01 | broken-dockerfile     | ★☆☆☆☆ warm-up | 5–10m  | Reading a Dockerfile for common smells      |
| 02 | optimize-image        | ★★☆☆☆        | 10–15m | Multi-stage builds, distroless, layer size  |
| 03 | from-scratch          | ★★★☆☆        | 15–20m | Working from a production checklist         |
| 04 | broken-workflow       | ★★★☆☆        | 15–20m | GH Actions YAML, action versions, perms     |
| 05 | build-and-push        | ★★★★☆        | 25–35m | GHCR publish, buildx caching, metadata      |

Total time for one full pass: **~90 minutes**. Aim to do 2–3 passes before
the interview, dropping your times each round.

## How to practice each exercise

1. Read the exercise's `README.md`. **Don't peek at SOLUTIONS.**
2. Work in the exercise folder. Build the image / run the workflow.
3. Verify against the "Done looks like" checklist.
4. Compare your answer against `SOLUTIONS/<exercise>/`.
5. Read the one-paragraph reasoning — make sure you can articulate every choice
   out loud (this is what interviewers actually score).

## Talk-out-loud checklist (the meta-skill)

During the live interview, *narrate* what you're doing. Interviewers grade
how you think, not just whether your final answer is correct. Hit these as
you work:

- [ ] **State the goal** before typing: "I'm going to make this image build,
      run as non-root, and shrink to under 30 MB."
- [ ] **Call out what you notice**: "This `COPY . .` happens before
      `npm install` — that breaks layer caching, I'll reorder it."
- [ ] **Explain the trade-off** when you make a choice: "I'm using distroless
      instead of Alpine — gives up shell access, gains a smaller, safer image."
- [ ] **Verify, don't assume**: "Let me build it and curl /health to confirm."
- [ ] **Acknowledge what you'd do with more time**: "In production I'd also
      add a HEALTHCHECK, pin the Node version with `.nvmrc`, and cache npm
      via the actions/setup-node cache option."

The single biggest tell of a strong candidate is **building incrementally
and verifying as they go** — not writing a 40-line YAML, hitting "save", and
hoping. Type, run, observe, fix.

## Quick verification

```bash
# Build + smoke-test all four services in one command:
./check.sh

# Or stand them all up at once:
docker compose up --build
# then in another terminal:
curl localhost:3000/health   # policy-api
curl localhost:5000/health   # claims-service
curl localhost:8080/health   # quote-engine
curl localhost:8000/health   # portal
```

## Pushing to GitHub and practicing the PR flow

This repo only *truly* exercises GitHub Actions once it lives on GitHub.
See the "Push to your own repo" section at the bottom of this file, or the
instructions printed when this repo was generated.

### PR flow practice

After pushing once:

```bash
git checkout -b fix/01-broken-dockerfile
# edit exercises/01-broken-dockerfile/Dockerfile
git add exercises/01-broken-dockerfile/Dockerfile
git commit -m "fix(01): repair broken policy-api Dockerfile"
git push -u origin fix/01-broken-dockerfile
gh pr create --fill
# (or open the URL printed by 'git push' in your browser)
```

Then on the PR page: read the diff as a reviewer would, click into the
Actions tab on the PR, watch CI run. **This is exactly the interview
loop.** Repeat for each exercise on its own branch.

## Push to your own repo

```bash
# Create an empty repo on GitHub first (no README, no .gitignore), then:
git remote add origin git@github.com:<your-username>/devops-practice.git
git branch -M main
git push -u origin main
```

Once pushed, copy a fixed workflow into `.github/workflows/` to see Actions
actually execute:

```bash
mkdir -p .github/workflows
cp SOLUTIONS/04-broken-workflow/ci.yml .github/workflows/ci.yml
git add .github/workflows/ci.yml
git commit -m "ci: enable workflow"
git push
```

Then watch `https://github.com/<your-username>/devops-practice/actions`.
