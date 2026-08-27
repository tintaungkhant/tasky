# tasky MVP — Design Spec

Date: 2026-08-27

## 1. Problem & Scope

`tasky` is a task-oriented CLI on top of Git worktrees. A single task (e.g. a Jira ticket) often spans multiple repositories; `tasky` removes the need to manually compute filesystem paths and type long `git worktree add` commands.

MVP scope: two commands, `tasky setup` and `tasky add`. No worktree listing, removal, or lifecycle management — those are explicitly deferred. Non-goals: no Git commit/push/pull/merge/rebase, no PR/Jira/CI integration.

## 2. Tech Stack

- **Language**: Go
- **CLI framework**: `spf13/cobra` — subcommand routing, flags, help text
- **Interactive prompts**: `charmbracelet/huh` — select/input/confirm forms
- **Config format**: YAML at `~/.config/tasky/config.yaml`
- **Git operations**: shell out to the `git` binary via `os/exec` (no git library — MVP only needs `worktree add`, branch listing, current branch, rev-parse)

## 3. Package Layout

```
cmd/                  cobra command definitions (thin — wire prompt/task/git together)
  root.go
  setup.go
  add.go
internal/
  config/             load/save YAML config, root matching
  git/                shell out to git: worktree add, list branches, current branch, repo root
  task/               pure logic: root/repo/task resolution, branch template rendering, worktree path calc
  prompt/             huh form wrappers used by cmd/add.go and cmd/setup.go
main.go
```

Layering rationale: `internal/task` is pure and unit-testable without touching git or a terminal. `internal/git` isolates all `os/exec` calls (tested against real temp repos). `internal/prompt` isolates huh so commands stay thin. `cmd/` only orchestrates.

## 4. Config Schema & Root Resolution

```yaml
# ~/.config/tasky/config.yaml
roots:
  - name: company
    path: ~/Work/company
    tasks_dir: ~/Work/company/tasks
    branch_template: "task/{task-id}/{branch}"
  - name: personal
    path: ~/Work/personal
    tasks_dir: ~/Work/personal/tasks
    branch_template: "{branch}"
```

- `path` — absolute root directory containing project repos
- `tasks_dir` — where task worktrees are created (commonly `<path>/tasks`, but stored explicitly to allow deviation)
- `branch_template` — Go-style placeholder string; supported placeholders: `{task-id}`, `{branch}`

**Root resolution algorithm:**
1. Find repo root via `git rev-parse --show-toplevel` from cwd (if inside a repo).
2. Match the repo root path (or, if not inside a repo, the cwd) against each configured root's `path` as a prefix. If multiple match, the longest `path` wins (handles nested roots).
3. No match → error: `no configured root contains this directory — run 'tasky setup'`.
4. Repo name = basename of the detected repo root (when inside a repo).

`tasky setup` is interactive-only (huh forms): list existing roots, add a new root (name, path, tasks_dir, branch_template — tasks_dir defaults to `<path>/tasks`, template defaults to `{branch}`), edit or remove an existing root. No non-interactive/flag form for MVP.

## 5. `tasky add` Flow

**Invocation forms:**
- `tasky add` — fully interactive
- `tasky add <task-id> <branch>` — run inside a repo; repo auto-detected from cwd
- `tasky add <repo> <task-id> <branch>` — run from a root directory; repo given explicitly
- `--from <base>` flag — explicit base branch, overrides current-branch default

**Field resolution order** (each field: explicit input wins, else inferred, else prompt):

| Field | Source priority |
|---|---|
| Repo | cwd detection → positional arg → prompt-select (list of subdirectories under the resolved root's `path`) |
| Task ID | positional arg → prompt-input |
| Base branch | `--from` flag → (interactive only) prompt-select of local branches, current branch pre-selected as default |
| Branch name | positional arg (used as-is) → prompt-input |

**New vs existing branch:**
- Fully-interactive mode: after branch name is entered, check if a local branch with the *rendered* name already exists. If yes, skip the base-branch question (already answered or irrelevant) and treat as "use existing branch" — no `-b` flag. If no, proceed with base-branch selection and "create new branch" semantics.
- Arg-based mode: no existing-branch detection: the branch name argument is rendered through the template and passed straight to `git worktree add`; if it happens to already exist, git's own behavior applies (new branch creation with `-b` on an existing branch name errors — surfaced to the user, not swallowed).

**Branch name rendering:** apply the resolved root's `branch_template`, substituting `{task-id}` and `{branch}` verbatim (no truncation, no sanitization — see Task ID in [CONTEXT.md](../../../CONTEXT.md)). Example: template `task/{task-id}/{branch}`, task-id `CRO-001-payment`, branch `feat/payment` → `task/CRO-001-payment/feat/payment`. Invalid git ref characters are not validated by `tasky` — `git` rejects them and its error surfaces to the user.

**Repo validation:** whether the repo is cwd-detected, given as a positional arg, or picked from a prompt, `tasky` confirms it's an actual git repo (`.git` present / `git rev-parse --show-toplevel` succeeds) before proceeding — clear error otherwise. Matching a repo name from an arg is exact, case-sensitive. The root-level interactive repo picker lists only subdirectories of the root's `path` that contain `.git`, which naturally excludes `tasks_dir` and other non-repo clutter without a special-case rule.

**Worktree path:** `<tasks_dir>/<task-id>/<repo-name>`.

**Confirmation:** interactive mode only — before executing, print resolved Task/Repository/Base/Branch/Worktree path and prompt `Create worktree? [Y/n]`. Fully args+flags-specified invocations skip the confirmation and execute directly (MVP success criteria: `tasky add CRO-001-payment feat/payment` just works, no prompts).

**Git execution:**
- New branch: `git worktree add -b <rendered-branch> <path> <base>`
- Existing branch: `git worktree add <path> <rendered-branch>`

## 6. Error Handling

- Wrap all git command failures: capture stderr, surface a clean CLI error message — never dump raw `exec.ExitError`/Go stack traces.
- Worktree path already exists on disk → error before invoking git.
- Ambiguous "create new branch" where the rendered branch name already exists (arg-based mode) → let git's own error surface as-is (no silent overwrite, no magic reuse).
- No configured root contains the current directory → clear error pointing to `tasky setup`.
- `tasks_dir` missing on disk → create it (`mkdir -p`) automatically.
- Configured root `path` itself missing on disk → error (misconfiguration — do not auto-create; a missing root path likely means the config is stale).

## 7. Testing Strategy

- `internal/config` — table-driven unit tests: load/save round-trip, root-matching (including nested-root longest-prefix case, no-match case).
- `internal/task` — table-driven unit tests: branch template rendering, worktree path calculation. Pure functions, no mocking needed.
- `internal/git` — integration-style tests against real temp repos (`t.TempDir()` + `git init`), exercising actual `worktree add`, branch listing, current-branch detection. No exec mocking — real git is fast and authoritative.
- `cmd/` — thin glue layer; a couple of smoke tests only (e.g. `tasky add` with full args produces the expected worktree end-to-end in a temp root). Prompt-driven interactive paths are not unit tested for MVP (huh forms are hard to drive headlessly) — verified manually.

## 8. Out of Scope (MVP)

- `tasky list`, `tasky rm`, or any other worktree lifecycle command
- Non-interactive `tasky setup` (flags/config-file editing outside the TUI)
- Any VCS other than git
- Git commit/push/pull/merge/rebase, PR/Jira/CI/CD integration
