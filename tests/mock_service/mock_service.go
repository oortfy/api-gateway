package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Response struct {
	Service string      `json:"service"`
	Path    string      `json:"path"`
	Method  string      `json:"method"`
	Headers http.Header `json:"headers"`
	Data    interface{} `json:"data"`
	Time    time.Time   `json:"time"`
}

func main() {
	serviceName := getEnv("SERVICE_NAME", "mock-service")
	port := getEnv("PORT", "8080")
	delay, _ := strconv.Atoi(getEnv("RESPONSE_DELAY", "0"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Simulate processing delay
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		// Special handler for API key validation
		if serviceName == "auth-service" && r.URL.Path == "/auth/validate-api-key" {
			handleAPIKeyValidation(w, r)
			return
		}

		// Regular response for all other paths
		resp := Response{
			Service: serviceName,
			Path:    r.URL.Path,
			Method:  r.Method,
			Headers: r.Header,
			Data:    map[string]string{"message": "This is a mock response"},
			Time:    time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)

		log.Printf("[%s] %s %s", serviceName, r.Method, r.URL.Path)
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "up",
			"service": serviceName,
		})
	})

	// WebSocket mock endpoint
	if serviceName == "websocket-service" {
		http.HandleFunc("/socket", func(w http.ResponseWriter, r *http.Request) {
			// This is a placeholder for WebSocket - in a real implementation
			// we would upgrade the connection here
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "WebSocket endpoint")
		})
	}

	log.Printf("Starting %s on port %s", serviceName, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleAPIKeyValidation(w http.ResponseWriter, r *http.Request) {
	// Get API key from request
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("apiKey")
	}

	// Simple validation - accept any non-empty key
	if apiKey == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid or missing API key",
		})
		return
	}

	// Return success with fake user info
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid": true,
		"user": map[string]string{
			"id":    "mock-user-id",
			"name":  "Mock User",
			"email": "mock@example.com",
			"role":  "admin",
		},
	})
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
