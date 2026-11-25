# Adopting legacy configuration

Run `tasky import` to adopt YAML Roots, direct child Repos, and existing Task
directories into SQLite. It honors `$TASKY_CONFIG` and stores `tasky.sqlite3`
beside that file. The command first verifies the entire input, then records it
in one transaction. It does not change YAML, source files, branches, worktree
locations, or editor workspace files. A missing Tasks Directory means no legacy
Tasks; it is not created during import.

Legacy Task directories use `<Task ID>[@<name>]`. Duplicate Task IDs, names with
multiple `@` separators, symbolic links in the Tasks Directory, non-Git
directories inside a Task, and conflicts with stored identities require review.
Fix the reported ambiguity before retrying. Worktree folder basenames do not
determine Repo identity: Git verifies the checkout root, its registration, and
its primary Repo location, including nested and external Repos.

Imports preserve existing IDs. Repeating or concurrently running import creates
no duplicate records. A verification failure writes no mappings; a storage
failure rolls back the entire transaction. SQLite also rolls back an interrupted
transaction. Retry after fixing the reported problem. Import never overwrites
an existing name or reassigns a worktree to a different Task.

During this migration stage, existing `add`, `list`, `delete`, `vscode`, and shell
completion continue to use the unchanged YAML and legacy directories. Import
does not remove that compatibility path. Legacy commands can still modify Git
and directories; import only adds verified mappings, never removes missing ones.
SQLite-backed creation and readers are introduced in #5. Avoid mixing old and
new binaries once Task names become independent of directory names.

Rollback at this stage is to continue using the legacy CLI and YAML; neither
depends on the imported database. Preserve the database if any mappings should
be retained. Import is not a filesystem or Git backup.
