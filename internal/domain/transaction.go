package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Transaction struct {
	ID                   uuid.UUID       `json:"id"`
	SourceAccountID      uuid.UUID       `json:"source_account_id"`
	DestinationAccountID uuid.UUID       `json:"destination_account_id"`
	Amount               decimal.Decimal `json:"amount"`
	IdempotencyKey       uuid.UUID       `json:"idempotency_key"`
	Status               string          `json:"status"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}
