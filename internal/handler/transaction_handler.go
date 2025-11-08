package handler

import (
	"encoding/json"
	"net/http"

	"internal-transfers/internal/errors"
	"internal-transfers/internal/service"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TransactionHandler struct {
	transactionService *service.TransactionService
}

func NewTransactionHandler(transactionService *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

type TransferRequest struct {
	SourceAccountID      json.Number `json:"source_account_id"`      // Use json.Number
	DestinationAccountID json.Number `json:"destination_account_id"` // Use json.Number
	Amount               string      `json:"amount"`
	IdempotencyKey       string      `json:"idempotency_key,omitempty"`
}

type TransferResponse struct {
	TransactionID  string  `json:"transaction_id"`
	Status         string  `json:"status"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

func (h *TransactionHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.NewAppError(errors.InvalidInput, "invalid request body").WithDetails(err.Error()))
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeError(w, errors.NewAppError(errors.InvalidAmount, "invalid amount format").WithDetails(err.Error()))
		return
	}

	// Parse optional idempotency key
	var idempotencyKey *uuid.UUID
	if req.IdempotencyKey != "" {
		key, err := uuid.Parse(req.IdempotencyKey)
		if err != nil {
			writeError(w, errors.NewAppError(errors.InvalidInput, "invalid idempotency_key format").WithDetails(err.Error()))
			return
		}
		idempotencyKey = &key
	}

	transferReq := &service.TransferRequest{
		SourceAccountID:      req.SourceAccountID.String(),      // Convert to string
		DestinationAccountID: req.DestinationAccountID.String(), // Convert to string
		Amount:               amount,
		IdempotencyKey:       idempotencyKey,
	}

	transaction, err := h.transactionService.Transfer(transferReq)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			writeError(w, appErr)
		} else {
			writeError(w, errors.NewAppError(errors.InternalError, "an unexpected error occurred").WithDetails(err.Error()))
		}
		return
	}

	// Build response with optional idempotency key
	response := TransferResponse{
		TransactionID: transaction.ID.String(),
		Status:        transaction.Status,
	}

	if transaction.IdempotencyKey != nil {
		keyStr := transaction.IdempotencyKey.String()
		response.IdempotencyKey = &keyStr
	}

	writeJSON(w, http.StatusCreated, response)
}
