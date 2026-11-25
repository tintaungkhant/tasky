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

After import, existing commands use [persisted identities and paths](tasks.md).
Unadopted YAML Roots retain their legacy workflow. The marked managed `.tasky`
directory is excluded from import, so new managed checkouts are not interpreted
as legacy Tasks. Import never removes missing mappings or overwrites names;
renaming an adopted Task can therefore require review before a later import.

YAML and adopted paths remain unchanged. Older binaries can still use the
original legacy layout, but cannot interpret newly created managed checkouts or
renamed Task metadata. Avoid mixing binary versions after adoption. Preserve the
database to retain new identities and associations; import is not a filesystem
or Git backup and cannot reconstruct forgotten managed mappings.
