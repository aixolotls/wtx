# WTX Compatibility Contract (Git State Matrix)

This document defines deterministic behavior for common repository states.

## Fresh clone with commits and origin configured
- Detection: `git rev-parse --is-inside-work-tree` succeeds, `HEAD` resolves to a commit, and `origin` exists.
- Expected behavior: regular checkout/create flows work with default base refs like `origin/main`.
- User guidance: none required on success.

## Repo with no commits (unborn HEAD)
- Detection: repository exists but `git rev-parse --verify HEAD^{commit}` fails.
- Expected behavior: creating a branch with unresolved remote base ref fails fast.
- User guidance: show actionable error suggesting `--from <local-branch>` or config update.

## Repo with no origin remote
- Detection: `git remote` does not include `origin` (or any remotes).
- Expected behavior: new-branch flows default to a local fallback base (typically `main`) when no remote exists.
- User guidance: if fallback is not what you want, pass `--from <local-branch>` (for example `--from main`) or update defaults in `wtx config`.

## Repo with detached HEAD
- Detection: `git rev-parse --abbrev-ref HEAD` returns `HEAD`/detached state.
- Expected behavior: existing branch checkout works; creating a new branch requires a resolvable base.
- User guidance: pass explicit `--from <base>` when defaults are not resolvable.

## Repo with uncommitted changes
- Detection: `git status --porcelain` reports tracked/untracked changes.
- Expected behavior: do not silently delete or reuse dirty worktrees for destructive actions.
- User guidance: commit/stash changes or choose a clean target worktree.

## Empty directory initialized as repo
- Detection: `.git` exists and no commits are reachable.
- Expected behavior: same behavior as unborn HEAD.
- User guidance: create an initial commit or pass an explicit local base once available.

## Non-git directory
- Detection: no `.git` directory/file found while walking parent directories.
- Expected behavior: command fails immediately.
- User guidance: return `not in a git repository`.
