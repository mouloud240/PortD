# PortD

Project hub for Algérie Télécom internships. Authenticated admins and interns manage projects, ports, and runtime state from a single registry with stable proxied URLs instead of raw host-and-port combinations.

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![templ](https://img.shields.io/badge/templ-server%20rendered-00ADD8?logo=templ)](https://templ.guide)
[![SQLite](https://img.shields.io/badge/SQLite-persistent-003B57?logo=sqlite)](https://sqlite.org)
[![Caddy](https://img.shields.io/badge/Caddy-reverse%20proxy-714BCE?logo=caddy)](https://caddyserver.com)

## What it does

- **Project registry** — create, assign, and track projects with slugs, ports, and lifecycle state.
- **Stable URLs** — `base_url/<project-slug>` proxies to the project's main port via Caddy.
- **Startup recovery** — on launch, PortD runs `start.sh`/`start.bat` for projects marked `should_run` via a bounded worker pool.
- **Port discovery** — maps listening host ports to registered projects and caches observations.
- **Activity log** — every creation, lifecycle change, startup attempt, and route update is recorded.
- **Role-based access** — admins manage everything; interns view and manage only their assigned projects.

## Stack

- **Go** — HTTP server, business logic, filesystem work, process execution, deployment binary.
- **templ** — typed, server-rendered HTML components.
- **Alpine.js + AJAX plugin** — client-side state and progressive fragment updates (no separate JSON SPA).
- **SQLite** — persistent state via `sqlc`-generated typed Go code.
- **Caddy** — reverse proxy configured through its API.

## Getting started

```sh
cp .env.example .env
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

- Format: `gofmt -w .`
- Vet: `go vet ./...`
- Test: `go test ./...`
- SQL lives in `.sql` files; use `sqlc` to generate typed Go. Do not hand-write database access in handlers.
- Test external side effects (Caddy, process execution) through interfaces/fakes.

## Documentation

- [Product requirements](agent_docs/prd.md) — detailed flows, data model, and acceptance criteria.
- [Agent guide](agent.md) — engineering rules, system model, and verification expectations.
- [Prototype](prototype/README.md) — frontend-only reference screens.
