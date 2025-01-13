// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"
)

const defaultQuota = 5

var listenAddr = flag.String("addr", ":8082", "server listen address")

func main() {
	flag.Parse()

	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	db, err := openDB()
	if err != nil {
		log.Fatal("User database error: ", err)
	}
	if err = initDB(ctx, db); err != nil {
		log.Print("User database initialization error: ", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Print("User database close error: ", err)
		}
	}()

	rErr := refreshQuotas(ctx, db, 3*time.Second, 3, defaultQuota*2)

	log.Printf("Starting User server at %s ...", *listenAddr)
	srv := newServer(ctx, *listenAddr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	log.Println("User server started.")

	select {
	case err = <-errCh:
		stop()
	case err = <-rErr:
		stop()
	case <-ctx.Done():
		err = srv.Shutdown(context.Background())
	}
	if err != nil {
		log.Print("User server error:", err)
	}
	log.Print("User server stopped.")
}

func refreshQuotas(ctx context.Context, db *sql.DB, d time.Duration, incr, ceil int) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		ticker := time.NewTicker(d)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = addQuota(ctx, db, incr, ceil)
			}
		}
	}()
	return errCh
}
