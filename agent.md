# PortD Agent Guide

## Product context

PortD is an internal project hub for Algérie Télécom internships. It gives
authenticated admins and interns a stable project URL instead of a raw
host-and-port combination, and gives administrators one place to manage
interns, projects, ports, runtime state, proxy routes, and activity history.

The source architecture is [agent_docs/System_overview.svg](agent_docs/System_overview.svg).
Treat it as the behavioural reference when implementing the product.

## Required stack

- Go for the HTTP server, business logic, filesystem work, process execution,
  Caddy integration, and deployment binary.
- `templ` for typed, server-rendered HTML components.
- Alpine.js for small client-side state and interactions.
- The Alpine AJAX plugin for progressive page and fragment updates. Prefer it
  over building a separate JSON SPA API for standard UI interactions.
- Keep templates in `views/layouts` and `views/pages`. Keep browser assets in
  `web/static`. Do not embed page HTML in Go strings.
- Load Alpine and Alpine AJAX from pinned local files in `web/static`. Use
  `x-data` for small UI state and `x-target` for progressive form updates.
  Server responses must support normal full-page requests and enhanced
  requests marked by `X-Alpine-Request` and `X-Alpine-Target`.
- Tailwind CSS for utility-first styling, configured in `tailwind.config.js` and
  compiled to `web/static/app.css` with `just build-css`.
- SQLite for persistent state.
- `sqlc` for all application SQL. Keep SQL queries in `.sql` files and use
  generated, typed Go code; do not hand-write database access in handlers.
- Caddy as the reverse proxy, configured through its API.

Do not introduce React, a Node backend, an ORM, or a second database unless the
requirements explicitly change.

## System model

1. An admin creates intern profiles.
2. An intern creates a project for themself, or an admin creates one and
   assigns one or more interns. The project has a main port and optional
   internal ports.
3. PortD creates the project directory with `start.sh` and/or `start.bat` plus
   a `README.md`.
4. PortD persists project, assignment, port, route, lifecycle, and audit data
   in SQLite.
5. PortD configures Caddy so `base_url/<project-name>` proxies to the project’s
   main localhost port.
6. On startup, PortD loads projects marked `should_run` but not live, then uses
   a bounded worker pool to run the OS-appropriate start script. It records the
   resulting lifecycle state and activity.
7. A port-discovery service reads host listening ports, maps known ports to
   projects, and caches the latest observation for display and diagnostics.

## Engineering rules

- Keep domain logic independent of HTTP and templates. Handlers coordinate;
  services own workflows; repositories use generated `sqlc` queries.
- Make external side effects explicit and recoverable: validate before creating
  directories, start processes only after database state is committed as ready,
  and record both success and failure in the activity log.
- Never interpolate untrusted project values into shell commands, filesystem
  paths, Caddy configuration, or SQL. Validate project slugs and use argument
  arrays with `os/exec`.
- Treat startup scripts as trusted admin-managed project artifacts. Capture
  command output and exit errors for the activity log; do not expose sensitive
  output to ordinary users.
- Allocate ports transactionally and enforce uniqueness in SQLite. A main port
  must be one of the project’s assigned ports.
- Make Caddy updates idempotent. If a database change and Caddy update cannot
  complete as one operation, persist a retryable route-sync state and show it
  to admins.
- Do not run arbitrary discovered directories automatically. Discovery is
  observational; only registered projects with `should_run = true` may start.
- Use UTC timestamps, explicit lifecycle enums, structured errors, context
  cancellation, and bounded concurrency.
- Authorize every request. Admins manage all records and infrastructure actions;
  interns may view and manage only projects to which they are assigned.

## UI rules

- Render complete initial pages on the server. Alpine enhances filters, dialogs,
  confirmations, and refresh controls only.
- Return HTML fragments for AJAX actions when updating part of a page. Preserve
  normal form navigation as a functional fallback.
- Use semantic HTML, labels, keyboard-accessible controls, visible focus, and
  concise status text. Use monospace for ports, filesystem paths, commands,
  and URLs.
- Use Tailwind CSS utility classes for styling. Do not write ad-hoc raw CSS files
  or embed raw styles in HTML <style> tags.
- The key screens are dashboard, project list, project detail, intern CRUD,
  project CRUD, port table, and activity log.
- The project-detail dashboard is also the project bootstrap guide. It must show
  the generated directory, public URL, main and internal ports, startup script,
  host/OS guidance, required application bind address (`localhost`), how to run
  and stop the project, and the generated README. These instructions must match
  the actual scaffold rather than being static, generic copy.

## Verification expectations

- Format Go and templ-generated Go, run unit tests, and run `go vet` before
  hand-off.
- Build and verify Tailwind CSS with `just build-css`.
- Test database constraints and service workflows with a temporary SQLite DB.
- Test Caddy and process execution through interfaces/fakes; do not require a
  live proxy or launch arbitrary project scripts in unit tests.
- Keep migrations, sqlc queries, generated code, and schema changes aligned.
