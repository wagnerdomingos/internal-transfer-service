package repository

import (
	"database/sql"
)

// SQLExecutor represents both sql.DB and sql.Tx
type SQLExecutor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// DB represents a database that can begin transactions
type DB interface {
	SQLExecutor
	Begin() (*sql.Tx, error)
}

// Ensure sql.DB implements DB interface
var _ DB = (*sql.DB)(nil)

// TxWrapper wraps sql.Tx to implement SQLExecutor
type TxWrapper struct {
	*sql.Tx
}

func (t *TxWrapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.Tx.Exec(query, args...)
}

func (t *TxWrapper) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.Tx.Query(query, args...)
}

func (t *TxWrapper) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.Tx.QueryRow(query, args...)
}
