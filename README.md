# tasky

Task-oriented CLI on top of Git worktrees. Give it a task ID, a repo, and a branch; it creates a correctly named branch and a correctly placed `git worktree` — no manual path or branch computation.

```
~/Work/company/                          ← Root
├── api/                                 ← Repo
├── web/                                 ← Repo
└── tasks/                               ← Tasks Directory
    └── CRO-001@refunds/                 ← Task (ID "CRO-001", name "refunds")
        ├── api@task-CRO-001-fix-refund/ ← worktree on task/CRO-001/fix-refund
        ├── web@task-CRO-001-refund-ui/  ← worktree on task/CRO-001/refund-ui
        └── CRO-001@refunds.code-workspace
```

## Why

- **No more stash-and-switch.** Every task gets its own checkout per repo, so jumping between tickets never disturbs in-progress work.
- **One task, many repos.** A ticket touching `api` and `web` keeps both worktrees side by side in one task folder.
- **Consistent branch names.** A per-Root Branch Template (e.g. `task/{task-id}/{branch}`) is applied automatically.
- **Clean teardown.** Remove a task's worktrees — and optionally their branches — in one command; empty task folders are cleaned up.
- **Editor-ready.** Generate a multi-root `.code-workspace` so VS Code (or Cursor, Windsurf, VSCodium…) sees every repo's Git state at once.

## Install

Requires Go 1.27+ and `git`.

```sh
go install github.com/tintaungkhant/tasky@latest
```

Or from source:

```sh
git clone https://github.com/tintaungkhant/tasky
cd tasky
go build -o tasky .
```

## Quick start

```sh
tasky setup                                   # configure a Root once
cd ~/Work/company/api
tasky add CRO-001 fix-refund --name refunds   # create worktree for this repo
tasky add web CRO-001 refund-ui               # same task, another repo
tasky vscode CRO-001                          # write a workspace file
tasky delete CRO-001 --delete-branch          # done: tear it all down
```

## Concepts

| Term | Meaning |
|------|---------|
| **Root** | A top-level directory (e.g. `~/Work/company`) holding one or more repos. tasky picks the Root containing your current directory by longest path-prefix match. |
| **Repo** | A Git repository directly under a Root (must contain `.git`). |
| **Task** | A unit of work (e.g. a Jira ticket) that may span several repos. Its worktrees live under `<tasks_dir>/<task-id>[@<name>]/`. |
| **Task ID** | The literal string you type to name a task. Used verbatim — never truncated or parsed. |
| **Task name** | Optional human-readable label, stored in the folder name as `<task-id>@<name>`. |
| **Tasks Directory** | Per-Root directory where task folders are created (default `<root>/tasks`). |
| **Branch Template** | Per-Root pattern rendered into the branch name. Placeholders: `{task-id}`, `{task-name}`, `{branch}`. Substituted verbatim with no sanitization — Git rejects invalid names itself. |

Worktree paths are `<tasks_dir>/<task-dir>/<repo>@<branch>`, with `/` in the branch replaced by `-`. Because the branch is part of the path, one task can hold several worktrees of the same repo on different branches.

## Configuration

`tasky setup` is an interactive menu to add, edit, or remove Roots. Config is saved to `~/.config/tasky/config.yaml` (override with `$TASKY_CONFIG`):

```yaml
roots:
  - name: company
    path: /Users/me/Work/company
    tasks_dir: /Users/me/Work/company/tasks
    branch_template: task/{task-id}/{branch}
```

When adding a Root, `tasks_dir` defaults to `<path>/tasks` and `branch_template` defaults to `{branch}` (no prefix).

## Commands

All commands except `setup` must be run from inside a configured Root (in a repo or the Root directory itself).

### `tasky add [repo] <task-id> <branch>`

Create a task-scoped worktree.

- **0 args** — fully interactive: pick or create a task ID, optional task name, base branch, branch name (as-is or with the template prefix), then confirm.
- **2 args** — `<task-id> <branch>`; repo is the one you're in.
- **3 args** — `<repo> <task-id> <branch>`; repo named explicitly (works from the Root directory).

If you run it from the Root directory without a repo arg, tasky prompts you to pick one.

Branch handling:
- A **new** branch name is rendered through the Branch Template and created from the base.
- An **existing** local branch is reused as-is (no template applied). tasky refuses if it's already checked out elsewhere and tells you where.

| Flag | Description |
|------|-------------|
| `--from <branch>` | Base branch for the new branch (default: current branch). |
| `--name <name>` | Human-readable task name. Ignored if the task already exists. |

### `tasky list [task-id]`

Show a task's worktrees with their repo, current branch, and path.

- No args — interactive, filterable task picker.
- `<task-id>` — show that task.
- `--all` — print every task as plain text.

### `tasky vscode [task-id]`

Write `<task-dir>.code-workspace` inside the task folder, listing every worktree as a workspace folder. Prompts for the task when none is given.

```sh
tasky vscode CRO-001
code ~/Work/company/tasks/CRO-001@refunds/CRO-001@refunds.code-workspace
```

### `tasky delete [task-id] [repo] [branch]`

Remove worktrees, and optionally their branches.

- **No args** — interactive: pick a task, then the whole task or a single worktree; confirm; asked per worktree whether to delete its branch too.
- `<task-id>` — remove every worktree in the task.
- `<task-id> <repo>` — remove that repo's worktree (errors if there are several; add a branch).
- `<task-id> <repo> <branch>` — remove that exact worktree.

The task folder is removed once it's empty.

| Flag | Description |
|------|-------------|
| `--delete-branch` | Also delete each removed worktree's branch (non-interactive mode). |
| `--force` | Force removal of worktrees/branches with uncommitted or unmerged changes. |

### Shell completion

Task IDs complete for `list`, `vscode`, and `delete`; `delete` also completes repo and branch from the chosen task.

```sh
# zsh
tasky completion zsh > "${fpath[1]}/_tasky"
# bash
tasky completion bash > /etc/bash_completion.d/tasky
# fish
tasky completion fish > ~/.config/fish/completions/tasky.fish
```

## Development

```sh
go test ./...
```

Layout:

```
cmd/              Cobra commands (add, list, vscode, delete, setup, completion)
internal/config/  Config load/save and Root matching
internal/git/     Thin wrappers over git commands
internal/task/    Pure domain logic: branch rendering, task/worktree paths
internal/ui/      Terminal styling
```

Domain vocabulary is in [docs/glossary.md](docs/glossary.md).
