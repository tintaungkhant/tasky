# tasky

Task-oriented CLI on top of Git worktrees: give it a task ID, repo and branch; it creates the named branch and worktree.

## Rules

Rules live in `.ai/rules/`, one topic per file. Each file starts with an `Applies to:` line. At the start of every task, read every rule file whose topic touches the task. When unsure whether a file applies, read it.

- `autonomy.md`: what you may do without asking, safety rules that always apply, `.env` files, disagreeing with the user, subagents.
- `engineering.md`: domain thinking, resource use, how much to build, comments, packages, library docs, generators, server-side validation.
- `verification.md`: reporting checks honestly, tests, visual checks, real side effects.
- `git.md`: branches, secrets, staging, commits, commit messages, worktrees.
- `docs.md`: keeping docs true, recording decisions and generated files.

## Checks

These must pass before a commit:

- `go build ./...`
- `go vet ./...`
- `go test ./...`

## Planner

Tickets and backlog live in: GitHub issues (gh). Planner skills: `ryan-plan` (plan a feature), `ryan-next` (next ticket), `ryan-remember` (park an idea), `ryan-backlog` (show the backlog), `ryan-talk` (discuss only).

Discussing is not planning. When the user explores an idea or asks "what if" (for example "what if we move from a VPS to k8s?"), answer and discuss only: no spec, ticket, plan or file. When the discussion ends, ask once in one line what to do with it: park it in the backlog, plan it now, record a settled decision in "Decisions", or nothing. "Nothing" is a normal answer.

## Decisions

Settled decisions. Do not reopen one unless the user does. When a decision changes, strike the old line through and add the date.

- Go, with `spf13/cobra` for commands and `charmbracelet/huh` for interactive prompts.
- Config is YAML at `~/.config/tasky/config.yaml` (`$TASKY_CONFIG` overrides).
- Git operations shell out to the `git` binary via `os/exec`; no git library.
- Task ID and branch are substituted into the Branch Template verbatim, with no sanitization; `git` rejects invalid names.
- Git failures surface as clean CLI errors carrying git's stderr, never a raw `exec.ExitError` or stack trace.
- A missing `tasks_dir` is created automatically.
- `internal/git` tests run against real temp repos (`t.TempDir()` + `git init`), no exec mocking. Interactive `huh` prompt paths are not unit tested.

Approved revamp direction (2026-11-25; CLI completed 2026-11-26):

- [Tasky CLI Revamp spec #1](https://github.com/tintaungkhant/tasky/issues/1) defines the accepted scope and acceptance criteria.
- Revamp the CLI first; GUI and local web server come later. No additional Workspace grouping layer.
- Each Task belongs to one Root and supports multiple Repos and multiple worktrees per Repo.
- SQLite owns Task identity and mappings independently of names and paths; Git remains authoritative for branch and worktree state.
- Preserve existing Git workflows and allow users to stop using Tasky without losing code, branches, or worktrees.

The CLI revamp and SQLite integration are complete. See [docs/storage.md](docs/storage.md) for current storage behavior and [docs/architecture.md](docs/architecture.md) for shared application ownership.

Approved GUI direction (2026-11-26; planned, not implemented): [spec #22](https://github.com/tintaungkhant/tasky/issues/22), tickets #23–#32.

- `tasky serve` runs a local-only service and prints its web dashboard URL, like MinIO's console; no desktop wrapper or remote access.
- Frontend: React + TypeScript + shadcn/ui + Tailwind. Built assets ship inside the Go binary; users need no Node installation or separate frontend service.
- The dashboard covers all current CLI workflows and uses explicit Root selection. Local paths are validated in Go; Git remains authoritative and external changes appear on refresh.
- CLI and HTTP callers share Go application behavior, including identity checks, cleanup protections, and recovery. The local service protects requests from unrelated websites.
- Desktop packaging, remote access, user accounts, cloud sync, terminals, and code editing are outside this GUI scope. Existing Git-operation and integration exclusions below still apply.

### Out of scope

Features deliberately left out or removed. Do not add them back without asking.

- Any VCS other than git.
- Git commit, push, pull, merge or rebase.
- PR, Jira or CI/CD integration.

## Generated files

Do not hand-edit these; a regenerate overwrites hand edits. Regenerate with the listed command.

- (none yet)

## Deploy

No deploy plan yet. Do not deploy or change servers or cloud resources. When the user is ready, they run `ryan-ship`.

## Domain terms

`docs/glossary.md` defines the domain terms (Root, Repo, Task, Task ID, Tasks Directory, Branch Template). Use them in code, tests, tickets and messages; avoid the synonyms it lists.

## Docs

`docs/` describes how the project works now. See `.ai/rules/docs.md`.
