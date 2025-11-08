package repository

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"

	"internal-transfers/internal/domain"
	"internal-transfers/internal/errors"
)

type accountRepository struct {
	db     SQLExecutor
	logger *slog.Logger
}

func NewAccountRepository(db SQLExecutor, logger *slog.Logger) domain.AccountRepository {
	return &accountRepository{
		db:     db,
		logger: logger,
	}
}

func (r *accountRepository) CreateAccount(account *domain.Account) error {
	query := `
		INSERT INTO accounts (id, balance, created_at, updated_at) 
		VALUES ($1, $2, $3, $4)
	`

	now := time.Now()
	_, err := r.db.Exec(
		query,
		account.ID,
		account.Balance.String(),
		now,
		now,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" { // unique_violation
				r.logger.Warn("Duplicate account creation attempt", "account_id", account.ID)
				return errors.ErrDuplicateAccount
			}
		}
		r.logger.Error("Failed to create account", "account_id", account.ID, "error", err)
		return errors.NewAppError(errors.InternalError, "failed to create account").WithDetails(err.Error())
	}

	r.logger.Info("Account created successfully", "account_id", account.ID)
	return nil
}

func (r *accountRepository) GetAccount(id int64) (*domain.Account, error) {
	query := `
		SELECT id, balance, created_at, updated_at 
		FROM accounts WHERE id = $1
	`

	return r.scanAccount(query, id)
}

func (r *accountRepository) GetAccountForUpdate(id int64) (*domain.Account, error) {
	query := `
		SELECT id, balance, created_at, updated_at 
		FROM accounts WHERE id = $1 FOR UPDATE
	`

	return r.scanAccount(query, id)
}

func (r *accountRepository) scanAccount(query string, id int64) (*domain.Account, error) {
	var account domain.Account
	var balanceStr string

	err := r.db.QueryRow(query, id).Scan(
		&account.ID,
		&balanceStr,
		&account.CreatedAt,
		&account.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn("Account not found", "account_id", id)
			return nil, errors.ErrAccountNotFound
		}
		r.logger.Error("Failed to get account", "account_id", id, "error", err)
		return nil, errors.NewAppError(errors.InternalError, "failed to get account").WithDetails(err.Error())
	}

	balance, err := decimal.NewFromString(balanceStr)
	if err != nil {
		r.logger.Error("Failed to parse balance", "account_id", id, "balance_str", balanceStr, "error", err)
		return nil, errors.NewAppError(errors.InternalError, "failed to parse balance").WithDetails(err.Error())
	}

	account.Balance = balance
	return &account, nil
}

func (r *accountRepository) UpdateAccountBalance(id int64, newBalance decimal.Decimal) error {
	query := `
		UPDATE accounts 
		SET balance = $1, updated_at = $2 
		WHERE id = $3
	`

	result, err := r.db.Exec(query, newBalance.String(), time.Now(), id)
	if err != nil {
		r.logger.Error("Failed to update account balance", "account_id", id, "error", err)
		return errors.NewAppError(errors.InternalError, "failed to update account balance").WithDetails(err.Error())
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.NewAppError(errors.InternalError, "failed to get rows affected").WithDetails(err.Error())
	}

	if rowsAffected == 0 {
		r.logger.Warn("No account found to update", "account_id", id)
		return errors.ErrAccountNotFound
	}

	r.logger.Info("Account balance updated", "account_id", id, "new_balance", newBalance)
	return nil
}
