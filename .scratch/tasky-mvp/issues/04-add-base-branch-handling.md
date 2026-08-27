# 04 — `tasky add` base branch handling

**What to build:** Explicit base-branch selection and existing-branch reuse. `--from <base>` lets the developer override the current-branch default. When the rendered branch name already exists as a local branch, `tasky` reuses it (`git worktree add <path> <branch>`, no `-b`) instead of trying to create it fresh.

**Blocked by:** 02

**Status:** done

- [x] `--from <base>` flag on `tasky add`: overrides current-branch-as-base default, used directly as the base for `git worktree add -b`
- [x] `internal/git`: list local branches, get current branch — used for base-branch resolution
- [x] Existing-branch detection: if a local branch matching the *rendered* branch name already exists, use it as-is (no `-b`, no base-branch question) instead of treating it as a new branch
- [x] `internal/git` integration tests: branch listing, current-branch detection, and worktree-add-without-`-b` against an existing branch, all against real temp repos
