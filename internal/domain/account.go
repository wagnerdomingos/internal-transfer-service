package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Account struct {
	ID        uuid.UUID       `json:"account_id"`
	Balance   decimal.Decimal `json:"balance"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type AccountRepository interface {
	CreateAccount(account *Account) error
	GetAccount(id uuid.UUID) (*Account, error)
	UpdateAccountBalance(id uuid.UUID, newBalance decimal.Decimal) error
	WithTransaction(fn func(repo AccountRepository) error) error
}
