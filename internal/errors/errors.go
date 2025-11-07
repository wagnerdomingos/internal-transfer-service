package errors

import (
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	InvalidInput         ErrorCode = "invalid_input"
	AccountNotFound      ErrorCode = "account_not_found"
	InsufficientBalance  ErrorCode = "insufficient_balance"
	DuplicateAccount     ErrorCode = "duplicate_account"
	DuplicateTransaction ErrorCode = "duplicate_transaction"
	InvalidAmount        ErrorCode = "invalid_amount"
	SameAccountTransfer  ErrorCode = "same_account_transfer"
	InternalError        ErrorCode = "internal_error"
)

type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
}

func (e AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

func NewAppErrorf(code ErrorCode, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// HTTPStatus returns the appropriate HTTP status code for the error
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case InvalidInput, InvalidAmount, SameAccountTransfer:
		return http.StatusBadRequest
	case AccountNotFound:
		return http.StatusNotFound
	case InsufficientBalance:
		return http.StatusUnprocessableEntity
	case DuplicateAccount, DuplicateTransaction:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// Predefined errors for common cases
var (
	ErrInvalidAccountID     = NewAppError(InvalidInput, "invalid account ID")
	ErrAccountNotFound      = NewAppError(AccountNotFound, "account not found")
	ErrInsufficientBalance  = NewAppError(InsufficientBalance, "insufficient balance")
	ErrDuplicateAccount     = NewAppError(DuplicateAccount, "account already exists")
	ErrDuplicateTransaction = NewAppError(DuplicateTransaction, "transaction already processed")
	ErrInvalidAmount        = NewAppError(InvalidAmount, "invalid amount")
	ErrSameAccountTransfer  = NewAppError(SameAccountTransfer, "source and destination accounts cannot be the same")
)
