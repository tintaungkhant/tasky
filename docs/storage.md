# SQLite storage foundation

`internal/store` provides persistent identities and associations for the CLI
revamp. It is a storage API, not yet connected to commands. Existing commands
continue to use YAML configuration and legacy Task directories. Registration,
legacy adoption, and command integration are tracked in issues #3–#5; this
foundation does not import, move, or modify existing work.

Call `store.Open(path)` with an explicit database filename, then close the Store
when finished. Opening creates missing parent directories and initializes schema
version 1 in a transaction. There is no default database location yet. Unsupported
schema versions and invalid databases return errors rather than being replaced.
The pure-Go `modernc.org/sqlite` driver avoids a C compiler requirement.

## Identity and associations

Roots, Repos, Tasks, and worktree associations have opaque, persisted internal
IDs. A Task's user-facing Task ID is a separate string, unique within its Root.
The same Task ID in another Root identifies a different Task. Names can contain
Unicode, spaces, and punctuation; neither names nor paths determine a Task's
internal ID. Queries use persisted IDs, never directory-name parsing.

A Repo belongs to one Root. A worktree association links one Task to one Repo
in that same Root. A Task can have several Repos and several worktrees from
each Repo. A worktree path can have only one association across the database;
conflicts are rejected without replacing existing records. No branches or live
Git status are stored.

Locations are stored as absolute, cleaned paths. Comparison keys ignore letter
case on Windows. The storage layer does not resolve filesystem aliases or check
Git registration: callers must verify locations with Git before recording them.
In particular, a stored path does not prove a directory currently contains the
original worktree. Missing paths do not delete persisted identities.

## Writes and recovery

SQLite constraints enforce uniqueness and same-Root references. Each creation
or association is one atomic statement. Every connection enables foreign keys
and waits up to five seconds for a competing writer. Schema initialization uses
an immediate transaction so concurrent opens cannot partially initialize it.
`ErrConflict` and `ErrNotFound` distinguish conflicting records from invalid
references; lock failures identify the operation and suggest retrying.

An interrupted write is rolled back by SQLite when the database is reopened.
Retrying a rejected write cannot overwrite the existing association. SQLite
transactions do not include Git operations; callers combining Git and storage
must report partial completion and provide recovery rather than assuming both
changes are atomic. Direct Git use remains independent of this database.
