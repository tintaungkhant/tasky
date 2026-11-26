# Application operations

`internal/app` coordinates registered Task operations independently of Cobra,
interactive prompts, output writers, and process-wide command flags. CLI adapters
collect selections and confirmations, call these operations, and format the
returned values and errors. `internal/web` provides the protected local HTTP
service and uses shared Registry context resolution. Dashboard workflow handlers
reuse the existing application and domain operations; see [service.md](service.md).

## Context and ownership

`app.OpenRegistered(configPath, cwd, rootSelector)` takes the configuration filename
and directory context explicitly. Callers supply absolute paths; the CLI resolves
`config.Path()` and `os.Getwd()` at its boundary. The returned Context owns an open
Store that the caller must close. A nil Context retains the unadopted YAML CLI
workflow; an explicit or registered Root failure does not silently fall back to
directory names. Context-relative association paths use its supplied directory,
not a process-wide `chdir`. `config.LoadFile` and `Config.SaveFile` support explicit
configuration locations; existing Load/Save preserve environment-based CLI use.

`app.UpdatePreferences` validates the dashboard's absolute local paths and
preference fields, then uses `config.SaveFileIfUnchanged`. Config owns locked,
complete file publication and stale-list detection; CLI `setup` uses that same
save boundary. Registry still owns registration/Git validation. Store owns
transactionally consistent Root removal previews and conditional deletion, so
HTTP confirmation cannot forget mappings created since the preview.

Read-only Task discovery lists mappings through Store and calls `work.Manager.Inspect`,
the same inspector used by CLI listing. It requires an exact Root identity and
does not need YAML settings or a reachable Root directory to report broken locations.
HTTP and frontend code do not implement Git validation or identity checks.

HTTP creation validates exact Root/Repo membership before opening the shared
application Context, so form values cannot fall back to directory or name
selection. Options expose that Context's effective settings and live Git branch
list; submission calls `Context.Create` again. Rename scopes an internal Task ID
to its Root and calls Store's label-only `RenameTask`, also used by the CLI's
application operation, so a removed Task's public ID cannot retarget a stale form.

Association previews and submissions share `Context`'s checkout verification
with CLI attach. Store owns the atomic reviewed target/source comparison and
snapshot detach. HTTP constructs a Context from exact persisted Root/Repo IDs
without loading unrelated YAML creation settings. Reconnect passes the inspected
mapping snapshot through `work.Manager.Reconnect` to Store's compare-and-swap;
HTTP does not resolve the old mapping again by public Task ID or path.

Editor generation opens the configured Context for the exact Root and selects
the Task by internal identity. Both callers share the same association snapshot,
live inspection and workspace publisher; HTTP accepts no command to execute.

The existing packages remain responsible for their invariants:

- `registry`: Root/Repo registration, filesystem/Git identity, and context selection.
- `work`: verified Git creation, inspection, reconnection, removal, and recovery.
- `store`: SQLite identities, same-Root associations, conflicts, journals, and atomic writes.
- `adopt`: legacy import verification.
- `config` and `task`: YAML preferences and shared domain calculations.

The application Context combines these rules for Task lookup, branch selection,
creation and renaming, attach/detach, reconnect, cleanup, recovery, and workspace
generation. Creation requests and cleanup/recovery options are explicit values.
Removal receives the expected mapping snapshot; journal checks still reject stale
ownership or location changes. CLI target selection and prompts remain adapters;
worktree identity checks and dirty/unmerged protections remain in `work`.

## Files and diagnostics

Workspace generation returns its path and whether identical output already
existed. The shared publisher syncs a complete file and uses exclusive publication
to preserve existing files. Workspace content and preservation rules are described
in [editor.md](editor.md).

Backup decoding accepts a bounded reader, so CLI files and future HTTP uploads use
the same JSON validation and 64 MiB limit. Restore returns structured location
warnings after the Store's atomic merge. CLI adapters print those warnings;
future handlers can serialize them without parsing terminal output. The snapshot
format and pending-operation safeguards remain owned by the Store; see
[backup.md](backup.md).

`app.Import` accepts an explicit configuration path, verifies the entire legacy
input before opening/writing mappings, and returns the adopted Root count. No new
Git markers, database schema, dependencies, background processes, or public CLI
commands are introduced by this boundary.

## Verification

`cmd/app_test.go` calls the application directly with a different process directory
and configuration environment, then checks that CLI inspection and workspace
generation observe the same data. It also exercises context-relative attach,
reconnect, detach, dirty protection, and forgetting recovery intent against real
temporary Git Repos. Existing CLI lifecycle tests remain the behavior-preservation
boundary. `internal/app/backup_test.go` covers stream validation and oversized input.
