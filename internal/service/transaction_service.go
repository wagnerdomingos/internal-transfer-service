package service

import (
	"log/slog"
	"strconv"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"internal-transfers/internal/domain"
	"internal-transfers/internal/errors"
	"internal-transfers/internal/repository"
)

type TransactionService struct {
	store  *repository.Store
	logger *slog.Logger
}

func NewTransactionService(
	store *repository.Store,
	logger *slog.Logger,
) *TransactionService {
	return &TransactionService{
		store:  store,
		logger: logger,
	}
}

type TransferRequest struct {
	SourceAccountID      string
	DestinationAccountID string
	Amount               decimal.Decimal
	IdempotencyKey       uuid.UUID
}

func (s *TransactionService) Transfer(req *TransferRequest) (*domain.Transaction, error) {
	s.logger.Info("Processing transfer",
		"source_account_id", req.SourceAccountID,
		"destination_account_id", req.DestinationAccountID,
		"amount", req.Amount,
		"idempotency_key", req.IdempotencyKey)

	// Parse account IDs first
	sourceID, destID, err := s.parseAccountIDs(req.SourceAccountID, req.DestinationAccountID)
	if err != nil {
		return nil, err
	}

	// Validate transfer
	if err := s.validateTransfer(sourceID, destID, req.Amount); err != nil {
		return nil, err
	}

	var transaction *domain.Transaction

	// Process everything in a single database transaction
	err = s.store.WithTransaction(func(store *repository.Store) error {
		// Check for existing transaction with same idempotency key INSIDE transaction
		existingTx, err := store.Transaction().GetTransactionByIDempotencyKey(req.IdempotencyKey)
		if err != nil {
			return err
		}
		if existingTx != nil {
			s.logger.Info("Returning existing transaction for idempotency key",
				"idempotency_key", req.IdempotencyKey,
				"transaction_id", existingTx.ID)
			transaction = existingTx
			return nil
		}

		// Determine deterministic order by comparing account IDs to avoid deadlocks
		var firstID, secondID int64
		if sourceID < destID {
			firstID, secondID = sourceID, destID
		} else {
			firstID, secondID = destID, sourceID
		}

		// Lock first account
		firstAccount, err := store.Account().GetAccountForUpdate(firstID)
		if err != nil {
			return err
		}

		// Lock second account
		secondAccount, err := store.Account().GetAccountForUpdate(secondID)
		if err != nil {
			return err
		}

		// Map locked rows back to source and destination
		var sourceAccount, destAccount *domain.Account
		if firstID == sourceID {
			sourceAccount = firstAccount
			destAccount = secondAccount
		} else {
			sourceAccount = secondAccount
			destAccount = firstAccount
		}

		// Create transaction record as pending INSIDE transaction
		transaction = &domain.Transaction{
			ID:                   uuid.New(),
			SourceAccountID:      sourceID,
			DestinationAccountID: destID,
			Amount:               req.Amount,
			IdempotencyKey:       req.IdempotencyKey,
			Status:               "pending",
		}

		if err := store.Transaction().CreateTransaction(transaction); err != nil {
			return err
		}

		// Check sufficient balance
		if sourceAccount.Balance.LessThan(req.Amount) {
			transaction.Status = "failed"
			if updateErr := store.Transaction().UpdateTransactionStatus(transaction.ID, "failed"); updateErr != nil {
				return updateErr
			}
			return errors.ErrInsufficientBalance
		}

		// Perform the transfer
		newSourceBalance := sourceAccount.Balance.Sub(req.Amount)
		newDestBalance := destAccount.Balance.Add(req.Amount)

		// Update accounts
		if err := store.Account().UpdateAccountBalance(sourceID, newSourceBalance); err != nil {
			return err
		}

		if err := store.Account().UpdateAccountBalance(destID, newDestBalance); err != nil {
			return err
		}

		// Mark transaction as completed
		transaction.Status = "completed"
		return store.Transaction().UpdateTransactionStatus(transaction.ID, "completed")
	})

	if err != nil {
		s.logger.Error("Transfer failed", "error", err)
		return nil, err
	}

	s.logger.Info("Transfer completed successfully", "transaction_id", transaction.ID)
	return transaction, nil
}

func (s *TransactionService) parseAccountIDs(sourceIDStr, destIDStr string) (int64, int64, error) {
	sourceID, err := strconv.ParseInt(sourceIDStr, 10, 64)
	if err != nil || sourceID <= 0 {
		return 0, 0, errors.ErrInvalidAccountID
	}

	destID, err := strconv.ParseInt(destIDStr, 10, 64)
	if err != nil || destID <= 0 {
		return 0, 0, errors.ErrInvalidAccountID
	}

	return sourceID, destID, nil
}

func (s *TransactionService) validateTransfer(sourceID, destID int64, amount decimal.Decimal) error {
	if sourceID == destID {
		return errors.ErrSameAccountTransfer
	}

	if amount.IsNegative() || amount.IsZero() {
		return errors.NewAppError(errors.InvalidAmount, "amount must be positive")
	}

	// Validate reasonable limits
	maxAmount := decimal.NewFromInt(1_000_000_000) // 1 billion
	if amount.GreaterThan(maxAmount) {
		return errors.NewAppError(errors.InvalidAmount, "amount exceeds maximum limit")
	}

	minAmount := decimal.NewFromFloat(0.01)
	if amount.LessThan(minAmount) {
		return errors.NewAppError(errors.InvalidAmount, "amount below minimum limit")
	}

	return nil
}
