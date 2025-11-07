package errors

import (
	"fmt"
)

type ErrorCode string

const (
	AccountNotFound      ErrorCode = "account_not_found"
	DuplicateAccount     ErrorCode = "duplicate_account"
	DuplicateTransaction ErrorCode = "duplicate_transaction"
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

// Predefined errors for common cases
var (
	ErrAccountNotFound      = NewAppError(AccountNotFound, "account not found")
	ErrDuplicateAccount     = NewAppError(DuplicateAccount, "account already exists")
	ErrDuplicateTransaction = NewAppError(DuplicateTransaction, "transaction already processed")
)
