package db

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func Connect(host string, port int, name, user, password string) (*sqlx.DB, error) {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + strconv.Itoa(port),
		Path:   name,
	}
	dsn := u.String() + "?sslmode=disable"
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return db, nil
}

func Begin(db *sqlx.DB) (*sqlx.Tx, error) {
	return db.Beginx()
}

func WithTx(db *sqlx.DB, fn func(*sqlx.Tx) error) error {
	tx, err := Begin(db)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
