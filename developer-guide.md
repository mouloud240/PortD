Author: Mouloud Hasrane 
Last updated : 2026-09-13


-----------

Hello developer , and welcome to the codebase , wether you wanted to debug , fix a bug , or add a new feature , this guide will help you to get started with the codebase and understand how it works.

To help you understand better the codebase, this guide will contain zero ai generated content, and will be written by a human developer who has worked on the codebase (aka me).


So let's get started.


## Tech stack and overview:

This project (portd) is a project that allows interns to create and manage their internship projects , and employees to easilty access those projects without having to remember the ip and port of each project.
The project was built using :
Golang: this project uses server side rendering with golang , using only the std lib (net/http) with templ for rendering
(Check `internal/web/templates` for the templates used in the project)
Database: the project uses sqlite with sqlc to generate the database code and queries, and golang-migrate to manage migrations check `internal/db` for the database code and queries, and `migrations` for the migrations

The project relies on Caddy as a reverse proxy, We choose traefik because it supports dynamic configuration via a REST API, and it is easy to use and configure, and it also supports automatic https with Let's Encrypt.

## Getting started:

I recommend you to read the [README.md](README.md) file first, it contains all the information you need to get started with the project, and it also contains the project layout and architecture.

## Project layout:

```
.
├── cmd/portd/main.go        # binary entrypoint: loads config, builds internal/app, starts HTTP server
├── cmd/migrate/main.go      # runs golang-migrate up/down against internal/db/migrations
├── internal/app/app.go      # wiring: opens sqlite, builds services, router, health poller lifecycle
├── internal/config/         # env parsing (DB path, ports range, projects dir, admin creds, base URL)
├── internal/http/
│   ├── router.go            # all routes on net/http ServeMux
│   ├── handlers/            # one file per area: pages, auth, projects, projects_actions, ports, interns, activity, features
│   └── middleware/          # RequireSession, RequireAdmin, RequireProjectMember
├── internal/httperr/        # shared handler error type -> status code + error page
├── internal/auth/           # login/session logic, password hashing, context helpers
├── internal/interns/        # intern accounts CRUD (admin only)
├── internal/projects/       # project CRUD, dir scaffolding, slug/validation, detect unregistered dirs
├── internal/ports/          # port states (available/assigned/running/used), allocate.go, scanner.go (OS scan)
├── internal/runtime/        # process manager: runs start.sh/start.bat, tracks pid, killtree, scaffold.go
├── internal/proxy/          # reverse-proxy registration (direct host vs proxied subpath mode)
├── internal/healthcheck/    # poller.go: periodic HTTP checks per project, healthcheck.go status logic
├── internal/activity/       # activity log service (who did what, shown on /activity)
├── internal/db/
│   ├── queries/*.sql        # source of truth: projects, ports, interns, sessions, routes, healthchecks, activity_logs
│   ├── generated/*.go       # sqlc output, never edit by hand
│   └── migrations/*.sql     # golang-migrate versions (000001 initial schema, 000002 routes, ...)
├── views/                   # templ templates: layouts/, pages/, components/ (+ generated *_templ.go)
├── web/static/              # served at /static: app.css/hub.css, js (alpine.min.js, alpine-ajax), css/
├── web/input.css + tailwind.config.js  # tailwind source -> compiled css
├── sqlc.yaml / Justfile     # codegen (sqlc generate, templ generate) and common tasks (just ...)
└── README.md / PRODUCT.md / DESIGN.md / UI_ARCHITECTURE.md / docs/  # setup, product spec, design notes
```

A normal http request to the server will go through the following steps:
```
  Client-->Handler --> Router --> Hander --> Service- Database --> Templates (w.render() or w.redirect) ---> client 
                                                      
```
The project relies heavily on the `internal` package , and dependency injection is used for the services and handlers.



## Important features:

While most features are standard CRUD operations , there are some features that are important to understand well:
### Port manager (`/internal/ports`):

This package is responsible for managing the ports assigned to each project , as well sa ports used by the os, 

We define 2 types of ports:
- **Main port**: this is the port that is used to access the project from the browser, generally this port is the frontend port.
- **Internal ports**: these are the ports that are used by the project for internal communication, such as database, api, etc.

A port may be in one of the following states:
- **Available**: the port is available to be assigned to a project.
- **Assigned**: the port is assigned to a project, but the project is not running
- **Running**: the port is assigned to a project, and the project is running
- **Used**: the port is used by the os or another process and does is not assigned to a project, and cannot be assigned to a project.


### Process manager (`/internal/runtime`):

This package is responsible for managing states of the other projects, and for starting and stopping the projects, when a project is started via portd, the project manager will monitor this process and register it's pid and state in the database, and when the project is stopped, the process manager will unregister the process from the database.

The process manager looks for the `start.sh` or `start.bat` file in the project directory, and executes it to start the project, and when the project is stopped, it will kill the process and unregister it from the database.
 
> When using linux make sure to give the `start.sh` file executable permissions using `chmod +x start.sh` before starting the project, and that the bash interpreter is defined in the first line of the file using `#!/bin/bash` or `#!/usr/bin/env bash`

### Project directory manager (`/internal/projects`):

This package  is responsible for managing the projects files in the projects directory, including:

- Creating the project directory and files when a new project is created
- Deleting the project directory and files when a project is deleted
- Auto detecting old projects that  are not registered in the database in the projects dir



### Reverse proxy manager (`/internal/proxy`):

This package is responsible for managing the reverse proxy configuration , and for registering the projects in the reverse proxy, and for unregistering the projects from the reverse proxy when they are deleted or stopped.

each project runs in 2 modes:
- **Direct mode**: the project is running and the main port is registered in the reverse proxy, and the project is accessible via the internal url `http://<project-name>.<host>` (e.g. `http://my-project.localhost`)
- **Proxied mode**: the project is running and the main port is registered in the reverse proxy, and the project is accessible via the internal url `http://<host>/<project-slug>` (e.g. `http://localhost/my-project`)


We choose this approach because of the constraints we had including :
- No ability to change the dns so that removed any option of using a subdomain for each project.
- No abilty to edit host file for each machine since windows group reverts them  after a while.
- No access to the internet so we can't use a public dns service like nip.io or xip.io to resolve the subdomains to the local ip address.
- And if we use paths for each project immediaty , routnig inside each individual project will be broken since the project will be running in a subpath and not in the root path, and this will break the routing of the project.



## What to do next:
The project still has a lot of potential for features to be added that I unfortunately don't have the time to implement, but if you want to contribute to the project, here are some ideas for features that can be added:

- Add employee accounts and permissions to allow them to browse and access the projects.
- Add watching ports to the port manager to automatically detect when a port is used by the os or another process and mark it as used.
- Add watching processes to the process manager to automatically detect when a project is stopped or killed and unregister it from the database.
- A centralized port manager for all projects so you can flip projects ports without manually updating the code. (this might become easier if you implement the next feature)
- Adopt containerization for the projects (docker, podman, etc.) to allow for better isolation and management of the projects, and to allow for easier deployment of the projects, ports management (with port mapping), and process management (with container lifecycle management).




## Conclusion:
Thiis project is a great starting point and a better learning project for a developer who wants to learn about golang , on what I think is the kind of projects where golang shines, and I hope this guide will help you understand the project better and contribute to it.

For any questions or help , don't hesitate to reach out to me on my email:
[mouloudhasrane@gmail.com](mailto:mouloudhasrane@gmail.com)





