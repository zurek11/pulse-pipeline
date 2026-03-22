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

---

## PR Creation

### Feature / Fix PR

For regular feature or fix work:

```bash
gh pr create \
  --title "🎉 feat: add event tracking pipeline" \
  --body "$(cat <<'EOF'
## Summary

- Event tracking HTTP endpoint added
- Kafka producer integration
- Unit tests for validation

## Test plan

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `curl /api/v1/track` returns 202

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)" \
  --reviewer zurek11
```

### Release PR

When completing a phase and cutting a new version, use **exactly** this format so all release PRs are consistent:

**Title:** `🚀 Release v{version} — {phase description}`

**Body:** The CHANGELOG.md section for that version verbatim, followed by the standard footer.

Extract the release notes automatically:

```bash
# Get the release notes for a specific version from CHANGELOG.md
RELEASE_NOTES=$(awk '/^## \[0\.1\.0\]/,/^## \[/' CHANGELOG.md | head -n -2)

gh pr create \
  --title "🚀 Release v0.1.0 — Phase 1: Foundation" \
  --body "$(awk '/^## \[0\.1\.0\]/,/^## \[/' CHANGELOG.md | head -n -2)

🤖 Generated with [Claude Code](https://claude.com/claude-code)" \
  --reviewer zurek11
```

Adjust the version pattern in the `awk` command to match the version being released.

**Rules for release PRs:**
1. Title always starts with `🚀 Release v{X.Y.Z}`
2. Title always ends with the phase name from CHANGELOG (e.g. `— Phase 1: Foundation`)
3. Body always opens with the verbatim CHANGELOG section for that version
4. Body always ends with the `🤖 Generated with Claude Code` footer
5. Always tag `--reviewer zurek11`

---

## Rules

1. Never commit directly to `main`
2. Emoji prefix on every commit
3. Keep commits atomic — one logical change
4. Double-check no secrets in diff before push
5. Run `make test && make lint` before every push
6. Always create PR with `--reviewer zurek11`
7. Update `CHANGELOG.md` before cutting a release PR