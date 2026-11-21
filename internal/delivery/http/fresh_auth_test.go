package httpdelivery_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
)

// revocableVerifier accepts every token in VerifyIDToken but rejects it in
// the revocation check, like a Firebase user disabled after token issue.
type revocableVerifier struct{}

func (revocableVerifier) VerifyIDToken(context.Context, string) (*auth.Token, error) {
	return &auth.Token{UID: "disabled-user"}, nil
}
func (revocableVerifier) VerifyIDTokenAndCheckRevoked(context.Context, string) (*auth.Token, error) {
	return nil, errors.New("user disabled")
}

func TestFreshAuthRejectsRevokedTokens(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	for name, tt := range map[string]struct {
		guard func(middleware.TokenVerifier) func(http.Handler) http.Handler
		want  int
	}{
		"plain auth accepts until expiry": {middleware.RequireAuth, 204},
		"fresh auth checks revocation":    {middleware.RequireFreshAuth, 401},
	} {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("Authorization", "Bearer token")
		w := httptest.NewRecorder()
		tt.guard(revocableVerifier{})(ok).ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Fatalf("%s: %d", name, w.Code)
		}
	}
}
