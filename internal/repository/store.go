package repository

import (
	"database/sql"
	"log/slog"

	"internal-transfers/internal/domain"
	"internal-transfers/internal/errors"
)

// Store provides a unified interface for all repository operations with transaction support
type Store struct {
	executor SQLExecutor
	logger   *slog.Logger
}

// NewStore creates a new Store instance
func NewStore(db *sql.DB, logger *slog.Logger) *Store {
	return &Store{
		executor: db,
		logger:   logger,
	}
}

// Account returns an AccountRepository using the current executor
func (s *Store) Account() domain.AccountRepository {
	return NewAccountRepository(s.executor, s.logger)
}

// Transaction returns a TransactionRepository using the current executor
func (s *Store) Transaction() domain.TransactionRepository {
	return NewTransactionRepository(s.executor, s.logger)
}

// WithTransaction executes a function within a database transaction
func (s *Store) WithTransaction(fn func(*Store) error) error {
	// Only sql.DB can begin transactions
	db, ok := s.executor.(*sql.DB)
	if !ok {
		return errors.ErrCannotBeginTransaction
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	txStore := &Store{
		executor: &TxWrapper{Tx: tx},
		logger:   s.logger,
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(txStore); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
