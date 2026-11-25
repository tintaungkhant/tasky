# Mapping backup and restore

```
tasky backup <new-file.json>
tasky restore <file.json>
```

Both commands address all registered Roots in the database beside the YAML
configuration (`$TASKY_CONFIG` still selects that configuration location).
They do not depend on current-directory Root selection.

Backup captures Root, Repo, Task, and worktree identities, names, paths, and
associations in one consistent SQLite snapshot. The versioned JSON format is
independent of the SQLite file format. It contains no code, branches, working-tree
changes, editor workspaces, or YAML configuration. Keep the YAML configuration
separately to retain custom Tasks Directories and Branch Templates; without it,
registered Roots use the defaults described in [Task commands](tasks.md).

Backup publishes a complete, synced file atomically and never overwrites an
existing file. Choose a new filename for each backup. Publication uses a hard
link within the destination directory; choose a filesystem supporting hard links
(such as NTFS or ordinary Unix filesystems). On an unsupported filesystem it
fails safely. The temporary file is removed after publication or an ordinary
failure; an interrupted process may leave a `.tasky-backup-*` temporary file.

## Safe restoration

Restore validates the backup version, record identities, absolute locations,
and same-Root references. It rejects malformed JSON, unknown fields, trailing
data, and input larger than 64 MiB. It merges all records in one transaction:
identical existing records are accepted, while conflicting records cause the
entire restore to roll back. Existing mappings are never deleted, renamed, or
replaced. Repeating a successful restore is harmless. Concurrent mapping writes
serialize through SQLite; a busy database produces a retryable error.

After restoring, Tasky checks locations and Git registration and reports stale,
missing, invalid, inaccessible, or unverified worktrees for review. The original
recorded identity is retained; a replacement checkout is never automatically
enrolled. Use [inspection and reconnection](inspection.md), or detach and attach
after review when the original Git identity no longer exists. Git status reads
disable optional locks so verification does not refresh the index.

Backup and restore refuse to run while **any** Root has pending creation or
cleanup intent. Use `tasky root list`, then `tasky pending --root <Root>` and
resolve each operation with `recover` or explicitly abandon its intent with
`recover --forget`. This prevents a mapping-only snapshot from losing unfinished
operation recovery. A restore interrupted before transaction commit leaves
existing mappings intact; retry the same backup after checking the database.

## Database unavailable

Tasky reports the database location and suggests checking permissions, disk space,
and the Tasky version. A corrupted or unsupported database is never overwritten
by restore. Preserve that database and its SQLite journal files before moving
them aside and restoring into a fresh database location. Selecting another
`TASKY_CONFIG` path also selects a separate database, allowing review of a restore
without replacing the original database.

Ordinary Git commands continue working independently of Tasky's database.
Backup and restore do not relocate, create, remove, or modify Git worktrees,
branches, or source files. A mapping backup does not replace a code backup.
