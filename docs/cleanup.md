# Safe cleanup

These operations have separate effects:

- `tasky detach <task-id> <path>` removes only an association.
- `tasky delete <task-id> [Repo] [branch]` removes verified worktrees and their
  mappings, retaining branches by default.
- Adding `--delete-branch` also requests deletion of each removed worktree's
  current branch. Detached HEAD has no branch to delete.

For registered Roots, cleanup checks the [recorded Git identity](inspection.md)
again immediately before removal. An unrelated replacement, invalid registration,
unverified legacy mapping, or changed branch is refused. `--force-worktree`
explicitly overrides Git's dirty-worktree protection. `--force-branch` overrides
Git's unmerged-branch protection. Neither bypasses identity checks. The older
`--force` flag overrides both protections for compatibility; use the narrower
flags when only one override is intended. Unadopted YAML Roots retain legacy
cleanup; import them to use persisted identity and recovery.

Tasky never recursively removes a Task directory. Only an empty recognized
parent directory may be removed. Unrelated files and editor workspace files
outside the removed checkout remain intact. Forcing removal explicitly discards
uncommitted and untracked files inside the selected checkout.

## Interrupted and partial operations

Schema v4 adds durable cleanup intent before Git runs. SQLite serializes cleanup
against other Tasky mapping writes. Pending locations cannot be detached,
reassigned, or reconnected until intent is recovered or explicitly forgotten.
Removing a Root with pending cleanup is refused.

`tasky pending --root <Root>` shows operation IDs, paths, branches, whether the
worktree removal was recorded, and whether branch deletion was requested. After
an interruption, the recorded phase may lag Git: recovery checks live registration
before continuing. A missing checkout with a remaining registration requires
manual Git review; an externally moved original checkout must be reconnected.

```
tasky recover <operation-ID> --root <Root>
tasky recover <operation-ID> --root <Root> --force-worktree
tasky recover <operation-ID> --root <Root> --force-branch
tasky recover <operation-ID> --root <Root> --forget
```

Each completed target is printed. A failure stops later targets and reports the
pending operation and recovery command. If worktree removal succeeds but branch
deletion fails, its mapping is removed and the branch step remains pending.
Recovery never deletes a branch whose commit changed after intent was recorded.
`--forget` abandons only intent; it never deletes files, mappings, or branches.
Inspect `tasky list` afterwards and detach stale mappings if necessary.

Git operations cannot share a transaction with SQLite. The journal covers crashes
between Git and mapping updates; Git's own locks and protection checks still
apply. Avoid moving or editing a checkout concurrently with its removal through
another Git process.
