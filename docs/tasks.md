# Registered Task workflows

After `tasky import`, or after registering a Root and Repo, `add`, `list`,
`delete`, `vscode`, and completion use SQLite mappings for that Root. Unadopted
YAML Roots retain the legacy directory workflow. Import existing Tasks before
switching an existing YAML Root to registration-based use.

```sh
tasky add T-1 foo/bar --name 'Refund / ပြန်အမ်းခြင်း @ web'
tasky add T-1 foo-bar
tasky add T-1 another --root company --repo api --as-is
tasky rename T-1 'New / Task name'
tasky list T-1 --root company
```

Task ID is a literal metadata value, unique within its Root. Names can contain
Unicode, spaces, and punctuation. `rename` changes only the name: the internal
identity, branches, paths, and associations remain unchanged. An existing
Task's name is retained when `add --name` is supplied again.

New checkouts use `<Tasks Directory>/.tasky/<internal Task ID>/<opaque ID>`.
The managed `.tasky` namespace has a format marker and is excluded from legacy
import. A pre-existing unmarked `.tasky` directory requires review or a different
Tasks Directory. Display names and branches are never path components, so
`foo/bar` and `foo-bar` cannot fold into the same path. Directories are reserved
exclusively, and filesystem writes use Go's `os.Root` to constrain symlink
traversal. Missing Tasks Directories are created automatically. Adopted paths
remain unchanged; a Task may contain many Repos and many worktrees per Repo.

`--root` selects a registered Root by name or ID. `add --repo` (or the existing
positional Repo argument) selects a registered Repo; both flags allow use from
any directory. Without a selector, current-directory context is used. Ambiguity
requires explicit selection or an interactive choice. YAML still supplies
`tasks_dir` and `branch_template` when its Root path matches the registration.
Otherwise defaults are `<Root>/tasks` and `{branch}`.

Existing branches are reused verbatim. New branches use the Branch Template,
unless `--as-is` requests the literal argument. Template substitution remains
verbatim; Git validates branch names and its stderr is shown for invalid names.
An already-checked-out branch is rejected. `--from` selects the base for a new
branch. The interactive Task, name, Repo, base, and branch choices remain.

## Interrupted creation

SQLite schema version 2 adds creation intent records, reserving a Repo/branch
before Git runs. Competing creates cannot silently create duplicate mappings.
The verified checkout replaces its intent in one SQLite transaction. No Git
rollback or branch deletion is attempted after a failure.

```sh
tasky pending --root company
tasky recover <operation-ID> --root company
tasky recover <operation-ID> --root company --forget
```

Failures print the operation ID and path. `recover` records the checkout only
after verifying its Git registration, Repo, and requested branch. If Git never
created a valid checkout, inspect the path and `git worktree list`, then use
`--forget` to abandon intent. This preserves all files, branches, and worktrees.
Retry creation afterwards; an existing branch can be reused. An empty Task or
reserved directory can remain after a failed operation. These are visible
partial results, not automatically deleted data.

## Compatibility

Registered `list` and completion read metadata and persisted paths, never parse
managed directory names. `delete` uses verified Git locations and removes the
corresponding mapping only after Git succeeds. It preserves unrelated files and
editor workspace files; a failure reports partial removal. Comprehensive live
health reporting and cleanup refinements remain in #7–#8.

`vscode` includes mapped paths, including multiple checkouts per Repo. It keeps
the original workspace location for an adopted Task and otherwise writes under
the managed Task directory. An identical existing workspace file is retained;
a differing existing file is preserved with an error asking you to move or
preserve it before regeneration. Editor refresh improvements remain in #10.

Schema migration from version 1 to 2 is transactional and preserves IDs. Earlier
binaries reject the newer schema. Keep using the new binary for registered
Tasks; old binaries cannot interpret the managed layout. YAML and adopted legacy
worktrees remain usable independently, and ordinary Git use never depends on
the database. Do not run legacy import to recover deleted managed mappings.
