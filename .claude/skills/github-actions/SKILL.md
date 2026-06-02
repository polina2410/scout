---
name: github-actions
description: >
  GitHub Actions CI/CD specialist for Next.js. Use when setting up or modifying CI pipelines,
  adding workflow files, or debugging failing GitHub Actions runs. Trigger words: CI, github
  actions, workflow, pipeline, set up CI, failing action, add linting to CI.
argument-hint: create|review|debug
---

# GitHub Actions

$ARGUMENTS a GitHub Actions workflow for this Next.js project.

| Action | Description |
|--------|-------------|
| `create` | Scaffold a new workflow file |
| `review` | Audit an existing workflow for issues |
| `debug` | Diagnose a failing workflow run |

---

## Standard CI Pipeline for This Project

Every PR and push to `main` should run:

```
Install (cached) → Lint → Test → Build
```

### Workflow File: `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Setup pnpm
        uses: pnpm/action-setup@v4
        with:
          version: latest

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'pnpm'

      - name: Install dependencies
        run: pnpm install --frozen-lockfile

      - name: Lint
        run: pnpm lint

      - name: Test
        run: pnpm test:run

      - name: Build
        run: pnpm build
        env:
          # Add any public env vars the build needs
          NEXT_PUBLIC_API_URL: ${{ secrets.NEXT_PUBLIC_API_URL }}
```

---

## Core Principles

### Fail Fast
Order steps from cheapest to most expensive:
1. Install (cached — fast)
2. Lint (seconds)
3. Test (seconds–minutes)
4. Build (slowest — run last)

If lint fails, don't waste time running tests or build.

### Cache Dependencies
Always cache via `pnpm/action-setup` + `actions/setup-node` with `cache: 'pnpm'`. This saves 30–60s per run.

### Pin Action Versions
Use `@v4` tags — not `@latest`. Unpinned actions can break without warning.

### Secrets
- Store sensitive values in **GitHub repo Settings → Secrets and variables → Actions**
- Never hardcode secrets in workflow files
- Use `${{ secrets.SECRET_NAME }}` syntax
- `NEXT_PUBLIC_*` vars needed at build time must be passed as `env:` in the build step

### Minimum Permissions
Always set `permissions: read-all` at the workflow level, then grant only what each job needs. Never use broad write permissions by default:
```yaml
permissions:
  contents: read
  pull-requests: write  # only if the job posts comments
```

### No Publishing from CI
Never publish packages or deploy from a general CI workflow. Publishing must be a separate, manually triggered workflow with a scoped token — not the default `GITHUB_TOKEN`.

### Forked PR Secret Isolation
Never expose `secrets` to workflows triggered by `pull_request` from a fork — GitHub blocks this by default. Do not use `pull_request_target` unless you fully understand the security implications, and never check out untrusted fork code in that context.

### Dependency Review
Add the Dependency Review action to block PRs that introduce known vulnerabilities:
```yaml
- name: Dependency Review
  uses: actions/dependency-review-action@v4
  # runs automatically on pull_request events
```

---

## Common Patterns

### Run only when relevant files change
```yaml
on:
  push:
    paths:
      - '**/*.ts'
      - '**/*.tsx'
      - 'package.json'
      - 'pnpm-lock.yaml'
      - '.github/workflows/**'
```

### Cancel redundant runs (same PR, new push)
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

### Upload test results on failure
```yaml
- name: Upload test results
  if: failure()
  uses: actions/upload-artifact@v4
  with:
    name: test-results
    path: test-results/
```

---

## Checklist for New Workflows

- [ ] Triggers defined (`push`, `pull_request`, or both)
- [ ] Node version pinned (match `.nvmrc` or `package.json` engines if set)
- [ ] `pnpm install --frozen-lockfile` used — reproducible installs
- [ ] `pnpm/action-setup` + `cache: 'pnpm'` configured
- [ ] Steps ordered: lint → test → build
- [ ] Secrets referenced via `${{ secrets.* }}`, not hardcoded
- [ ] `concurrency` set to cancel stale runs
- [ ] Action versions pinned (`@v4` not `@latest`)
- [ ] `permissions: read-all` set at workflow level, elevated only where needed
- [ ] No publishing or deploying from this workflow
- [ ] Dependency Review action added for PRs touching `pnpm-lock.yaml`
- [ ] No `secrets` exposed to workflows triggered by forked PRs
