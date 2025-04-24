// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package databasesql is a testing application for the [database/sql] package.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/auto/internal/test/trigger"
)

const (
	tableDefinition = `CREATE TABLE contacts (
							contact_id INTEGER PRIMARY KEY,
							first_name TEXT NOT NULL,
							last_name TEXT NOT NULL,
							email TEXT NOT NULL UNIQUE,
							phone TEXT NOT NULL UNIQUE);`

	tableInsertion = `INSERT INTO 'contacts'
						('first_name', 'last_name', 'email', 'phone') VALUES
						('Moshe', 'Levi', 'moshe@gmail.com', '052-1234567');`
)

func main() {
	var trig trigger.Flag
	flag.Var(&trig, "trigger", trig.Docs())
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Wait for auto-instrumentation.
	err := trig.Wait(ctx)
	if err != nil {
		slog.Error("Error waiting for auto-instrumentation", "error", err)
		os.Exit(1)
	}

	err = run(ctx)
	if err != nil {
		slog.Error("Error running database workflow", "error", err)
	}
}

func run(ctx context.Context) error {
	const scopeName = "go.opentelemetry.io/auto/database/sql"
	ctx, span := otel.GetTracerProvider().Tracer(scopeName).Start(ctx, "run")
	defer span.End()

	tmpDir := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "test-*.db")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}

	dbName := tmpFile.Name()
	defer os.Remove(dbName)

	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	_, err = db.Exec(tableDefinition)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	_, err = db.Exec(tableInsertion)
	if err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}

	for _, p := range []string{
		"SELECT * FROM contacts",
		"INSERT INTO contacts (first_name) VALUES ('Mike')",
		"UPDATE contacts SET last_name = 'Santa' WHERE first_name = 'Mike'",
		"DELETE FROM contacts WHERE first_name = 'Mike'",
		"DROP TABLE contacts",
		"syntax error",
	} {
		rows, err := query(ctx, db, p)
		if err != nil {
			slog.Info("failed to query database", "error", err)
		} else {
			slog.Info("query result", "rows", rows)
		}
	}
	return nil
}

type Row struct {
	ID        int
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

func query(ctx context.Context, db *sql.DB, query string) ([]Row, error) {
	slog.Info("querying database", "query", query)

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	result, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var rows []Row
	for result.Next() {
		var row Row
		err := result.Scan(
			&row.ID,
			&row.FirstName,
			&row.LastName,
			&row.Email,
			&row.Phone,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}
