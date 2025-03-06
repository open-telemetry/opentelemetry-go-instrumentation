// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"

	_ "github.com/mattn/go-sqlite3"

	"go.opentelemetry.io/auto/examples/rolldice/user/internal"
)

const dsn = "file:user.db?cache=shared&mode=rwc&_busy_timeout=9999999"

const (
	createTable = `CREATE TABLE users (
		id    INTEGER PRIMARY KEY,
		name  TEXT NOT NULL,
		quota INTEGER
	);`

	queryAll  = `SELECT id,name,quota FROM users`
	queryUser = queryAll + ` WHERE name = ?`

	updateQuota = `UPDATE users SET quota = ? WHERE id = ?`
	insertUser  = `INSERT INTO users (name, quota) VALUES (?, ?)`
	decrement   = `UPDATE users SET quota = quota - 1 WHERE name = ? AND quota > 0 RETURNING id, quota`
)

var users = []string{
	"Alice", "Bob", "Carol", "Dan", "Erin", "Faythe", "Grace", "Heidi", "Ivan",
	"Judy", "Mallory", "Niaj", "Olivia", "Peggy", "Rupert", "Sybil", "Trent",
	"Victor", "Walter",
}

func openDB() (*sql.DB, error) {
	return sql.Open("sqlite3", dsn)
}

func initDB(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, createTable)
	if err != nil {
		return err
	}

	for _, u := range users {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, e := getUser(ctx, db, u)
		if errors.Is(e, errUser) {
			err = errors.Join(err, addUser(ctx, db, u, defaultQuota))
		} else if e != nil {
			err = errors.Join(err, e)
		}
	}
	return err
}

func addQuota(ctx context.Context, db *sql.DB, incr, ceil int) (err error) {
	opts := &sql.TxOptions{Isolation: sql.LevelDefault, ReadOnly: false}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errors.Join(err, tx.Rollback())
		}
	}()

	var rows *sql.Rows
	rows, err = tx.QueryContext(ctx, queryAll)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var u internal.User
		err = rows.Scan(&u.ID, &u.Name, &u.Quota)
		if err != nil {
			return err
		}

		if u.Quota < ceil {
			newQuota := max(ceil, u.Quota+incr)
			_, err = tx.ExecContext(ctx, updateQuota, newQuota, u.ID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

var (
	errInsufficient = errors.New("insufficient quota")
	errUser         = errors.New("unknown user")
)

func useQuota(ctx context.Context, db *sql.DB, name string) (internal.User, error) {
	u := internal.User{Name: name}
	err := db.QueryRowContext(ctx, decrement, name).Scan(&u.ID, &u.Quota)
	if errors.Is(err, sql.ErrNoRows) {
		err = errUser
	}
	if err == nil {
		if u.Quota == 0 {
			err = errInsufficient
		} else if rand.Intn(4) < 1 { // nolint: gosec  // Weak random number generator is fine.
			// Simulate the "database is locked" issue
			// (https://github.com/mattn/go-sqlite3/issues/274). Actually
			// triggering this issue causes run away CPU usage by the user
			// service.
			err = errors.New("database table is locked: users")
		}
	}
	return u, err
}

func addUser(ctx context.Context, db *sql.DB, name string, quota int) error {
	_, err := db.ExecContext(ctx, insertUser, name, quota)
	return err
}

func getUser(ctx context.Context, db *sql.DB, name string) (internal.User, error) {
	u := internal.User{Name: name}
	err := db.QueryRowContext(ctx, queryUser, name).Scan(&u.ID, &u.Quota)
	if errors.Is(err, sql.ErrNoRows) {
		err = errUser
	}
	return u, err
}
