# Local HTTP service

```sh
tasky serve
tasky serve --port 9100
tasky serve --port 0
```

The default port is 9000. Port 0 asks the operating system for an available port.
Tasky prints a Dashboard access URL and stays in the foreground until Ctrl+C
(or SIGTERM). An occupied port produces an error suggesting `--port`; it never
silently selects another port unless 0 was requested. Shutdown waits up to five
seconds for active requests, then closes remaining connections and releases the
listener. Starting the service does not open or modify a database.

The service provides a React dashboard shell with explicit Root selection,
refresh, connection/error/empty states, and keyboard-accessible section tabs.
Root options include their name, path, and internal identity; duplicate names
remain independently selectable. Selection starts empty and is cleared if a
refresh removes the selected Root. Registration supports YAML preferences,
Root/Repo registration, verified legacy import, and confirmed Root mapping
removal. The Tasks tab searches registered Tasks, inspects live Git state,
creates worktrees, and renames Tasks. Association and recovery screens follow in the GUI plan,
[spec #22](https://github.com/tintaungkhant/tasky/issues/22); use the existing CLI
commands for those operations today. Production assets are embedded in the Go
binary. Users need no frontend build, Node installation, or separate service.

## Local access and sessions

The listener is fixed to `127.0.0.1` using IPv4 TCP. There is no host/address flag
or setting that enables network access. Use the exact printed address, rather
than replacing it with `localhost`, a hostname, or another port. The HTTP Host
must match the bound address and port; forwarded-host headers do not grant access.

The printed URL contains a randomly generated, per-run credential in its fragment
(`/#token=…`). Keep the full URL private. The first-party connection page removes
the fragment from the displayed URL, exchanges the credential through a same-origin
JSON POST, and establishes an HttpOnly, SameSite=Strict session cookie scoped to
`/api/`. Cookies are session-only and distinguish ports. All API routes except the
credential exchange require the session cookie. Reloads can reuse that session;
restarting the service invalidates previous access links and cookies.

The fragment never enters HTTP request URLs or referrers. Responses are not cached,
the connection page cannot be framed, and its content security policy permits
only same-origin scripts, styles, fonts, and connections (no inline scripts or
styles). No CORS access is
enabled. An unrelated Origin is rejected, state-changing requests require the
exact dashboard Origin, and browser requests to the API marked cross-site or
same-site rather than same-origin are rejected. These checks also apply to the
session exchange. The public connection page contains no configuration or Repo data.

This local session protects against unrelated websites and accidental requests;
it is not isolation from other processes running as the same operating-system
user. HTTP and its cookie are local-only; remote authentication and TLS hosting
are outside the approved scope.

## Current API

| Route | Behavior |
| --- | --- |
| `GET /` | Embedded dashboard shell; no session required and no configuration data. |
| `GET /assets/...` | Embedded production JS, CSS, and fonts. Missing assets return 404; no SPA fallback or directory listing. |
| `POST /api/session` | Same-origin `application/json` body with a `token` field from the access link; returns 204 and sets the session cookie. Unknown fields, malformed/trailing JSON, and bodies over 4 KiB are rejected. |
| `GET /api/status` | Authenticated `{ "status": "ok" }` response without database access. |
| `GET /api/roots` | Authenticated registered Roots as an array of `{ ID, Name, Path }`; no parameters. Empty registrations return `[]`. |
| `GET /api/preferences` | YAML preferences as `{ Roots: [{ Name, Path, TasksDir, BranchTemplate }] }`; missing configuration returns an empty list. |
| `PUT /api/preferences` | `{ expected: <loaded preferences>, config: <edited preferences> }`; validates and saves the entire YAML Root list, rejecting stale edits. |
| `POST /api/registration/roots` | `{ Name, Path }`; registers an existing Root directory, returning its identity. |
| `GET /api/repos?root=…` | Lists Repos for exactly one registered Root ID. Names and directory context are not selectors here. |
| `GET /api/tasks?root=…` | Exact Root ID; returns an array of `{ Task, Worktrees: [{ Mapping, Repo, State }] }` with fresh shared Git inspection. Empty lists are `[]`. |
| `GET /api/tasks/options?root=…&repo=…` | Exact Root and Repo IDs; returns `{ Branches, CurrentBranch, TasksDir, BranchTemplate }` from Git and effective application settings. |
| `POST /api/tasks/create` | `{ RootID, RepoID, TaskID, Name, Branch, Base, AsIs }`; creates one verified worktree through `app.Context.Create`, returning 201 with `{ Path, Branch }` only on completion. Name, Base, and AsIs are optional. |
| `POST /api/tasks/rename` | `{ RootID, ID, Name }`; renames an exact internal Task ID in the selected Root. Name is required but may be empty. Preserves branches, paths, and associations. |
| `POST /api/registration/repos` | `{ RootID, Name, Path }`; registers the verified primary Git worktree, including nested/external paths. |
| `POST /api/import` | Empty JSON object; runs shared verified legacy import against saved YAML preferences. |
| `GET /api/registration/removal?root=…` | Exact Root ID; returns the Root and every Repo, Task, and worktree mapping that removal would forget. Pending operations block the preview. |
| `POST /api/registration/remove` | `{ expected: <removal preview>, confirm: true }`; compares the full snapshot in the deletion transaction, rejects stale mappings or pending operations, and forgets only mappings. |
| `GET /api/context?root=…&repo=…` | Authenticated registered Root/Repo resolution through the shared registry. Both selectors are optional; absent selectors use the directory captured when `serve` started. |

Context requests accept only one `root` and one `repo` selector. Invalid query
encoding, duplicates, unknown parameters, and ambiguous or missing registrations
return actionable errors. Context data serializes the shared Registry Context
(`Root` and `Repo` records, with their existing exported field names). The database
is beside the configuration selected at startup by `$TASKY_CONFIG`; request-time
environment changes do not switch it. Context lookup opens the Store and preserves
the CLI's registration and selection rules. All test databases are disposable.
Root listing uses the same captured configuration's Store; its first request can
initialize a missing database, like context lookup. It does not import YAML Roots
or infer a selected Root from the service's working directory.

API errors are JSON with `error.code` and `error.message`. Authentication failures
are 401, Host/Origin failures 403, invalid input/context 400, wrong methods 405,
mapping conflicts 409, missing records/endpoints 404, and other operation failures
500. Wrapped operation messages retain location, partial completion, pending IDs,
and recovery instructions instead of reducing them to a generic error. Workflow
endpoints are added by subsequent tickets; they use the same error envelope and
the existing [application operations](architecture.md). Registration and Task mutations
require a single JSON body, reject unknown fields/trailing values/query parameters,
and limit bodies to 1 MiB. Root/Repo paths must be absolute. Input errors retain
the relevant path or validation message. Names are labels; registration APIs use
exact persisted IDs for existing records.

## Task creation and renaming

In Tasks, choose **Add worktree**, select a Repo by name/path/identity, and enter
a new or existing Task ID. The form displays the effective Tasks Directory and
Branch Template and provides native suggestions for local branches and bases.
Existing branches are reused verbatim. New branches use the template unless
**Use branch verbatim** is checked. An empty base uses the Repo's current branch;
if HEAD is detached, supply a base explicitly. Git validates the branch and base.
Refresh branch options after external Git changes; submission re-reads current
settings and Git state rather than trusting the displayed suggestions.

An optional Unicode name applies only to new Tasks; adding another worktree to
an existing Task keeps its saved name. Repeat the form to add worktrees across
Repos or multiple worktrees from one Repo. Each completed request reports its
path and branch and refreshes discovery. Managed paths and creation protection
remain those of the [registered Task workflow](tasks.md).

Creation is one worktree per request, with no batch rollback. Failures retain
inputs and show the complete operation error. If a Task record was established,
the error identifies it; a reserved creation includes its pending ID, location,
and `tasky pending`/`tasky recover` instructions. Earlier successful requests remain
completed. An empty Task, directory, branch, or checkout can remain after a
failure; refresh inspection and review pending intent before retrying. Recovery
currently uses the CLI. A disconnected request may also have completed: inspect
Git, Tasks, and pending intent before submitting again. Switching Roots or closing
the form does not cancel an already submitted operation in the original Root.

Select a Task to edit its optional name, including clearing it. Rename targets
the selected internal identity, never a public Task ID that could have been
re-used. It only updates the stored label and then refreshes inspection. It does
not require valid YAML preferences or reachable Git locations. Root and Repo
selectors for creation must be exact persisted IDs, and creation requires an
absolute Tasks Directory.

## Task discovery and inspection

Select a Root, then search by Task ID or optional Unicode name. Search filters
the loaded list in the browser without changing persisted names or IDs. Select
a Task with the native keyboard-accessible picker to see each associated Repo,
Repo path, worktree path, current branch (including detached HEAD), location
health, and clean or dirty/untracked Git porcelain status. Multiple worktrees
from the same Repo remain separate. A Task with no associations remains visible.

Refresh Tasks re-reads mappings and live Git state; external changes are not
polled automatically. The inspection is a point-in-time observation, not a lock
on Git. Switching Roots clears the search and Task selection, and late responses
from a previous Root are discarded. Refresh hides previous details while loading
or when it fails; deleted Tasks cannot remain selected after a successful refresh.
The same public Task ID in another Root is a different Task.

Inspection uses the existing `work.Manager.Inspect` checks. Missing, inaccessible,
invalid, and unverified mappings retain their paths and actionable CLI guidance;
untrusted locations do not display branch or clean status. Replacement checkouts
are rejected through recorded Git administrative identity. Inspection neither
repairs nor enrolls mappings. Use `tasky reconnect`, `attach`, or `detach` after
reviewing the reported problem. Discovery requires an exact persisted Root ID,
not a Root name or inferred current directory, and does not depend on YAML
preferences or the continued existence of the Root directory.

## Registration and preferences

The Registration tab separates three effects:

- YAML preferences: add, edit, or remove entries, including Tasks Directory and
  Branch Template. Entries are edited by list position, so duplicate names stay
  independently editable. Save applies the complete displayed list. Root paths
  must exist and be directories; Tasks Directory may be missing and is not created
  by saving. Names and Branch Templates cannot be blank. Templates are kept
  verbatim; Git validates rendered branch names when a Task is created.
- Registered mappings: add Roots, list/register Repos in the selected Root, or
  preview and confirm forgetting that Root and all its mappings. Registration
  does not edit YAML, move directories, or change Git. There is no individual
  Repo removal command in the current CLI/dashboard.
- Legacy import: adopts saved YAML configuration and verified existing CLI data
  through the same operation as `tasky import`. Save preference edits first.
  Import is repeatable; validation failures and conflicts are displayed without
  discarding inputs. See [adoption.md](adoption.md) for preservation guarantees.

YAML saving rewrites formatting/comments, like `tasky setup`. Saves publish a
synced, complete temporary file by rename and preserve existing file permissions
and configuration symlinks. A small `gofrs/flock` dependency supplies native
cross-platform file locking. The empty `<config path>.lock` sidecar stays on disk;
the operating system releases its lock when a process exits. Contending saves
return an actionable retry error. Dashboard forms and CLI `setup` compare their
original preference list under that lock before writing, so another Tasky save
requires reloading stale edits. External editors do not participate in this lock;
avoid saving the same file simultaneously outside Tasky.

Root removal shows names, paths, IDs, and counts for all affected mappings, then
requires an explicit checkbox before confirmation. SQLite compares the exact
preview and deletes in one writer transaction. New, renamed, reassigned, or moved
mappings invalidate the preview. The dashboard refuses removal with pending
creation or cleanup intent; inspect/recover it through the CLI first. Removal
preserves source files, worktree directories, branches, YAML preferences, and
editor workspace files. Re-registering cannot reconstruct forgotten identities.

## Verification

`go test ./internal/web -count=1` exercises real loopback HTTP requests and service
shutdown against disposable paths. `go test ./cmd -run TestServeCommand -count=1`
checks CLI startup output, flag validation, and cancellation. Tests close every
listener and use no browser. Required Go build, vet, and full test checks still
apply. Frontend behavior checks are described below; no browser is opened by them.

## Frontend development

The frontend was scaffolded with Vite's React/TypeScript generator and shadcn's
Radix Nova preset. The default shadcn Button and Tabs handle visual defaults and
tab keyboard behavior; the Root selector uses a native labeled select. Tailwind
provides layout without custom visual components. No router or client data-cache
library is needed for the current shell.

With Node 22.12+ and npm, from `frontend/`:

```sh
npm ci
npm run build
npm run typecheck
npm test
npm run lint
```

`build` type-checks and replaces `internal/web/dist/` with production HTML, JS,
CSS, and local fonts. Commit those assets with the sources: `go build ./...`
and `go install` then work without Node. `typecheck` runs TypeScript independently;
`test` runs Vitest/Testing Library in jsdom, checking session exchange, explicit
Root identity selection, refresh, keyboard tabs, empty states, and errors, plus
YAML editing, Repo registration, repeatable import, explicit removal consent,
Task search and selection, live refresh, unhealthy locations, stale Root responses,
creation forms and failure/retry states, and exact Task rename requests.
The generated shadcn components emit Fast Refresh export warnings during lint;
they do not affect the production build.

After editing, rebuild the frontend, then rebuild/restart `tasky serve`; Go embeds
assets at compile time. Use the exact newly printed access URL. The local session
boundary deliberately does not permit a separate Vite origin or a CORS proxy.
Source maps and development servers are not shipped. Generated paths and
regeneration commands are recorded in `AGENTS.md`.

Browser/visual verification was not run (repository permission setting). To
inspect manually, run `go run . serve`, open its printed URL, check Root selection
and refresh, then use Tab and arrow keys through the section tabs. On a disposable
Root, check Registration's YAML add/edit/remove, import, and Root/Repo forms;
inspect the full removal preview and checkbox before forgetting mappings. In Tasks,
search a Unicode name, select a Task with the keyboard, change its Git branch or
files externally, and click Refresh Tasks. Check detached HEAD and unavailable
location guidance on disposable worktrees. Recovery and Backup content still
explains which workflows are coming next. In a disposable Root, use Add worktree
with a new templated branch, an existing branch, and a verbatim branch/base.
Repeat across Repos, then rename the Task and confirm paths and branches stay
unchanged. A failed creation must show its error and any pending recovery guidance.
