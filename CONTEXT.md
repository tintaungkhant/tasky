# tasky

Task-oriented CLI on top of Git worktrees — turns a task, a repo, and a branch into a correctly placed `git worktree add`, with no manual path or branch computation.

## Language

**Root**:
A configured top-level directory (e.g. `~/Work/company`) containing one or more repos and a Tasks Directory. `tasky` picks the Root that contains the current repo by longest matching path prefix.
_Avoid_: Workspace, project group

**Repo**:
A single Git repository living directly under a Root. Must contain `.git` — `tasky` validates this before acting on a repo, whether it's cwd-detected, passed as an arg, or picked from a prompt.
_Avoid_: Project, package

**Task**:
The unit of work a developer is doing (e.g. a Jira ticket), identified by its Task ID, that may span one or more Repos. All worktrees for a Task live together under `<tasks_dir>/<Task ID>/`.
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
