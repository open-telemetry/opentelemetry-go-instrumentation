// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	_ "github.com/mattn/go-sqlite3"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const backend = "backend"

const dsn = "file:user.db?cache=shared&mode=rwc&_busy_timeout=9999999"

const (
	createTable = `CREATE TABLE users (
		id    INTEGER PRIMARY KEY,
		name  TEXT NOT NULL,
		score INTEGER
	);`

	queryAll  = `SELECT id,name,score FROM users`
	queryUser = queryAll + ` WHERE name = ?`

	updateScore = `UPDATE users SET score = ? WHERE id = ?`
	insertUser  = `INSERT INTO users (name, score) VALUES (?, ?)`
)

type user struct {
	ID    int
	Name  string
	Score int
}

func openDB() (*sql.DB, error) {
	return sql.Open("sqlite3", dsn)
}

func initDB(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, createTable)
	return err
}

func addUser(ctx context.Context, db *sql.DB, u user) error {
	_, span := otel.Tracer(backend).Start(
		ctx,
		"addUser",
		trace.WithAttributes(attribute.String("user", u.Name)),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	_, err := db.ExecContext(ctx, insertUser, u.Name, u.Score)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

var errUser = errors.New("unknown user")

func getUser(ctx context.Context, db *sql.DB, name string) (user, error) {
	_, span := otel.Tracer(backend).Start(
		ctx,
		"getUser",
		trace.WithAttributes(attribute.String("user", name)),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	u := user{Name: name}
	err := db.QueryRowContext(ctx, queryUser, name).Scan(&u.ID, &u.Name, &u.Score)
	if errors.Is(err, sql.ErrNoRows) {
		err = errUser
	} else if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return u, err
}

func setUserScore(ctx context.Context, db *sql.DB, name string, score int) (err error) {
	ctx, span := otel.Tracer(backend).Start(
		ctx, "setUserScore",
		trace.WithAttributes(
			attribute.String("user", name),
			attribute.Int("score", score),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	var u user
	u, err = getUser(ctx, db, name)
	if err != nil {
		if errors.Is(err, errUser) {
			u.Score = score
			return addUser(ctx, db, u)
		} else {
			return err
		}
	}

	_, err = db.ExecContext(ctx, updateScore, score, u.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func newServer(ctx context.Context, addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/{user}/score/{score}", handleUserScore)

	return &http.Server{
		Addr: addr,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		Handler: mux,
	}
}

func handleUserScore(w http.ResponseWriter, req *http.Request) {
	user := req.PathValue("user")

	scoreStr := req.PathValue("score")
	score, err := strconv.Atoi(scoreStr)
	if err != nil {
		http.Error(w, "Invalid score", http.StatusBadRequest)
		return
	}

	db, err := openDB()
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = db.Close() }()

	ctx := req.Context()
	err = setUserScore(ctx, db, user, score)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%d", score)
}
