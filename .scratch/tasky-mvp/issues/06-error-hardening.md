# 06 — Error hardening across `add`

**What to build:** A cross-cutting pass making every failure mode in the `add` flow (args-based and interactive) fail with a clear, actionable message instead of a raw Go/git error dump.

**Blocked by:** 05

**Status:** done

- [x] Worktree path already exists on disk → clear error before invoking `git`, no partial/corrupt state
- [x] Rendered branch name already exists → per ticket 04, this is handled uniformly (arg-based and interactive) by existing-branch reuse rather than a create attempt, so no create-time collision to surface; covered by `TestAddExistingBranchReuse`
- [x] No configured Root contains the current directory → clear error pointing to `tasky setup`
- [x] `tasks_dir` missing on disk → auto-created (`mkdir -p`) explicitly before use, no error
- [x] Configured Root `path` itself missing on disk → clear misconfiguration error, no auto-create
- [x] All `git` command failures across the flow surface wrapped stderr, never a raw `exec.ExitError`/stack trace
- [x] Test coverage for each error path above (`cmd/add_test.go`: worktree-exists, no-matching-root, tasks_dir-autocreate, root-path-missing, git-failure-wrapped)
