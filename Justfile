set dotenv-load := true

db := env_var_or_default("PORTD_DB_PATH", "tmp/portd.db")

generate:
    sqlc generate

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

test:
    go test ./...
