# tasky

Task-oriented CLI on top of Git worktrees — turns a task, a repo, and a branch into a correctly placed `git worktree add`, with no manual path or branch computation.

## Language

**Root**:
A configured directory (e.g. `~/Work/company`) that groups Repos and Tasks. Registered Repos may be nested inside it or live elsewhere. Registration context uses Repo membership first and otherwise requires exactly one containing Root. Legacy Task commands still use YAML and longest path-prefix matching; see [registration.md](registration.md).
_Avoid_: Workspace, project group

**Repo**:
A single Git repository registered with a Root, identified through Git across its primary and linked worktrees. Registration imposes no directory layout. Legacy Task commands still discover Repos directly under a YAML Root.
_Avoid_: Project, package

**Task**:
The unit of work a developer is doing, with a persisted internal identity and a Root-scoped Task ID. It can span multiple Repos and multiple worktrees per Repo. Its optional name is independent of branches and filesystem paths. New worktrees use an opaque managed layout; adopted worktrees keep their original paths. See [tasks.md](tasks.md).
_Avoid_: Ticket, story

**Task ID**:
A literal string the developer supplies to identify a Task within a Root (e.g. `CRO-001-payment`). Stored separately from the opaque internal identity and optional name, and substituted into `{task-id}` verbatim. Different Roots may reuse a Task ID. Only the legacy workflow uses Task IDs in directory names.
_Avoid_: Ticket ID, task slug

**Tasks Directory**:
The configured directory (per Root) where Task worktrees are created, e.g. `~/Work/company/tasks`. Stored explicitly per Root rather than always assumed to be `<root>/tasks`.
_Avoid_: Worktrees dir, tasks folder

**Branch Template**:
A per-Root string pattern (e.g. `task/{task-id}/{branch}`) that `tasky` renders into the actual Git branch name, substituting `{task-id}` and `{branch}` verbatim with no sanitization — invalid characters are left for `git` itself to reject.
_Avoid_: Naming convention, branch format
