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
	SourceAccountID      string `json:"source_account_id"`
	DestinationAccountID string `json:"destination_account_id"`
	Amount               string `json:"amount"`
	IdempotencyKey       string `json:"idempotency_key"`
}

type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

func (h *TransactionHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.NewAppError(errors.InvalidInput, "invalid request body"))
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeError(w, errors.NewAppError(errors.InvalidAmount, "invalid amount format"))
		return
	}

	idempotencyKey, err := uuid.Parse(req.IdempotencyKey)
	if err != nil {
		writeError(w, errors.NewAppError(errors.InvalidInput, "invalid idempotency_key format"))
		return
	}

	transferReq := &service.TransferRequest{
		SourceAccountID:      req.SourceAccountID,
		DestinationAccountID: req.DestinationAccountID,
		Amount:               amount,
		IdempotencyKey:       idempotencyKey,
	}

	transaction, err := h.transactionService.Transfer(transferReq)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			writeError(w, appErr)
		} else {
			writeError(w, errors.NewAppError(errors.InternalError, "an unexpected error occurred"))
		}
		return
	}

	response := TransferResponse{
		TransactionID: transaction.ID.String(),
		Status:        transaction.Status,
	}

	writeJSON(w, http.StatusCreated, response)
}
