# PortD

Project hub for Algérie Télécom internships. Authenticated admins and interns manage projects, ports, and runtime state from a single registry with stable proxied URLs instead of raw host-and-port combinations.

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![templ](https://img.shields.io/badge/templ-server%20rendered-00ADD8?logo=templ)](https://templ.guide)
[![SQLite](https://img.shields.io/badge/SQLite-persistent-003B57?logo=sqlite)](https://sqlite.org)
[![Caddy](https://img.shields.io/badge/Caddy-reverse%20proxy-714BCE?logo=caddy)](https://caddyserver.com)

## What it does

- **Project registry** — create, assign, and track projects with slugs, ports, and lifecycle state.
- **Two access modes** — Direct mode (`server:<main-port>`) needs no proxy or app changes; Proxied mode (`base_url/<project-slug>`) is available for apps configured for a path base.
- **Runtime controls** — project owners and admins can start or stop a project from Project Detail; PortD selects `start.sh` on Unix and `start.bat` on Windows, and flags missing files before launch.
- **Port discovery** — maps listening host ports to registered projects and caches observations.
- **Activity log** — creations, lifecycle changes, runtime actions, ports, healthchecks, intern and session events are recorded asynchronously; admins review them at `/activity`.
- **Role-based access** — admins manage everything; interns view and manage only their assigned projects.

## Stack

- **Go** — HTTP server, business logic, filesystem work, process execution, deployment binary.
- **templ** — typed, server-rendered HTML components.
- **Alpine.js + AJAX plugin** — client-side state and progressive fragment updates (no separate JSON SPA).
- **SQLite** — persistent state via `sqlc`-generated typed Go code.
- **Tailwind CSS** — utility-first styling configured via `tailwind.config.js` and compiled to `web/static/app.css`.
- **Caddy** — reverse proxy configured through its API.

## Getting started

```sh
cp .env.example .env
just migrate-up
go run ./cmd/portd
```

Open `http://127.0.0.1:8080`. The default HTTP address is configurable via `PORTD_HTTP_ADDR`.

## Project layout

```
cmd/portd/        — entry point
internal/         — app, config, db, http, interns, ports, projects, proxy, runtime
views/            — templ components, layouts, pages
web/static/       — static assets
prototype/        — frontend-only proof-of-work prototype (not part of the build)
agent_docs/       — system overview diagram and PRD
```

## Architecture

The source architecture is documented in [agent_docs/System_overview.svg](agent_docs/System_overview.svg). Treat it as the behavioural reference when implementing or changing the product.

## Development

- Generate typed database code: `just generate`
- Create empty migration files: `just create-migration add_projects`
- Apply pending SQLite migrations: `just migrate-up`
- Format: `gofmt -w .`
- Vet: `go vet ./...`
- Test: `just test`
- Build CSS: `just build-css` (or `just watch-css` for development)
- SQL lives in `.sql` files; use `sqlc` to generate typed Go. Do not hand-write database access in handlers.
- Test external side effects (Caddy, process execution) through interfaces/fakes.

`PORTD_DB_PATH` controls the local SQLite database path and defaults to
`tmp/portd.db`. Migrations are applied explicitly with `just migrate-up`; the
server does not modify the database schema during startup. The `Justfile`
expects `just` and `sqlc` to be installed, and runs the migration CLI through
`go run`.

## Documentation

- [Product requirements](agent_docs/prd.md) — detailed flows, data model, and acceptance criteria.
- [Database schema](agent_docs/database_schema.md) — current tables, relationships, constraints, and migration workflow for onboarding.
- [Agent guide](agent.md) — engineering rules, system model, and verification expectations.
- [Prototype](prototype/README.md) — frontend-only reference screens.
