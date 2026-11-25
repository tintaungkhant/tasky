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
The unit of work a developer is doing (e.g. a Jira ticket), identified by its Task ID, that may span one or more Repos. All worktrees for a Task live together under `<tasks_dir>/<Task ID>[@<name>]/`.
_Avoid_: Ticket, story

**Task ID**:
A single literal string the developer supplies to name a Task (e.g. `CRO-001-payment`). Used verbatim in two places: as the Task's folder name under the Tasks Directory, and substituted into `{task-id}` in the Branch Template. Never truncated, parsed, or derived — if a developer wants a short branch name and a descriptive folder, they must type a short Task ID themselves.
_Avoid_: Ticket ID, task slug

**Tasks Directory**:
The configured directory (per Root) where Task worktrees are created, e.g. `~/Work/company/tasks`. Stored explicitly per Root rather than always assumed to be `<root>/tasks`.
_Avoid_: Worktrees dir, tasks folder

**Branch Template**:
A per-Root string pattern (e.g. `task/{task-id}/{branch}`) that `tasky` renders into the actual Git branch name, substituting `{task-id}` and `{branch}` verbatim with no sanitization — invalid characters are left for `git` itself to reject.
_Avoid_: Naming convention, branch format
