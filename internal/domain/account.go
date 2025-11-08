package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Account struct {
	ID        int64           `json:"account_id"`
	Balance   decimal.Decimal `json:"balance"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type AccountRepository interface {
	CreateAccount(account *Account) error
	GetAccount(id int64) (*Account, error)
	GetAccountForUpdate(id int64) (*Account, error)
	UpdateAccountBalance(id int64, newBalance decimal.Decimal) error
}
