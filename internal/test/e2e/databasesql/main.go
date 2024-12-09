// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	dbName = "test.db"

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

// Server is Http server that exposes multiple endpoints.
type Server struct {
	db *sql.DB
}

// Create the db file.
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

// NewServer creates a server struct after initialing rand.
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

	_, err = database.Exec(tableInsertion)
	if err != nil {
		panic(err)
	}

	return &Server{
		db: database,
	}
}

func (s *Server) query(w http.ResponseWriter, req *http.Request, query string) {
	ctx := req.Context()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(req.Context(), query)
	if err != nil {
		slog.Error("query failed", "query", query, "error", err)
		return
	}

	slog.Info("queryDB called", "query", query)
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
		fmt.Fprintf(w, "ID: %d, firstName: %s, lastName: %s, email: %s, phone: %s\n", id, firstName, lastName, email, phone)
	}
}

func (s *Server) selectDb(w http.ResponseWriter, req *http.Request) {
	s.query(w, req, "SELECT * FROM contacts")
}

func (s *Server) insert(w http.ResponseWriter, req *http.Request) {
	s.query(w, req, "INSERT INTO contacts (first_name) VALUES ('Mike')")
}

func (s *Server) update(w http.ResponseWriter, req *http.Request) {
	s.query(w, req, "UPDATE contacts SET last_name = 'Santa' WHERE first_name = 'Mike'")
}

func (s *Server) delete(w http.ResponseWriter, req *http.Request) {
	s.query(w, req, "DELETE FROM contacts WHERE first_name = 'Mike'")
}

func (s *Server) drop(w http.ResponseWriter, req *http.Request) {
	s.query(w, req, "DROP TABLE contacts")
}

func (s *Server) invalid(w http.ResponseWriter, req *http.Request) {
	s.query(w, req, "syntax error")
}

func main() {
	port := fmt.Sprintf(":%d", 8080)
	slog.Info("starting http server", "port", port)

	s := NewServer()

	http.HandleFunc("/query_db", s.selectDb)
	http.HandleFunc("/insert", s.insert)
	http.HandleFunc("/update", s.update)
	http.HandleFunc("/delete", s.delete)
	http.HandleFunc("/drop", s.drop)
	http.HandleFunc("/invalid", s.invalid)
	go func() {
		_ = http.ListenAndServe(":8080", nil)
	}()

	tests := []struct {
		url string
	}{
		{url: "http://localhost:8080/query_db"},
		{url: "http://localhost:8080/insert"},
		{url: "http://localhost:8080/update"},
		{url: "http://localhost:8080/delete"},
		{url: "http://localhost:8080/drop"},
		{url: "http://localhost:8080/invalid"},
	}

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	for _, t := range tests {
		resp, err := http.Get(t.url)
		if err != nil {
			slog.Error("failed GET", "error", err)
		}
		if resp != nil {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				slog.Error("failed to read HTTP body", "error", err)
			}
			slog.Info("request successful", "body", string(body[:]))
			_ = resp.Body.Close()
		}
	}

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
