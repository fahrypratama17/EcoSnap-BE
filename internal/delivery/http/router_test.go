package httpdelivery_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"firebase.google.com/go/v4/auth"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
)

type verifierFunc func(context.Context, string) (*auth.Token, error)

func (f verifierFunc) VerifyIDToken(ctx context.Context, token string) (*auth.Token, error) {
	return f(ctx, token)
}

func TestAuthMeRejectsInvalidAuthentication(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		result     *auth.Token
		err        error
		wantVerify bool
	}{
		{name: "missing header"},
		{name: "empty bearer", header: "Bearer "},
		{name: "wrong scheme", header: "Basic abc"},
		{name: "extra token", header: "Bearer abc def"},
		{name: "verification failed", header: "Bearer invalid", err: errors.New("sensitive SDK error"), wantVerify: true},
		{name: "nil token", header: "Bearer invalid", wantVerify: true},
		{name: "empty UID", header: "Bearer invalid", result: &auth.Token{}, wantVerify: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			router := httpdelivery.NewRouter(verifierFunc(func(context.Context, string) (*auth.Token, error) {
				called = true
				return tt.result, tt.err
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", w.Code)
			}
			if called != tt.wantVerify {
				t.Fatalf("verifier called = %v, want %v", called, tt.wantVerify)
			}
			if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal("error response must be JSON")
			}
			if w.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing Bearer challenge")
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error.Code != "unauthorized" {
				t.Fatalf("unexpected error body: %s", w.Body.String())
			}
			if strings.Contains(w.Body.String(), "sensitive SDK error") {
				t.Fatal("SDK error leaked into response")
			}
		})
	}
}

func TestAuthMeUsesVerifiedClaims(t *testing.T) {
	for _, header := range []string{"Bearer good-token", "bearer good-token"} {
		t.Run(header, func(t *testing.T) {
			type requestKey struct{}
			router := httpdelivery.NewRouter(verifierFunc(func(ctx context.Context, token string) (*auth.Token, error) {
				if token != "good-token" || ctx.Value(requestKey{}) != "request-context" {
					t.Fatal("verifier did not receive the token and request context")
				}
				return &auth.Token{
					UID:    "firebase-user-1",
					Claims: map[string]any{"email": "user@example.com", "email_verified": false},
				}, nil
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me?uid=someone-else", nil)
			req = req.WithContext(context.WithValue(req.Context(), requestKey{}, "request-context"))
			req.Header.Set("Authorization", header)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			var body struct {
				Data struct {
					UID           string `json:"uid"`
					Email         string `json:"email"`
					EmailVerified bool   `json:"email_verified"`
				} `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Data.UID != "firebase-user-1" || body.Data.Email != "user@example.com" || body.Data.EmailVerified {
				t.Fatalf("unexpected identity: %+v", body.Data)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("identity response must not be cached")
			}
		})
	}
}

func TestHealthDoesNotRequireAuthentication(t *testing.T) {
	router := httpdelivery.NewRouter(verifierFunc(func(context.Context, string) (*auth.Token, error) {
		t.Fatal("public health endpoint must not invoke Firebase")
		return nil, nil
	}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK || w.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("unexpected health response: %d %s", w.Code, w.Body.String())
	}
}
