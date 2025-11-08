package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Transaction struct {
	ID                   uuid.UUID       `json:"id"`
	SourceAccountID      int64           `json:"source_account_id"`
	DestinationAccountID int64           `json:"destination_account_id"`
	Amount               decimal.Decimal `json:"amount"`
	IdempotencyKey       uuid.UUID       `json:"idempotency_key,omitempty"`
	Status               string          `json:"status"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

type TransactionRepository interface {
	CreateTransaction(tx *Transaction) error
	GetTransactionByID(id uuid.UUID) (*Transaction, error)
	GetTransactionByIDempotencyKey(key uuid.UUID) (*Transaction, error)
	UpdateTransactionStatus(id uuid.UUID, status string) error
}
