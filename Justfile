set dotenv-load := true

db := env_var_or_default("PORTD_DB_PATH", "tmp/portd.db")

generate:
    sqlc generate

# Render .templ sources to *_templ.go (version pinned by the tool directive in go.mod).
templ:
    go tool templ generate

templ-fmt:
    go tool templ fmt

migrate-up:
    mkdir -p "$(dirname "{{db}}")"
    PORTD_DB_PATH="{{db}}" go run ./cmd/migrate up

create-migration name:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! "{{name}}" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
        echo "migration name must contain only lowercase letters, numbers, '_' or '-'" >&2
        exit 1
    fi
    next="$(
        find internal/db/migrations -maxdepth 1 -type f -name '*.up.sql' -printf '%f\n' |
            sed -n 's/^\([0-9]\+\)_.*/\1/p' |
            sort -n |
            tail -n 1
    )"
    if [[ -z "$next" ]]; then
        next=0
    fi
    next=$((10#$next + 1))
    version="$(printf '%06d' "$next")"
    up="internal/db/migrations/${version}_{{name}}.up.sql"
    down="internal/db/migrations/${version}_{{name}}.down.sql"
    if [[ -e "$up" || -e "$down" ]]; then
        echo "migration files already exist for version ${version}" >&2
        exit 1
    fi
    : > "$up"
    : > "$down"
    echo "created $up"
    echo "created $down"

# Generate first: a stale *_templ.go compiles cleanly and renders old markup.
test: templ
    go test ./...

# Live reload for development: watches Go and templ sources, Uses temp Live reload feature
dev:
     go tool templ generate --watch --proxy="http://localhost:8080" --cmd="go run ./cmd/portd"

build-css:
    npx -y tailwindcss@3 -i web/input.css -o web/static/app.css --minify

watch-css:
    npx -y tailwindcss@3 -i web/input.css -o web/static/app.css --watch
