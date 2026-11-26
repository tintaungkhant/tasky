# tasky

Task-oriented CLI on top of Git worktrees. Give it a task ID, a repo, and a branch; it creates a correctly named branch and a correctly placed `git worktree` — no manual path or branch computation.

```
~/Work/company/                          ← Root
├── api/                                 ← Repo
├── web/                                 ← Repo
└── tasks/                               ← Tasks Directory
    └── .tasky/
        ├── format
        └── <internal Task ID>/         ← Task ID "CRO-001", name "refunds"
            ├── <opaque ID>/           ← api worktree
            ├── <another opaque ID>/   ← web worktree
            └── <internal Task ID>.code-workspace
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

For the local dashboard, run `tasky serve` and open its printed URL. The current
shell provides Root selection and navigation; workflow screens are in progress.
Built assets ship in the binary, so Node is only needed when changing frontend
sources. See [the service and frontend build guide](docs/service.md).

## Quick start

```sh
tasky setup                                   # configure a Root once
tasky import                                  # adopt YAML and any legacy Tasks
cd ~/Work/company/api
tasky add CRO-001 fix-refund --name refunds   # create worktree for this repo
tasky add web CRO-001 refund-ui               # same task, another repo
tasky vscode CRO-001                          # write a workspace file
tasky delete CRO-001 --delete-branch          # done: tear it all down
```

## Concepts

After [`tasky import`](docs/adoption.md) or [registration](docs/registration.md),
[Task commands](docs/tasks.md) use SQLite identity and mappings. Task names are
independent of paths. Repos and attached worktrees can be nested or external;
adoption preserves existing locations and Git state. Unadopted YAML Roots retain
the [legacy workflow](docs/adoption.md).
Use [`attach` and `detach`](docs/associations.md) to organize existing checkouts;
reassignment requires an explicit source Task identity.

| Term | Meaning |
|------|---------|
| **Root** | A registered grouping of Repos and Tasks. Select by current-directory context or `--root`; ambiguous context requires selection. |
| **Repo** | A registered Git repository, wherever its primary and linked worktrees live. |
| **Task** | A persisted identity with associations to worktrees in one or more Repos. |
| **Task ID** | A literal, Root-scoped identifier. Different Roots may reuse it. |
| **Task name** | Optional Unicode label independent of branches, paths, and identity. |
| **Tasks Directory** | Per-Root directory where task folders are created (default `<root>/tasks`). |
| **Branch Template** | Per-Root pattern rendered into the branch name. Placeholders: `{task-id}`, `{task-name}`, `{branch}`. Substituted verbatim with no sanitization — Git rejects invalid names itself. |

New worktrees have opaque paths under the Tasks Directory, so `foo/bar` and
`foo-bar` cannot collide. A Task can hold multiple worktrees from the same Repo.
Renaming a Task changes its label only.

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

Task commands resolve the registered Root from the current Repo or directory;
use `--root <name-or-ID>` to select explicitly. `add` also accepts
`--repo <name-or-ID>`. Names must be unambiguous; IDs resolve duplicate labels.
See [registration](docs/registration.md) and [Task workflows](docs/tasks.md).

Use [import](docs/adoption.md) for existing YAML Roots and Task directories,
[attach/detach](docs/associations.md) for existing checkouts,
[list/reconnect](docs/inspection.md) for external changes,
[pending/recover](docs/cleanup.md) for interrupted cleanup, and
[backup/restore](docs/backup.md) for mapping recovery. `setup` edits YAML settings;
it does not register or import mappings.

### `tasky add [repo] <task-id> <branch>`

Create a task-scoped worktree.

- **0 args** — fully interactive: pick or create a task ID, optional task name, base branch, branch name (as-is or with the template prefix), then confirm.
- **2 args** — `<task-id> <branch>`; repo is the one you're in.
- **3 args** — `<repo> <task-id> <branch>`; repo named explicitly (works from the Root directory).

With no arguments, Repo selection is interactive when needed. With explicit
arguments outside a registered Repo, supply the Repo argument or `--repo`.

Branch handling:
- A **new** branch name is rendered through the Branch Template and created from the base.
- An **existing** local branch is reused as-is (no template applied). tasky refuses if it's already checked out elsewhere and tells you where.

| Flag | Description |
|------|-------------|
| `--from <branch>` | Base branch for the new branch (default: current branch). |
| `--name <name>` | Human-readable task name. Ignored if the task already exists. |
| `--root <name-or-ID>` | Select a registered Root explicitly. |
| `--repo <name-or-ID>` | Select a registered Repo instead of the positional argument. |
| `--as-is` | Use a new branch name verbatim, without the Branch Template. |

### `tasky list [task-id]`

Show a Task's worktrees with their Repo, current branch, path, and local changes.
Missing, inaccessible, invalid, and unverified mappings include recovery guidance.

- No args — interactive, filterable task picker.
- `<task-id>` — show that task.
- `--all` — print every task as plain text.

### `tasky vscode [task-id]`

Write a multi-root workspace using every verified association, including external
worktrees. Prompts for the Task when none is given. Prints the output path; open
that path in VS Code, Cursor, Windsurf, or VSCodium. See [editor support](docs/editor.md).

```sh
tasky vscode CRO-001 --root company
# Open the printed .code-workspace path in your editor.
```

### `tasky delete [task-id] [repo] [branch]`

Remove worktrees, and optionally their branches.

- **No args** — interactive: pick a task, then the whole task or a single worktree; confirm; asked per worktree whether to delete its branch too.
- `<task-id>` — remove every worktree in the task.
- `<task-id> <repo>` — remove that repo's worktree (errors if there are several; add a branch).
- `<task-id> <repo> <branch>` — remove that exact worktree.

Only empty recognized Task directories are removed. Editor workspace files and
unrelated files outside removed checkouts remain. Registered cleanup journals its
intent and reports partial results with [recovery guidance](docs/cleanup.md).

| Flag | Description |
|------|-------------|
| `--delete-branch` | Also delete each removed worktree's branch (non-interactive mode). |
| `--force` | Force removal of worktrees/branches with uncommitted or unmerged changes. |
| `--force-worktree` | Override dirty-worktree protection only (registered Roots). |
| `--force-branch` | Override unmerged-branch protection only (registered Roots). |

### Shell completion

Task IDs complete for `list`, `vscode`, and `delete`; `delete` also completes repo and branch from the chosen task.
Completion respects current context and `--root`; duplicate Repo names complete
as IDs. Branch completion verifies live worktree identity.

```sh
# zsh
tasky completion zsh > "${fpath[1]}/_tasky"
# bash
tasky completion bash > /etc/bash_completion.d/tasky
# fish
tasky completion fish > ~/.config/fish/completions/tasky.fish
```

### `tasky serve [--port <port>]`

Run the protected local HTTP service, print its Dashboard access URL, and stop
with Ctrl+C. The default port is 9000; use `--port 0` for an available port.
The current service offers a connection page and status/context APIs; dashboard
screens follow in the approved GUI plan. See [local service](docs/service.md) for
session protection, API behavior, and startup details.

## Development

```sh
go test ./...
```

Layout:

```
cmd/              Cobra commands (add, list, vscode, delete, setup, completion)
internal/config/  Config load/save and Root matching
internal/git/     Thin wrappers over git commands
internal/store/   SQLite identities, associations, operation journals, backup/restore
internal/work/    Verified worktree lifecycle and recovery
internal/app/     Shared application operations and structured results
internal/web/     Protected loopback HTTP service
internal/registry/ Root/Repo registration and context selection
internal/adopt/   Legacy import verification
internal/task/    Pure domain logic: branch rendering, task/worktree paths
internal/ui/      Terminal styling
```

Domain vocabulary is in [docs/glossary.md](docs/glossary.md).
Storage guarantees are described in [docs/storage.md](docs/storage.md).
Application ownership is described in [docs/architecture.md](docs/architecture.md).
Run `go build ./...`, `go vet ./...`, and `go test ./...` before committing.
See [compatibility verification](docs/compatibility.md) for the behavior coverage.
