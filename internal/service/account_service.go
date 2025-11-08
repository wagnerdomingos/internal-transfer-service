package service

import (
	"log/slog"
	"strconv"

	"github.com/shopspring/decimal"

	"internal-transfers/internal/domain"
	"internal-transfers/internal/errors"
	"internal-transfers/internal/repository"
)

type AccountService struct {
	store  *repository.Store
	logger *slog.Logger
}

func NewAccountService(store *repository.Store, logger *slog.Logger) *AccountService {
	return &AccountService{
		store:  store,
		logger: logger,
	}
}

func (s *AccountService) CreateAccount(accountID int64, initialBalance decimal.Decimal) (*domain.Account, error) {
	s.logger.Info("Creating account", "account_id", accountID, "initial_balance", initialBalance)

	if initialBalance.IsNegative() {
		return nil, errors.ErrInvalidAmount
	}

	// Validate reasonable limits
	maxInitialBalance := decimal.NewFromInt(10_000_000_000) // 10 billion
	if initialBalance.GreaterThan(maxInitialBalance) {
		return nil, errors.NewAppError(errors.InvalidAmount, "initial balance exceeds maximum limit")
	}

	// Validate account ID is positive
	if accountID <= 0 {
		return nil, errors.NewAppError(errors.InvalidInput, "account ID must be positive")
	}

	account := &domain.Account{
		ID:      accountID,
		Balance: initialBalance,
	}

	if err := s.store.Account().CreateAccount(account); err != nil {
		return nil, err
	}

	s.logger.Info("Account created successfully", "account_id", account.ID)
	return account, nil
}

func (s *AccountService) GetAccount(accountID string) (*domain.Account, error) {
	s.logger.Info("Getting account", "account_id", accountID)

	id, err := strconv.ParseInt(accountID, 10, 64)
	if err != nil || id <= 0 {
		return nil, errors.ErrInvalidAccountID
	}

	return s.store.Account().GetAccount(id)
}
