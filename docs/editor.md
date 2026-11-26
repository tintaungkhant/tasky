# Editor workspace files

`tasky vscode [task-id] [--root <name-or-ID>]` generates a multi-root
`.code-workspace` for VS Code, Cursor, Windsurf, VSCodium, and compatible editors.
Without a Task ID it prompts for a Task. Root selection follows
[registered context](registration.md), including external Repo worktrees.

Every registered Task association must pass live Git registration, Repo, and
recorded worktree identity checks. Missing, inaccessible, invalid, or unverified
associations fail the command with guidance and no new workspace file. Tasky does
not silently generate a partial workspace. Use [list/reconnect](inspection.md) or
[detach](associations.md) to resolve stale associations before retrying. Dirty
worktrees and detached HEAD are valid. Unassociated subdirectories are excluded.

The workspace contains every associated worktree, including external paths and
multiple worktrees per Repo. Paths are relative to the workspace when possible;
different filesystem volumes use absolute paths. Open the printed output path.
Tasky's inspection is a point-in-time check: changes made directly through Git
after inspection remain possible; regenerate after reviewing later changes.

New Tasks write `<Tasks Directory>/.tasky/<internal Task ID>/<internal Task ID>.code-workspace`.
For adopted Tasks whose first mapped worktree remains in the legacy Task
directory, generation retains `<legacy Task directory>/<legacy Task directory name>.code-workspace`.
Otherwise it uses the managed location. Renaming a managed Task leaves its
workspace location unchanged. The file contains folders only; editor preferences
belong to the user.

## Preserve and regenerate

An identical existing workspace is retained. Any differing file, including a
user-edited JSON or JSON-with-comments workspace, is preserved and generation
fails with instructions to move it aside. There is no force-overwrite flag.

After adding, detaching, or reconnecting worktrees, move the previous workspace
to a backup filename, rerun `tasky vscode`, and copy any desired settings to the
new file. Keep the backup until satisfied. Cleanup removes only empty recognized
Task directories and never deletes a workspace beside a checkout. A file inside
a checkout is subject to Git's normal worktree removal protections.

Publication writes and syncs a temporary file, then exclusively links the complete
file into place. Competing commands cannot replace existing output or expose a
partially written file. This needs a filesystem supporting hard links (such as
NTFS and ordinary Unix filesystems); unsupported filesystems fail safely.

## Unadopted YAML compatibility

Legacy `vscode` still uses the YAML Root and Task directory, but includes only
Git-verified checkout roots, reporting skipped arbitrary or invalid directories.
It now has the same preservation and publication rules as registered generation.
In particular, rerunning after adding a worktree requires preserving the existing
file first. Run [import](adoption.md) for persistent identity verification and
external-worktree associations; legacy directory discovery cannot prove historical
identity at a reused path.
