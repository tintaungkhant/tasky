# Root and Repo registration

The registration commands use SQLite. Registered Roots use the
[Task workflows](tasks.md), including `add`, `list`, `delete`, `vscode`, and
completion. Use [`tasky import`](adoption.md) before registering an existing YAML
Root with legacy Tasks. Registration itself does not import those mappings.
`setup` continues to edit YAML preferences.

```sh
tasky root add company /work/company
tasky root list
tasky repo add api /work/company/services/api --root company
tasky repo add shared /elsewhere/shared --root company
tasky repo list --root company
tasky context --root company --repo shared
```

Roots must be existing directories. Repos may be nested anywhere or live outside
the Root. Git verifies the checkout; registration preserves all files, branches,
and worktrees. Registering a linked checkout records its primary worktree as the
Repo location. Registering that Repo again in the same Root returns a conflict,
including through a linked checkout or filesystem alias. A Repo may be registered
in multiple Roots; context then requires an explicit Root.

`root list` and `repo list` print IDs, names, and paths. Names are labels and need
not be unique. Use the persisted ID whenever names are ambiguous. Registration
does not derive names from directory basenames.

## Context selection

`repo add` and `repo list` accept `--root <name-or-ID>`. Without it they resolve
the Root from the current directory. `context` additionally accepts
`--repo <name-or-ID>` and prints the selected Root and Repo.

- Explicit selectors must match exactly one registration. An explicit Repo can
  identify its Root from any directory; use both flags if it belongs to multiple
  Roots or its name is duplicated.
- Otherwise, Git's common directory identifies the current Repo, including
  subdirectories and Git-registered linked worktrees outside the Root. A nested
  Git Repo has its own identity. No Task association is needed to resolve it.
- Registered Repo membership takes precedence over physical Root containment.
  Without a registered Repo match, exactly one containing Root is required.
  Overlapping Root directories return an ambiguity error.
- Repo selection requires an explicit selector or a current Git Repo match.
  An unregistered checkout is never treated as a registered Repo by its name.

Unknown and ambiguous contexts return errors showing the flags and list commands
needed to make a choice. Missing or unreadable Repo locations cannot supply Git
context; explicit selectors remain available to inspect their stored mappings.
See [inspection and reconnection](inspection.md) for live health reporting and
reconnecting mappings after external moves.

## Storage and removal

The database is `tasky.sqlite3` beside the YAML file: normally
`~/.config/tasky/tasky.sqlite3`. `$TASKY_CONFIG` changes the YAML filename and thus
the database directory. YAML is not read or modified by registration commands.
See [storage.md](storage.md) for storage guarantees.

```sh
tasky root remove company
```

This forgets the selected Root and **all its Repo, Task, and worktree mappings**
in one SQLite transaction. It does not delete code, directories, Git worktrees,
branches, or YAML entries. Re-registering creates new identities; it does not
recover forgotten Task mappings. The legacy `setup` removal only edits YAML.

Foreign keys and uniqueness constraints protect concurrent changes. A failed
registration cannot leave a partial record or overwrite another registration;
a removed Root cannot acquire orphan Repo records. Duplicate registrations
report a conflict; a busy database asks the user to retry. Failed removal rolls
back the entire mapping change. No registration operation writes Git state.
