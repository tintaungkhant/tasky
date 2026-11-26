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
refresh removes the selected Root. Workflow screens follow in the GUI plan,
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
the existing [application operations](architecture.md).

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
Root identity selection, refresh, keyboard tabs, empty states, and errors.
The generated shadcn components emit Fast Refresh export warnings during lint;
they do not affect the production build.

After editing, rebuild the frontend, then rebuild/restart `tasky serve`; Go embeds
assets at compile time. Use the exact newly printed access URL. The local session
boundary deliberately does not permit a separate Vite origin or a CORS proxy.
Source maps and development servers are not shipped. Generated paths and
regeneration commands are recorded in `AGENTS.md`.

Browser/visual verification was not run (repository permission setting). To
inspect manually, run `go run . serve`, open its printed URL, check Root selection
and refresh, then use Tab and arrow keys through the section tabs. Section
content currently explains which workflows are coming next; it does not mutate
Tasks or registrations.
