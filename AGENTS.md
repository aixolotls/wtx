# Task Management

- Use Linear as the single source of truth for tasks in this repository.
- Tag all `wtx` repository tasks with the Linear issue label `wtx`.
- Prefer creating/updating/closing Linear issues instead of maintaining a local task list for active work.

# Testing Notes

- `local-e2e` tests are local-only and must be run manually with `make local-e2e` (or `go test -tags local_e2e ./e2e` with `WTX_LOCAL_E2E=1`).
- Keep `local-e2e` scenarios isolated from this repo by creating and using temporary test repositories.
- For any code changes in this repository, run `make local-e2e` before reporting completion (unless the user explicitly asks to skip it).

# UX Notes

- In action-driven flows, selecting an action that needs extra input should open a dedicated follow-up popup/screen instead of embedding the form in the action list.
- For git command failures shown to users, surface the real stderr/stdout message (not just generic exit codes like `exit status 128`).
