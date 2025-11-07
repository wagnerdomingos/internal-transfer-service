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
