package service

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"internal-transfers/internal/domain"
	"internal-transfers/internal/errors"
)

type AccountService struct {
	accountRepo domain.AccountRepository
	logger      *slog.Logger
}

func NewAccountService(accountRepo domain.AccountRepository, logger *slog.Logger) *AccountService {
	return &AccountService{
		accountRepo: accountRepo,
		logger:      logger,
	}
}

func (s *AccountService) CreateAccount(initialBalance decimal.Decimal) (*domain.Account, error) {
	s.logger.Info("Creating account", "initial_balance", initialBalance)

	if initialBalance.IsNegative() {
		return nil, errors.ErrInvalidAmount
	}

	account := &domain.Account{
		ID:      uuid.New(),
		Balance: initialBalance,
	}

	if err := s.accountRepo.CreateAccount(account); err != nil {
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

	return s.accountRepo.GetAccount(id)
}
