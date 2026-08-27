# 06 — Error hardening across `add`

**What to build:** A cross-cutting pass making every failure mode in the `add` flow (args-based and interactive) fail with a clear, actionable message instead of a raw Go/git error dump.

**Blocked by:** 05

**Status:** ready-for-agent

- [ ] Worktree path already exists on disk → clear error before invoking `git`, no partial/corrupt state
- [ ] Rendered branch name already exists but flow is trying to create it fresh (arg-based, no existing-branch detection) → git's own error surfaces cleanly, no silent overwrite
- [ ] No configured Root contains the current directory → clear error pointing to `tasky setup`
- [ ] `tasks_dir` missing on disk → auto-created (`mkdir -p`), no error
- [ ] Configured Root `path` itself missing on disk → clear misconfiguration error, no auto-create
- [ ] All `git` command failures across the flow surface wrapped stderr, never a raw `exec.ExitError`/stack trace
- [ ] Test coverage (table-driven where feasible) for each error path above
