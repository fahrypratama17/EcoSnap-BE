package middleware

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
)

type userUIDKey struct{}

// TokenVerifier is implemented by the Firebase Auth client. Tests can supply
// a verifier without credentials or a network connection.
type TokenVerifier interface {
	VerifyIDToken(context.Context, string) (*auth.Token, error)
}

// Identity contains Firebase identity claims only. Application roles and account
// status must be loaded from PostgreSQL before authorizing business operations.
type Identity struct {
	UID           string `json:"uid"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// RevocationVerifier is implemented by the Firebase Auth client. It also
// rejects revoked tokens and disabled users, at the cost of a network call.
type RevocationVerifier interface {
	VerifyIDTokenAndCheckRevoked(context.Context, string) (*auth.Token, error)
}

func RequireAuth(client TokenVerifier) func(http.Handler) http.Handler {
	return requireAuth(client.VerifyIDToken)
}

// RequireFreshAuth guards operations that move points (claim, redeem): a
// disabled Firebase account cannot use a still-unexpired token there. A
// verifier without revocation support (test doubles) falls back to
// VerifyIDToken.
func RequireFreshAuth(client TokenVerifier) func(http.Handler) http.Handler {
	if revocable, ok := client.(RevocationVerifier); ok {
		return requireAuth(revocable.VerifyIDTokenAndCheckRevoked)
	}
	return requireAuth(client.VerifyIDToken)
}

func requireAuth(verify func(context.Context, string) (*auth.Token, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				w.Header().Set("WWW-Authenticate", "Bearer")
				response.Error(w, http.StatusUnauthorized, "unauthorized", "ID token Firebase wajib disertakan sebagai Bearer token.")
				return
			}

			tokenStatus, err := verify(r.Context(), parts[1])
			if err != nil || tokenStatus == nil || tokenStatus.UID == "" {
				w.Header().Set("WWW-Authenticate", "Bearer")
				response.Error(w, http.StatusUnauthorized, "unauthorized", "ID token Firebase tidak valid atau sudah kedaluwarsa.")
				return
			}

			email, _ := tokenStatus.Claims["email"].(string)
			emailVerified, _ := tokenStatus.Claims["email_verified"].(bool)
			identity := Identity{UID: tokenStatus.UID, Email: email, EmailVerified: emailVerified}
			ctx := context.WithValue(r.Context(), userUIDKey{}, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UIDFromContext(ctx context.Context) (string, bool) {
	identity, ok := IdentityFromContext(ctx)
	return identity.UID, ok
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(userUIDKey{}).(Identity)
	return identity, ok
}
