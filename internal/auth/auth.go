package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type Authenticator struct {
	token string
}

func NewAuthenticator(token string) *Authenticator {
	return &Authenticator{
		token: token,
	}
}

func (a *Authenticator) Validate(r *http.Request) error {
	if a.token == "" {
		return nil
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			requestToken := strings.TrimPrefix(authHeader, "Bearer ")
			if requestToken == a.token {
				return nil
			}
			return &AuthError{Message: "invalid token"}
		}
	}

	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			var req map[string]interface{}
			if err := json.Unmarshal(body, &req); err == nil {
				if auth, ok := req["auth"].(map[string]interface{}); ok {
					if requestToken, ok := auth["token"].(string); ok {
						if requestToken == a.token {
							return nil
						}
						return &AuthError{Message: "invalid token"}
					}
				}
			}
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}

	return &AuthError{Message: "missing or invalid authentication"}
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := a.Validate(r); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"error": map[string]interface{}{
					"code":    -32001,
					"message": "Unauthorized: " + err.Error(),
				},
				"id": nil,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
