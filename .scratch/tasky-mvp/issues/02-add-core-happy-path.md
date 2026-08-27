# 02 — `tasky add` core happy path (args-only, inside repo, new branch)

**What to build:** From inside a repo, `tasky add <task-id> <branch>` creates a new Git worktree for a new branch off the current branch, with zero prompts and zero confirmation. This is the MVP success-criterion path (spec §19): `cd ~/Work/company/project-a && tasky add CRO-001-payment feat/payment` produces a correctly placed worktree on a correctly named branch. Covers Root resolution from cwd, Repo name detection, Branch Template rendering, worktree path calculation, and `git worktree add -b`.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] `internal/task`: renders the resolved Root's Branch Template, substituting `{task-id}` and `{branch}` verbatim (no truncation/sanitization)
- [ ] `internal/task`: computes worktree path as `<tasks_dir>/<task-id>/<repo-name>`
- [ ] `internal/git`: `worktree add -b <rendered-branch> <path> <base>` invocation, shelling out to `git`, wrapping stderr into a clean error (no raw stack traces)
- [ ] `tasky add <task-id> <branch>` run inside a repo: Root auto-detected from cwd via `git rev-parse --show-toplevel` + root-match, Repo name = basename of repo root, base branch = current branch, no prompts, no confirmation — worktree created directly
- [ ] Table-driven unit tests for `internal/task` (template rendering, path calc)
- [ ] Integration test for `internal/git` worktree creation against a real temp repo (`t.TempDir()` + `git init`)
- [ ] Smoke test: `tasky add` with full args against a temp root produces the expected worktree end-to-end
