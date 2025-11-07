package service

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"internal-transfers/internal/domain"
	"internal-transfers/internal/errors"
)

type TransactionService struct {
	accountRepo     domain.AccountRepository
	transactionRepo domain.TransactionRepository
	logger          *slog.Logger
}

func NewTransactionService(
	accountRepo domain.AccountRepository,
	transactionRepo domain.TransactionRepository,
	logger *slog.Logger,
) *TransactionService {
	return &TransactionService{
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		logger:          logger,
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

	// Check for existing transaction with same idempotency key
	existingTx, err := s.transactionRepo.GetTransactionByIDempotencyKey(req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existingTx != nil {
		s.logger.Info("Returning existing transaction for idempotency key",
			"idempotency_key", req.IdempotencyKey,
			"transaction_id", existingTx.ID)
		return existingTx, nil
	}

	// Validate transfer
	if err := s.validateTransfer(sourceID, destID, req.Amount); err != nil {
		return nil, err
	}

	// Create transaction record
	transaction := &domain.Transaction{
		ID:                   uuid.New(),
		SourceAccountID:      sourceID,
		DestinationAccountID: destID,
		Amount:               req.Amount,
		IdempotencyKey:       req.IdempotencyKey,
		Status:               "pending",
	}

	// Process transfer in a single database transaction
	err = s.accountRepo.WithTransaction(func(accountRepo domain.AccountRepository) error {
		// Re-get accounts within transaction to ensure consistency
		sourceAccount, err := accountRepo.GetAccount(sourceID)
		if err != nil {
			return err
		}

		destAccount, err := accountRepo.GetAccount(destID)
		if err != nil {
			return err
		}

		// Check sufficient balance
		if sourceAccount.Balance.LessThan(req.Amount) {
			return errors.ErrInsufficientBalance
		}

		// Perform the transfer
		newSourceBalance := sourceAccount.Balance.Sub(req.Amount)
		newDestBalance := destAccount.Balance.Add(req.Amount)

		// Update accounts
		if err := accountRepo.UpdateAccountBalance(sourceID, newSourceBalance); err != nil {
			return err
		}

		if err := accountRepo.UpdateAccountBalance(destID, newDestBalance); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Transfer failed", "error", err)
		// Still create the transaction record but mark as failed
		transaction.Status = "failed"
		if createErr := s.transactionRepo.CreateTransaction(transaction); createErr != nil {
			s.logger.Error("Failed to create failed transaction record", "error", createErr)
		}
		return nil, err
	}

	// Mark transaction as completed
	transaction.Status = "completed"
	if err := s.transactionRepo.CreateTransaction(transaction); err != nil {
		s.logger.Error("Failed to create completed transaction record", "error", err)
		return nil, err
	}

	s.logger.Info("Transfer completed successfully", "transaction_id", transaction.ID)
	return transaction, nil
}

func (s *TransactionService) parseAccountIDs(sourceIDStr, destIDStr string) (uuid.UUID, uuid.UUID, error) {
	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.ErrInvalidAccountID
	}

	destID, err := uuid.Parse(destIDStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, errors.ErrInvalidAccountID
	}

	return sourceID, destID, nil
}

func (s *TransactionService) validateTransfer(sourceID, destID uuid.UUID, amount decimal.Decimal) error {
	if sourceID == destID {
		return errors.ErrSameAccountTransfer
	}

	if amount.IsNegative() || amount.IsZero() {
		return errors.ErrInvalidAmount
	}

	return nil
}
