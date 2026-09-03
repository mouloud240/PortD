# PRD — PortD

## 1. Purpose

PortD is an internal project-management and hosting hub for Algérie Télécom
internships. It replaces manually shared localhost ports with a searchable
project registry and stable proxied URLs for authenticated admins and interns.

The system overview defines the central flow: PortD creates and records a
project, provisions its local directory, configures Caddy, starts eligible
projects after PortD starts, and correlates listening ports with registered
projects.

## 2. Users and outcomes

| User | Outcome |
| --- | --- |
| Intern | Creates and manages their assigned projects, follows bootstrap instructions, and opens project URLs. |
| Administrator | Manages intern and project records, all project operations, ports, routes, and activity. |

## 3. Scope

### Core capabilities

- Intern profile CRUD by administrators.
- Project CRUD: name, URL-safe slug, description, lifecycle, main port, internal
  ports, assigned one-or-more interns, directory, startup entry point, public
  path, and delivery-tracking state/milestones.
- Project scaffolding: create a project directory named from the project slug,
  with OS startup script(s) and a README that documents ports, URL, and run
  expectations.
- Caddy route management through the Caddy API. A route maps
  `base_url/<project-slug>` to `localhost:<main-port>`.
- Startup recovery: on PortD launch, queue projects where `should_run` is true
  and `is_live` is false; a bounded worker pool runs the correct `start.sh` or
  `start.bat` for the host OS and records the result.
- Host port discovery: inspect bound/listening ports, associate each with its
  registered project when possible, and cache the observation.
- Dashboard, project directory, project detail/dashboard, project tracking,
  port table, and activity log.
- Activity records for project creation, updates, lifecycle changes, startup
  attempts, detected ports, and Caddy route changes.

### Out of scope for the first release

- Public internet exposure, multi-host scheduling, containers, or orchestration.
- Automatic remediation of port conflicts.
- Running unregistered directories or arbitrary user-entered commands.
- Full uptime monitoring, alerting, and historical observability.
- A separate SPA or public JSON API.

## 4. Key flows

### Create a project

1. An intern submits a project for themself, or an administrator submits one
   for assigned intern(s), with a valid name, slug, description, main port, and
   optional internal ports.
2. PortD verifies assignments and that every requested port is available and
   unassigned.
3. PortD creates the project directory and standard startup/README artifacts.
4. PortD persists the project, assignments, ports, and intended route in SQLite.
5. PortD sends an idempotent route update to Caddy.
6. PortD records all outcomes in the activity log and shows the project’s URL.

If provisioning or route configuration fails, PortD must preserve enough state
to retry safely and must present the failure to an administrator.

### Project dashboard and bootstrap

Each project has a dedicated dashboard. It is the project's operational page
and its source of setup instructions. In addition to project ownership,
delivery tracking, URL, server, reachability, directory, entry point, and port
allocation, it presents the exact bootstrap instructions generated for that
project:

1. The directory to use: `/projects/<project-slug>/`.
2. The assigned main and internal ports, and which port is exposed through
   `base_url/<project-slug>`.
3. The host-specific startup command (`start.sh` on Unix-like hosts or
   `start.bat` on Windows) and how to stop/restart it through PortD.
4. The required local bind target: `localhost:<main-port>`; projects must not
   self-configure Caddy or expose their internal ports.
5. The expected project structure and the generated README, with a way to view
   or download it.

The dashboard instructions are project-specific and come from the same
scaffolding data that creates the directory and README. They must remain
consistent after a port, route, or startup configuration changes.

### Startup and runtime reconciliation

1. At application startup, PortD selects projects where `should_run = true` and
   `is_live = false`.
2. A bounded worker pool invokes each project’s OS-appropriate start script.
3. PortD captures the result, updates lifecycle state, and adds an activity
   event.
4. Port discovery gathers bound ports from the host, maps them to projects, and
   caches the current result for the port table.

## 5. Functional requirements

- Project slugs are unique and safe for both URL paths and project-directory
  names.
- A project has at least one assigned intern and at least one port.
- Exactly one project port is the main port; it is the Caddy upstream port.
- No assigned port may belong to more than one active project.
- Interns can create projects for themselves, search their assigned projects,
  update their permitted project fields and delivery tracking, and open their
  project URLs. They cannot change proxy or host-level configuration.
- Administrators can create, edit, archive, and inspect any project; create,
  edit, and remove intern profiles; assign multiple interns; retry failed route
  syncs; run infrastructure actions; and review activity.
- A project dashboard displays the generated bootstrap instructions and README
  alongside project setup, deployment configuration, and delivery tracking.
- Archiving disables the Caddy route and prevents automatic startup. Its ports
  become available only after the archive operation completes safely.
- The port table shows port, detected process where available, registered
  project, intern(s), and observation time.

## 6. Data model

| Entity | Essential fields |
| --- | --- |
| `interns` | id, name, email/identifier, active flag, timestamps |
| `projects` | id, name, slug, description, directory, startup command, `should_run`, `is_live`, lifecycle status, route-sync status, timestamps |
| `project_interns` | project id, intern id |
| `project_ports` | project id, port number, role (`main` or `internal`) |
| `port_observations` | port number, process metadata, project id nullable, observed at |
| `activity_logs` | actor nullable, event type, entity type/id, outcome, safe detail, created at |

SQLite constraints enforce unique active port ownership and unique project
slugs. The main-port constraint is enforced in service logic and tests.

## 7. Technical requirements

- Go application using `templ` server-rendered views.
- Alpine.js and its AJAX plugin enhance pages with fragment updates while forms
  continue to work without JavaScript.
- SQLite is accessed solely through `sqlc`-generated query code and versioned
  migrations.
- Caddy is the reverse proxy and is updated with its API; use the configured
  base URL and path-based routing from the system overview.
- OS process and port-inspection behavior is abstracted behind interfaces so it
  can be tested without modifying a developer machine.
- Logs and activity records exclude credentials, tokens, and unsafe raw command
  output.

## 8. Acceptance criteria

- An administrator can create an intern and a multi-intern project with one main
  and optional internal ports.
- Project creation leaves a documented local directory, durable SQLite records,
  an activity event, and a Caddy route to the main port.
- A project marked `should_run` is queued on PortD startup and shows either a
  successful live state or an actionable failure.
- The port table identifies registered ports and clearly marks unknown ports.
- Admins can locate and open any project by searching its name or assigned
  intern; interns can locate and open their assigned projects.
- Failed Caddy operations and failed startup scripts are visible and retryable
  without duplicating directories, routes, or port assignments.
- An intern can use their project dashboard to see the exact directory, URL,
  port allocation, OS-appropriate startup command, bind requirement, and
  generated README needed to bootstrap the project.
