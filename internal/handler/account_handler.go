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
	InitialBalance string `json:"initial_balance"`
}

type AccountResponse struct {
	ID      string `json:"id"`
	Balance string `json:"balance"`
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

	account, err := h.accountService.CreateAccount(initialBalance)
	if err != nil {
		writeError(w, err.(*errors.AppError))
		return
	}

	response := AccountResponse{
		ID:      account.ID.String(),
		Balance: account.Balance.String(),
	}

	writeJSON(w, http.StatusCreated, response)
}

func (h *AccountHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	accountID := vars["account_id"]

	account, err := h.accountService.GetAccount(accountID)
	if err != nil {
		writeError(w, err.(*errors.AppError))
		return
	}

	response := AccountResponse{
		ID:      account.ID.String(),
		Balance: account.Balance.String(),
	}

	writeJSON(w, http.StatusOK, response)
}
