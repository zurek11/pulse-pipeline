---
name: git-workflow
description: 'REQUIRED for ALL PRs, pushes, and commits. Invoke this skill before running any gh pr create or git push command.'
allowed-tools: Read, Grep, Bash(git:*), Bash(gh:*), Bash(make:*)
---

# Git Workflow

## Emoji Commit Messages

| Emoji | Type | Example |
|---|---|---|
| 🎉 | feat | `🎉 feat: add event tracking HTTP endpoint` |
| 🐛 | fix | `🐛 fix: resolve Kafka producer timeout on high load` |
| ♻️ | refactor | `♻️ refactor: extract MongoDB bulk writer into package` |
| 🧪 | test | `🧪 test: add table-driven tests for event validation` |
| 📝 | docs | `📝 docs: add architecture diagram to README` |
| 🔧 | chore | `🔧 chore: update Go dependencies` |
| 🐳 | docker | `🐳 docker: optimize multi-stage build for API` |
| 🏗️ | infra | `🏗️ infra: add GKE Terraform configuration` |
| 📊 | metrics | `📊 metrics: add events_processed_total counter` |
| 🚀 | release | `🚀 release: v0.3.0 — full pipeline with observability` |

## Pre-push Checklist

```bash
make test     # All Go tests pass
make lint     # golangci-lint clean
```

## Branch Flow

- `main` — stable, versioned releases only
- `feat/[description]` — new features
- `fix/[description]` — bug fixes
- `infra/[description]` — infrastructure changes
- Feature branches → PR → main. Adam reviews and merges.

## PR Creation

```bash
gh pr create \
  --title "🎉 feat: add event tracking pipeline" \
  --body "## What's new\n\n- Event tracking HTTP endpoint\n- Kafka producer integration\n- Unit tests for validation" \
  --reviewer zurek11
```

## Rules

1. Never commit directly to `main`
2. Emoji prefix on every commit
3. Keep commits atomic — one logical change
4. Double-check no secrets in diff before push
5. Run `make test && make lint` before every push
6. Always create PR with `--reviewer zurek11`
