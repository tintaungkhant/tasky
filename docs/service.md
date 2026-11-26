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

The current service provides a connection page and protected status/context
endpoints. The React dashboard and workflow screens follow in the GUI plan,
[spec #22](https://github.com/tintaungkhant/tasky/issues/22). Existing CLI commands
remain available. No frontend build, Node installation, or background process is
needed to run the current service.

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
only the bundled bootstrap script and same-origin connections. No CORS access is
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
| `GET /` | First-party connection page; no session required. |
| `POST /api/session` | Same-origin `application/json` body with a `token` field from the access link; returns 204 and sets the session cookie. Unknown fields, malformed/trailing JSON, and bodies over 4 KiB are rejected. |
| `GET /api/status` | Authenticated `{ "status": "ok" }` response without database access. |
| `GET /api/context?root=…&repo=…` | Authenticated registered Root/Repo resolution through the shared registry. Both selectors are optional; absent selectors use the directory captured when `serve` started. |

Context requests accept only one `root` and one `repo` selector. Invalid query
encoding, duplicates, unknown parameters, and ambiguous or missing registrations
return actionable errors. Context data serializes the shared Registry Context
(`Root` and `Repo` records, with their existing exported field names). The database
is beside the configuration selected at startup by `$TASKY_CONFIG`; request-time
environment changes do not switch it. Context lookup opens the Store and preserves
the CLI's registration and selection rules. All test databases are disposable.

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
apply. Dashboard rendering verification belongs to the frontend work.
