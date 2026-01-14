// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package httpPlusdb provides an example of how to OpenTelemetry
// auto-instrumentation for Go to instrument an application that runs an HTTP
// server and interacts with a database.
package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

const (
	sqlQuery = "SELECT * FROM contacts"
	dbName   = "test.db"

	tableDefinition = `CREATE TABLE contacts (
	contact_id INTEGER PRIMARY KEY,
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	email TEXT NOT NULL,
	phone TEXT NOT NULL);`

	tableInsertion = "INSERT INTO `contacts` " +
		"(`first_name`, `last_name`, `email`, `phone`) VALUES " +
		"('Moshe', 'Levi', 'moshe@gmail.com', '052-1234567');"
)

// Server is Http server that exposes multiple endpoints.
type Server struct {
	db *sql.DB
}

// CreateDb creates the db file.
func CreateDb() {
	file, err := os.Create(dbName)
	if err != nil {
		panic(err)
	}
	err = file.Close()
	if err != nil {
		panic(err)
	}
}

// NewServer creates a server struct after creating the DB and initializing it
// and creating a table named 'contacts' and adding a single row to it.
func NewServer() *Server {
	CreateDb()

	database, err := sql.Open("sqlite3", dbName)
	if err != nil {
		panic(err)
	}

	_, err = database.Exec(tableDefinition)
	if err != nil {
		panic(err)
	}

	return &Server{
		db: database,
	}
}

func (s *Server) queryDb(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		panic(err)
	}

	_, err = s.db.ExecContext(ctx, tableInsertion)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(req.Context(), sqlQuery)
	if err != nil {
		panic(err)
	}

	slog.Info("queryDb called")
	for rows.Next() {
		var id int
		var firstName string
		var lastName string
		var email string
		var phone string
		err := rows.Scan(&id, &firstName, &lastName, &email, &phone)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(
			w,
			"ID: %d, firstName: %s, lastName: %s, email: %s, phone: %s\n",
			id,
			firstName,
			lastName,
			email,
			phone,
		)
	}
}

func setupHandler(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/query_db", s.queryDb)
	return mux
}

func main() {
	port := fmt.Sprintf(":%d", 8080)
	slog.Info("starting http server", "port", port)

	s := NewServer()
	mux := setupHandler(s)
	if err := http.ListenAndServe( //nolint:gosec // Non-timeout HTTP server.
		port,
		mux,
	); err != nil {
		slog.Error("error running server", "error", err)
	}
}
