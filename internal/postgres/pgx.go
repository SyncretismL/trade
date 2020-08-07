package postgres

import (
	"authDB/pkg/logger"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq" // driver for postgre
	"github.com/pkg/errors"
)

//DB ...
type DB struct {
	Session *sql.DB
	Logger  logger.Logger
}

const (
	dbHost = "localhost"
	dbPort = "5432"
	dbUser = "syncretism"
	dbPass = "admin"
	dbName = "syncretism"
)

//New ...
func New(logger logger.Logger) *DB {
	dbinfo := fmt.Sprintf("user=%s password=%s host=%s port=%s database=%s sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := sql.Open("postgres", dbinfo)
	if err != nil {
		logger.Fatalf("failed open conn to db %s", err)

		return nil
	}

	if err := db.Ping(); err != nil {
		logger.Fatalf("failed ping to db %s", err)

		return nil
	}

	return &DB{
		Session: db,
		Logger:  logger,
	}
}

// Close ...
func (d *DB) Close() error {
	if err := d.Session.Close(); err != nil {
		return errors.Wrap(err, "can't close db")
	}

	return nil
}

type sqlScanner interface {
	Scan(dest ...interface{}) error
}
