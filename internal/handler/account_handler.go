package handler

import (
	"encoding/json"
	"net/http"

	"internal-transfers/internal/errors"
	"internal-transfers/internal/service"

	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
)

type AccountHandler struct {
	accountService *service.AccountService
}

func NewAccountHandler(accountService *service.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

type CreateAccountRequest struct {
	AccountID      int64  `json:"account_id"`
	InitialBalance string `json:"initial_balance"`
}

type AccountResponse struct {
	AccountID int64  `json:"account_id"`
	Balance   string `json:"balance"`
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errors.NewAppError(errors.InvalidInput, "invalid request body"))
		return
	}

	initialBalance, err := decimal.NewFromString(req.InitialBalance)
	if err != nil {
		writeError(w, errors.NewAppError(errors.InvalidAmount, "invalid initial_balance format"))
		return
	}

	account, err := h.accountService.CreateAccount(req.AccountID, initialBalance)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			writeError(w, appErr)
		} else {
			writeError(w, errors.NewAppError(errors.InternalError, "an unexpected error occurred"))
		}
		return
	}

	response := AccountResponse{
		AccountID: account.ID,
		Balance:   account.Balance.String(),
	}

	writeJSON(w, http.StatusCreated, response)
}

func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	accountID := vars["account_id"]

	account, err := h.accountService.GetAccount(accountID)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			writeError(w, appErr)
		} else {
			writeError(w, errors.NewAppError(errors.InternalError, "an unexpected error occurred"))
		}
		return
	}

	response := AccountResponse{
		AccountID: account.ID,
		Balance:   account.Balance.String(),
	}

	writeJSON(w, http.StatusOK, response)
}
