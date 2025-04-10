package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// HealthCheckHandler handles health check requests
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   "1.0.0", // This could be loaded from a version file or build info
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// NotFoundHandler handles 404 not found requests
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	resp := ErrorResponse{
		Error:   "not_found",
		Code:    http.StatusNotFound,
		Message: "The requested resource was not found",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(resp)
}

// MethodNotAllowedHandler handles 405 method not allowed requests
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	resp := ErrorResponse{
		Error:   "method_not_allowed",
		Code:    http.StatusMethodNotAllowed,
		Message: "The requested method is not allowed for this resource",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(resp)
}

// InternalErrorHandler handles 500 internal server error responses
func InternalErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	resp := ErrorResponse{
		Error:   "internal_server_error",
		Code:    http.StatusInternalServerError,
		Message: "An internal server error occurred",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(resp)
}
