# Existing worktree associations

Use a registered Root and Repo, or first adopt existing configuration with
`tasky import`. Association commands change SQLite mappings only; they never
move a checkout, switch or delete a branch, remove files, or write Git state.

```sh
tasky attach T-1 /elsewhere/existing-checkout --root company
tasky attach T-1 /elsewhere/another-checkout --root company --repo api
tasky detach T-1 /elsewhere/existing-checkout --root company
```

`attach` verifies an actual checkout root with Git, its live worktree
registration, and its registered Repo identity. Nested directories and unrelated
Repos are rejected. Without `--repo`, the supplied checkout's Git identity
selects the Repo within the chosen Root. Root context comes from the current
directory or `--root`. External paths and dirty worktrees are supported. Repos
must already be registered; attach does not guess membership from basenames.

The target Task is created atomically with the association if needed. `--name`
sets the name only for a new Task. Repeating the same attach returns the existing
mapping without duplicates or a rename. Many checkouts from one Repo can belong
to the same Task.

## Explicit reassignment

An attached checkout cannot silently move to another Task. A conflicting attach
prints the current **internal Task ID** and the required authorization flag:

```sh
tasky attach T-2 /elsewhere/existing-checkout --root company \
  --reassign-from <current-internal-Task-ID>
```

This internal ID is distinct from the user-facing Task ID. It identifies the
exact source Task, including across Roots that reuse public IDs. Review the
conflict before copying it. Cross-Root reassignment additionally requires the
checkout's Repo to be registered in the target Root.

The source identity is checked within the write transaction. If a competing
request has already moved or detached the mapping, a stale reassignment fails
without creating an empty target Task. Repeating an already completed request
for the same destination returns its existing association. Reassignment
preserves the worktree's internal ID and path; the source Task remains available.

## Detachment and recovery

`detach` removes only the association belonging to the specified Task. It keeps
the Task identity, checkout, files, and branch, including dirty changes. Repeated
detachment is harmless if there is still no association. A stale detach fails
if the location now belongs to another Task. Missing checkouts can be detached
using their recorded absolute path; no Git mutation is required.

Each attach, detach, and reassignment is a single SQLite transaction. New Task
creation and association changes commit together or roll back together. A
failure reports the conflict or storage error and leaves the previous mapping
intact. After an interrupted command, repeat it to observe or complete the
result; there are no filesystem or Git effects to compensate for. Pending
creation paths must first be handled with [`tasky recover`](tasks.md).

Detachment is different from `delete`, which removes a verified Git worktree,
and from `delete --delete-branch`, which also requests branch deletion. Mapping
commands never imply either destructive action.
