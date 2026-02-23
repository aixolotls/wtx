# Task Management

- Use Linear as the single source of truth for tasks in this repository.
- Tag all `wtx` repository tasks with the Linear issue label `wtx`.
- Prefer creating/updating/closing Linear issues instead of maintaining a local task list for active work.

# Testing Notes

- Keep e2e scenarios isolated from this repo by creating and using temporary test repositories.
- For any code changes in this repository, run `make e2e` before reporting completion (unless the user explicitly asks to skip it).
