# Inspecting and reconnecting worktrees

`tasky list <task-id> [--root <Root>]` reads the current branch and local
changes (including untracked files) from Git. Detached HEAD is shown explicitly.
It distinguishes missing paths, inaccessible paths, invalid registrations, and
unverified mappings. Errors include recovery guidance; they never invent a branch.

Tasky records the native filesystem identity of each checkout's Git administrative
directory when creating, importing, or attaching it. Branch switches and
`git worktree move` preserve this identity. A different worktree at an old path
is rejected even if it belongs to the same Repo. Tasky does not write identity
markers into Git or the checkout.

After moving a checkout externally, use Git to repair its registration if needed,
then run:

```
tasky reconnect <task-id> <old-path> <new-path> --root <Root>
```

Reconnection verifies the same Repo and worktree identity and atomically updates
only the path. It does not move files, change branches, or reassign ownership.
An occupied destination, pending creation, or stale concurrent request fails
without changing mappings. Inspect `tasky list` before retrying.

Schema v3 preserves existing identities and mappings. Older mappings have no
recorded Git identity and remain unverified: Tasky cannot reconstruct historical
filesystem identity. Review the checkout and explicitly enroll it with
`tasky reconnect <task-id> <old-path> <verified-path> --accept-identity`.
This flag only applies when no identity was recorded; it cannot override a mismatch.
Recreated or copied Git administrative directories require detaching and attaching
after review. On Unix the identity uses device/inode; moving to a different
filesystem requires the same explicit detach/attach review.
