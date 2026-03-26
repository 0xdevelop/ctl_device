package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticator_Validate_NoToken(t *testing.T) {
	auth := NewAuthenticator("")
	req := httptest.NewRequest("GET", "/test", nil)
	err := auth.Validate(req)
	if err != nil {
		t.Errorf("Expected no error when token is empty, got: %v", err)
	}
}

func TestAuthenticator_Validate_ValidBearerToken(t *testing.T) {
	auth := NewAuthenticator("mytoken")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer mytoken")
	err := auth.Validate(req)
	if err != nil {
		t.Errorf("Expected no error with valid Bearer token, got: %v", err)
	}
}

func TestAuthenticator_Validate_InvalidBearerToken(t *testing.T) {
	auth := NewAuthenticator("mytoken")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	err := auth.Validate(req)
	if err == nil {
		t.Error("Expected error with invalid Bearer token, got nil")
	}
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	}
	if authErr.Message != "invalid token" {
		t.Errorf("Expected 'invalid token' message, got: %s", authErr.Message)
	}
}

func TestAuthenticator_Validate_ValidJSONToken(t *testing.T) {
	auth := NewAuthenticator("mytoken")
	body := map[string]interface{}{
		"auth": map[string]interface{}{
			"token": "mytoken",
		},
		"method": "test",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	err := auth.Validate(req)
	if err != nil {
		t.Errorf("Expected no error with valid JSON token, got: %v", err)
	}
}

func TestAuthenticator_Validate_InvalidJSONToken(t *testing.T) {
	auth := NewAuthenticator("mytoken")
	body := map[string]interface{}{
		"auth": map[string]interface{}{
			"token": "wrongtoken",
		},
		"method": "test",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	err := auth.Validate(req)
	if err == nil {
		t.Error("Expected error with invalid JSON token, got nil")
	}
}

func TestAuthenticator_Validate_MissingAuth(t *testing.T) {
	auth := NewAuthenticator("mytoken")
	body := map[string]interface{}{
		"method": "test",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	err := auth.Validate(req)
	if err == nil {
		t.Error("Expected error with missing auth, got nil")
	}
}

func TestAuthenticator_Middleware_ValidToken(t *testing.T) {
	auth := NewAuthenticator("mytoken")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := auth.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer mytoken")
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestAuthenticator_Middleware_InvalidToken(t *testing.T) {
	auth := NewAuthenticator("mytoken")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := auth.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}

	errMap, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}
	if errMap["code"] != float64(-32001) {
		t.Errorf("Expected error code -32001, got %v", errMap["code"])
	}
	if !strings.Contains(errMap["message"].(string), "Unauthorized") {
		t.Errorf("Expected 'Unauthorized' in message, got: %s", errMap["message"])
	}
}

func TestAuthenticator_Middleware_NoToken(t *testing.T) {
	auth := NewAuthenticator("")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := auth.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 when no token required, got %d", rr.Code)
	}
}
