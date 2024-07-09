// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"go.uber.org/zap"
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
	logger.Info("rolldice called", zap.Int("dice", n))
	fmt.Fprintf(w, "%v", n)
}

var logger *zap.Logger

func setupHandler(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/rolldice", s.rolldice)
	return mux
}

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
	mux := setupHandler(s)
	if err := http.ListenAndServe(port, mux); err != nil {
		logger.Error("error running server", zap.Error(err))
	}
}
