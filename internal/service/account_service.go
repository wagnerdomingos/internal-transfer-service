package service

import (
	"log/slog"

	"github.com/google/uuid"
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

func (s *AccountService) CreateAccount(initialBalance decimal.Decimal) (*domain.Account, error) {
	s.logger.Info("Creating account", "initial_balance", initialBalance)

	if initialBalance.IsNegative() {
		return nil, errors.ErrInvalidAmount
	}

	// Validate reasonable limits
	maxInitialBalance := decimal.NewFromInt(10_000_000_000) // 10 billion
	if initialBalance.GreaterThan(maxInitialBalance) {
		return nil, errors.NewAppError(errors.InvalidAmount, "initial balance exceeds maximum limit")
	}

	account := &domain.Account{
		ID:      uuid.New(),
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

	id, err := uuid.Parse(accountID)
	if err != nil {
		return nil, errors.ErrInvalidAccountID
	}

	return s.store.Account().GetAccount(id)
}
