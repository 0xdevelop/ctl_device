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
	auth := NewAuthenticator("test-token")
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	
	err := auth.Validate(req)
	
	if err != nil {
		t.Errorf("Expected no error with valid bearer token, got: %v", err)
	}
}

func TestAuthenticator_Validate_InvalidBearerToken(t *testing.T) {
	auth := NewAuthenticator("test-token")
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	
	err := auth.Validate(req)
	
	if err == nil {
		t.Error("Expected error with invalid bearer token, got nil")
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
	auth := NewAuthenticator("test-token")
	
	body := map[string]interface{}{
		"auth": map[string]interface{}{
			"token": "test-token",
		},
		"method": "test",
	}
	bodyBytes, _ := json.Marshal(body)
	
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	
	err := auth.Validate(req)
	
	if err != nil {
		t.Errorf("Expected no error with valid JSON token, got: %v", err)
	}
}

func TestAuthenticator_Validate_InvalidJSONToken(t *testing.T) {
	auth := NewAuthenticator("test-token")
	
	body := map[string]interface{}{
		"auth": map[string]interface{}{
			"token": "wrong-token",
		},
		"method": "test",
	}
	bodyBytes, _ := json.Marshal(body)
	
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	
	err := auth.Validate(req)
	
	if err == nil {
		t.Error("Expected error with invalid JSON token, got nil")
	}
}

func TestAuthenticator_Validate_MissingAuth(t *testing.T) {
	auth := NewAuthenticator("test-token")
	
	body := map[string]interface{}{
		"method": "test",
	}
	bodyBytes, _ := json.Marshal(body)
	
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	
	err := auth.Validate(req)
	
	if err == nil {
		t.Error("Expected error with missing auth, got nil")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	}
	
	if !strings.Contains(authErr.Message, "missing or invalid") {
		t.Errorf("Expected 'missing or invalid' message, got: %s", authErr.Message)
	}
}

func TestAuthenticator_Middleware(t *testing.T) {
	auth := NewAuthenticator("test-token")
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	middleware := auth.Middleware(handler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestAuthenticator_Middleware_Unauthorized(t *testing.T) {
	auth := NewAuthenticator("test-token")
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	middleware := auth.Middleware(handler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
	
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc version 2.0, got: %v", resp["jsonrpc"])
	}
	
	errorObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}
	
	if code, ok := errorObj["code"].(float64); !ok || code != -32001 {
		t.Errorf("Expected error code -32001, got: %v", errorObj["code"])
	}
}

func TestAuthError_Error(t *testing.T) {
	err := &AuthError{Message: "test error"}
	
	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got: %s", err.Error())
	}
}
