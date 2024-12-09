// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"
)

// Server is Http server that exposes multiple endpoints.
type Server struct {
	rand *rand.Rand
}

// NewServer creates a server struct after initialing rand.
func NewServer() *Server {
	rd := rand.New(rand.NewSource(time.Now().Unix()))
	return &Server{
		rand: rd,
	}
}

func (s *Server) rolldice(w http.ResponseWriter, _ *http.Request) {
	n := s.rand.Intn(6) + 1
	slog.Info("rolldice called", "dice", n)
	fmt.Fprintf(w, "%v", n)
}

func setupHandler(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/rolldice", s.rolldice)
	return mux
}

func main() {
	port := fmt.Sprintf(":%d", 8080)
	slog.Info("starting http server", "port", port)

	s := NewServer()
	mux := setupHandler(s)
	if err := http.ListenAndServe(port, mux); err != nil {
		slog.Error("error running server", "error", err)
	}
}
