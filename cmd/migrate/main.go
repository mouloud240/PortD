package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"
)

const defaultDatabasePath = "tmp/portd.db"

func main() {
	if len(os.Args) != 2 || os.Args[1] != "up" {
		fmt.Fprintln(os.Stderr, "usage: migrate up")
		os.Exit(2)
	}

	databasePath := os.Getenv("PORTD_DB_PATH")
	if databasePath == "" {
		databasePath = defaultDatabasePath
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create database directory: %v\n", err)
		os.Exit(1)
	}

	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	driver, err := sqlite.WithInstance(database, &sqlite.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure SQLite migration driver: %v\n", err)
		os.Exit(1)
	}

	migration, err := migrate.NewWithDatabaseInstance("file://internal/db/migrations", "portd", driver)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure migrations: %v\n", err)
		os.Exit(1)
	}
	defer migration.Close()

	err = migration.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintf(os.Stderr, "apply migrations: %v\n", err)
		os.Exit(1)
	}
}
