// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

const (
	sqlQuery = "SELECT * FROM contacts"
	dbName   = "test.db"

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

func (s *Server) queryDb(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(req.Context(), sqlQuery)
	if err != nil {
		panic(err)
	}

	logger.Info("queryDb called")
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

func (s *Server) insert(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(req.Context(), "INSERT INTO contacts (first_name) VALUES ('Mike')")
	if err != nil {
		panic(err)
	}

	logger.Info("insert called")
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

func (s *Server) update(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(req.Context(), "UPDATE contacts SET last_name = 'Santa' WHERE first_name = 'Mike'")
	if err != nil {
		panic(err)
	}

	logger.Info("update called")
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

func (s *Server) delete(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(req.Context(), "DELETE FROM contacts WHERE first_name = 'Mike'")
	if err != nil {
		panic(err)
	}

	logger.Info("delete called")
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

func (s *Server) drop(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		panic(err)
	}

	rows, err := conn.QueryContext(req.Context(), "DROP TABLE contacts")
	if err != nil {
		panic(err)
	}

	logger.Info("drop called")
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

var logger *zap.Logger

func main() {
	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		fmt.Printf("error creating zap logger, error:%v", err)
		return
	}
	port := fmt.Sprintf(":%d", 8080)
	logger.Info("starting http server", zap.String("port", port))

	s := NewServer()

	http.HandleFunc("/query_db", s.queryDb)
	http.HandleFunc("/insert", s.insert)
	http.HandleFunc("/update", s.update)
	http.HandleFunc("/delete", s.delete)
	http.HandleFunc("/drop", s.drop)
	go func() {
		_ = http.ListenAndServe(":8080", nil)
	}()

	// give time for auto-instrumentation to start up
	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/query_db")
	if err != nil {
		logger.Error("Error performing GET", zap.Error(err))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading http body", zap.Error(err))
	}

	logger.Info("Body:\n", zap.String("body", string(body[:])))
	_ = resp.Body.Close()

	resp, err = http.Get("http://localhost:8080/insert")
	if err != nil {
		logger.Error("Error performing GET", zap.Error(err))
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading http body", zap.Error(err))
	}

	logger.Info("Body:\n", zap.String("body", string(body[:])))
	_ = resp.Body.Close()

	resp, err = http.Get("http://localhost:8080/update")
	if err != nil {
		logger.Error("Error performing GET", zap.Error(err))
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading http body", zap.Error(err))
	}

	logger.Info("Body:\n", zap.String("body", string(body[:])))
	_ = resp.Body.Close()

	resp, err = http.Get("http://localhost:8080/delete")
	if err != nil {
		logger.Error("Error performing GET", zap.Error(err))
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading http body", zap.Error(err))
	}

	logger.Info("Body:\n", zap.String("body", string(body[:])))
	_ = resp.Body.Close()

	resp, err = http.Get("http://localhost:8080/drop")
	if err != nil {
		logger.Error("Error performing GET", zap.Error(err))
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading http body", zap.Error(err))
	}

	logger.Info("Body:\n", zap.String("body", string(body[:])))
	_ = resp.Body.Close()

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
