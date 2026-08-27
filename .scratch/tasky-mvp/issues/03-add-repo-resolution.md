# 03 — `tasky add` repo resolution variants

**What to build:** `tasky add` works from a root directory too, not just from inside a repo. Repo can be given explicitly as an arg (`tasky add <repo> <task-id> <branch>`), or picked interactively from a prompt listing the root's git repos when run from the root directory with no repo detectable. Adds repo validation so a typo or non-git directory fails clearly instead of silently misbehaving.

**Blocked by:** 02

**Status:** done

- [x] `tasky add <repo> <task-id> <branch>` run from a root directory: Root auto-detected from cwd, Repo taken from the explicit arg (exact, case-sensitive match)
- [x] Repo validation: whichever way the repo was determined (cwd, arg, or prompt), confirm it's an actual git repo (`.git` present) before proceeding — clear error otherwise (bad arg, typo, non-repo dir)
- [x] `tasky add` run from a root directory with no repo arg: prompt-select listing only subdirectories of the root's `path` that contain `.git` (excludes `tasks_dir` and other clutter)
- [x] Unit/integration coverage for repo validation and the filtered listing logic
