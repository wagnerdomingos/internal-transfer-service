package repository

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	"internal-transfers/internal/domain"
	"internal-transfers/internal/errors"
)

type transactionRepository struct {
	db     SQLExecutor
	logger *slog.Logger
}

func NewTransactionRepository(db SQLExecutor, logger *slog.Logger) domain.TransactionRepository {
	return &transactionRepository{
		db:     db,
		logger: logger,
	}
}

func (r *transactionRepository) CreateTransaction(tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions
		(id, source_account_id, destination_account_id, amount, idempotency_key, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	now := time.Now()

	// Handle optional idempotency key
	var idempotencyKey interface{}
	if tx.IdempotencyKey != nil {
		idempotencyKey = *tx.IdempotencyKey
	} else {
		idempotencyKey = nil
	}

	_, err := r.db.Exec(
		query,
		tx.ID,
		tx.SourceAccountID,
		tx.DestinationAccountID,
		tx.Amount.String(),
		idempotencyKey,
		tx.Status,
		now,
		now,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" { // unique_violation
				// Check if it's idempotency key violation
				if pqErr.Constraint == "idx_transactions_idempotency_key" {
					r.logger.Warn("Duplicate idempotency key", "idempotency_key", tx.IdempotencyKey)
					return errors.ErrDuplicateTransaction
				}
			}
		}
		r.logger.Error("Failed to create transaction",
			"source_account_id", tx.SourceAccountID,
			"destination_account_id", tx.DestinationAccountID,
			"amount", tx.Amount,
			"error", err)
		return errors.NewAppError(errors.InternalError, "failed to create transaction").WithDetails(err.Error())
	}

	tx.CreatedAt = now
	tx.UpdatedAt = now
	r.logger.Info("Transaction created successfully", "transaction_id", tx.ID)
	return nil
}

func (r *transactionRepository) GetTransactionByID(id uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT id, source_account_id, destination_account_id, amount, idempotency_key, status, created_at, updated_at
		FROM transactions WHERE id = $1
	`

	return r.scanTransaction(query, id)
}

func (r *transactionRepository) GetTransactionByIDempotencyKey(key uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT id, source_account_id, destination_account_id, amount, idempotency_key, status, created_at, updated_at
		FROM transactions WHERE idempotency_key = $1
	`

	return r.scanTransaction(query, key)
}

func (r *transactionRepository) scanTransaction(query string, arg interface{}) (*domain.Transaction, error) {
	var transaction domain.Transaction
	var amountStr string
	var idempotencyKey sql.NullString

	err := r.db.QueryRow(query, arg).Scan(
		&transaction.ID,
		&transaction.SourceAccountID,
		&transaction.DestinationAccountID,
		&amountStr,
		&idempotencyKey,
		&transaction.Status,
		&transaction.CreatedAt,
		&transaction.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get transaction", "arg", arg, "error", err)
		return nil, errors.NewAppError(errors.InternalError, "failed to get transaction").WithDetails(err.Error())
	}

	// Parse amount
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, errors.NewAppError(errors.InternalError, "failed to parse amount").WithDetails(err.Error())
	}
	transaction.Amount = amount

	// Parse optional idempotency key
	if idempotencyKey.Valid {
		key, err := uuid.Parse(idempotencyKey.String)
		if err != nil {
			return nil, errors.NewAppError(errors.InternalError, "failed to parse idempotency key").WithDetails(err.Error())
		}
		transaction.IdempotencyKey = &key
	}

	return &transaction, nil
}

func (r *transactionRepository) UpdateTransactionStatus(id uuid.UUID, status string) error {
	query := `UPDATE transactions SET status = $1, updated_at = $2 WHERE id = $3`

	_, err := r.db.Exec(query, status, time.Now(), id)
	if err != nil {
		r.logger.Error("Failed to update transaction status",
			"transaction_id", id, "status", status, "error", err)
		return errors.NewAppError(errors.InternalError, "failed to update transaction status").WithDetails(err.Error())
	}

	r.logger.Info("Transaction status updated", "transaction_id", id, "status", status)
	return nil
}
