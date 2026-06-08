---
name: gh-issue-branch-pr
description: Opens a GitHub issue, keeps it updated as work proceeds, creates a branch named from the issue number and title, pushes commits, and opens a pull request. Use when the user asks to create an issue and PR, start work from an issue, or follow the issue-branch-push-PR workflow.
---

# GitHub issue → branch → push → PR

Complements [.cursor/rules/pr-issue-workflow.mdc](../../rules/pr-issue-workflow.mdc) (issue before PR, `Fixes #N` in commits, close issue only after green CI on `main`).

Use `gh` for all GitHub steps. Do not commit unless the user asks.

## Workflow

```
Task Progress:
- [ ] Step 1: Create issue
- [ ] Step 2: Branch from main using issue number + title slug
- [ ] Step 3: Implement and commit (Fixes #N)
- [ ] Step 4: Push branch
- [ ] Step 5: Create PR (title matches issue)
- [ ] Step 6: Keep issue updated until closed
```

The issue is the source of truth — not the PR description alone. Update it as work proceeds.

### Step 1: Create issue

```bash
gh issue create --title "Short imperative title" --body "$(cat <<'EOF'
## Problem
...

## Goal
...

## Acceptance criteria
- [ ] ...
EOF
)"
```

Record the issue number `N` from the returned URL.

### Step 2: Branch

Branch name: **`N/<title-slug>`** — issue number + slug of the issue title (lowercase, spaces → hyphens).

```bash
git fetch origin main
git checkout main && git pull origin main
git checkout -b N/short-imperative-title
```

Example: issue **#46** titled "Publish snapshot releases on push to main" → `46/publish-snapshot-releases-on-push-to-main`

### Step 3: Commit

```bash
git add <files>
git commit -m "$(cat <<'EOF'
feat(scope): concise summary

Optional body explaining why.

Fixes #N
EOF
)"
```

### Step 4: Push

```bash
git push -u origin HEAD
```

### Step 5: Create PR

PR **title** matches the issue title. Body references the issue.

```bash
gh pr create --title "Same as issue title" --body "$(cat <<'EOF'
## Summary
- ...

Fixes #N

## Test plan
- [ ] ...
EOF
)"
```

Add `--draft` only when the user requests a draft PR.

Then link the PR on the issue:

```bash
gh issue comment N --body "PR opened: https://github.com/OWNER/REPO/pull/XX"
```

### Step 6: Update issue as work proceeds

Comment or edit the issue at each milestone. Do not wait until merge.

| When | Action |
|------|--------|
| PR opened | Comment with PR link |
| Scope changes | Comment what changed and why; edit acceptance criteria if needed |
| Commits pushed | Comment brief summary if meaningful beyond the commit message |
| CI fails | Comment with failing job, link to run, and fix plan |
| CI green | Check off acceptance criteria verified in CI |
| Review feedback | Comment resolutions or open questions |
| Merged | Comment merge + `main` CI run link; close only after `main` is green |

**Comment** for progress notes:

```bash
gh issue comment N --body "CI green on PR #XX. Acceptance: snapshot tag publishes to GHCR."
```

**Edit** to check off acceptance criteria (preserve the rest of the body):

```bash
gh issue view N --json body -q .body   # read current body
gh issue edit N --body "$(cat <<'EOF'
...updated body with - [x] checked items...
EOF
)"
```

Reopen or keep open if `main` CI fails after merge; comment with the failure and fix-forward plan.

## PR creation checklist (parallel reads)

Before `gh pr create`, gather context:

```bash
git status
git diff main...HEAD
git log main..HEAD --oneline
```

## After merge

1. Comment on the issue with the merged commit and `main` CI run URL.
2. Close only when pipelines are green on `main` (`gh issue close N`).
3. If `main` fails, leave open, comment, fix forward, re-verify.
