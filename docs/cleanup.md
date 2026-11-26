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
Schema v5 also records and rechecks the original Repo's Git administrative
directory identity before every recovery step. Even a clone with the same branch
name and commit at the old Repo path is refused. Pending cleanup created before
v5 lacks this proof: inspect Git and use `recover --forget`, then review any
remaining mapping and branch manually instead of replaying unverified intent.
`--forget` abandons only intent; it never deletes files, mappings, or branches.
Inspect `tasky list` afterwards and detach stale mappings if necessary.

## Dashboard

In Tasks, select the worktrees to remove and choose **Review cleanup**. The review
shows each exact mapping, checkout path, current branch and revision, whether
branches will be deleted, and the separate protection overrides. Confirm the
review before removal. Changing a selection or option requires another review.
Detachment has its own confirmation and only removes the selected association.

Cleanup rechecks the complete selection before starting and checks each mapping
again when reserving durable intent. A changed association, branch, revision, or
Repo identity invalidates the review. The server stops at the first failed target
and reports completed, failed, and unattempted targets with recovery instructions.
Keep the result visible until reviewed, then refresh Tasks. Partial completion is
not rolled back by recreating checkouts or deleting Git data.

The Recovery tab lists pending creations and cleanups for the selected Root,
including operation IDs, paths, branches, recorded phase, deletion intent, and
protection overrides. Refresh after external changes or an interrupted request.
Recovery uses the same verification and journals as `tasky recover`; service
restarts do not erase intent. Creation recovery records an already verified
checkout and never runs a new worktree creation command.

Confirm recovery or **Forget intent** separately. Forgetting never deletes files,
mappings, or branches; inspect Tasks afterwards and reconnect or detach stale
mappings as needed. Cleanup recovery exposes dirty-worktree and unmerged-branch
overrides separately. Neither overrides identity verification. Pending-operation
listing and recovery use persisted registration without requiring YAML preferences.

Git operations cannot share a transaction with SQLite. The journal covers crashes
between Git and mapping updates; Git's own locks and protection checks still
apply. Avoid moving or editing a checkout concurrently with its removal through
another Git process.
