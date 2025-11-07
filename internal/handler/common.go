package handler

import (
	"encoding/json"
	"net/http"

	"internal-transfers/internal/errors"
)

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{Data: data}
	json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, appErr *errors.AppError) {
	w.Header().Set("Content-Type", "application/json")

	statusCode := appErr.HTTPStatus()
	errResponse := Error{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Details: appErr.Details,
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{Error: &errResponse})
}
