// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

const sqlQuery = "SELECT * FROM contacts"

// Server is Http server that exposes multiple endpoints.
type Server struct {
	db *sql.DB
}

// NewServer creates a server struct after initialing rand.
func NewServer() *Server {
	database, err := sql.Open("sqlite3", "test.db")
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

	// give time for auto-instrumentation to report signal
	time.Sleep(5 * time.Second)
}
