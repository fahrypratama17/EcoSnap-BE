package httpdelivery_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

// qrMemory accepts only the hash of the last issued token; one claim total.
type qrMemory struct{ hash, claimer string }

func (m *qrMemory) IssueQR(_ context.Context, uid, _, hash string, ttl time.Duration) (service.QRRecord, error) {
	if uid != "mitra" {
		return service.QRRecord{}, service.ErrDepositForbidden
	}
	m.hash = hash
	now := time.Now()
	return service.QRRecord{IssuedAt: now, ExpiresAt: now.Add(ttl)}, nil
}
func (m *qrMemory) PreviewQR(_ context.Context, _, hash string) (service.ClaimPreviewRecord, error) {
	if hash != m.hash {
		return service.ClaimPreviewRecord{}, service.ErrQRInvalid
	}
	return service.ClaimPreviewRecord{Deposit: service.DepositRecord{ID: "d", TotalPoint: 455}, PartnerName: "Tunas"}, nil
}
func (m *qrMemory) ClaimQR(_ context.Context, uid, hash string) (service.ClaimRecord, bool, error) {
	switch {
	case hash != m.hash:
		return service.ClaimRecord{}, false, service.ErrQRInvalid
	case m.claimer == uid:
		return service.ClaimRecord{Points: 455}, false, nil
	case m.claimer != "":
		return service.ClaimRecord{}, false, service.ErrDepositAlreadyClaimed
	}
	m.claimer = uid
	return service.ClaimRecord{Points: 455, Balance: 455}, true, nil
}

func TestDepositQRHTTP(t *testing.T) {
	uid := "mitra"
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: uid}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterDepositQRRoutes(mux, verifier, handler.NewDepositQRHandler(service.NewDepositQRService(&qrMemory{})))

	w := accountRequest(mux, "POST", "/api/v1/deposits/60000000-0000-4000-8000-000000000001/qr", "")
	var issued struct {
		Data struct{ Token, Warning string } `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &issued); err != nil || w.Code != 201 || len(issued.Data.Token) != 43 || issued.Data.Warning != service.QRBearerWarning {
		t.Fatalf("issue: %d %s", w.Code, w.Body)
	}
	body := `{"token":"` + issued.Data.Token + `"}`
	uid = "user-a"
	for _, tt := range []struct {
		name, path, body string
		want             int
	}{
		{"preview", "/api/v1/deposit-claims/preview", body, 200},
		{"token in query", "/api/v1/deposit-claims?token=" + issued.Data.Token, body, 404},
		{"extra field", "/api/v1/deposit-claims", strings.Replace(body, "{", `{"user_uid":"x",`, 1), 400},
		{"malformed token", "/api/v1/deposit-claims", `{"token":"abc"}`, 404},
		{"claim", "/api/v1/deposit-claims", body, 201},
		{"replay", "/api/v1/deposit-claims", body, 200},
	} {
		if w := accountRequest(mux, "POST", tt.path, tt.body); w.Code != tt.want {
			t.Fatalf("%s: %d %s", tt.name, w.Code, w.Body)
		}
	}
	uid = "user-b"
	if w := accountRequest(mux, "POST", "/api/v1/deposit-claims", body); w.Code != 409 || !strings.Contains(w.Body.String(), "deposit_already_claimed") {
		t.Fatalf("second claimer: %d %s", w.Code, w.Body)
	}
	if w := accountRequest(mux, "POST", "/api/v1/deposits/60000000-0000-4000-8000-000000000001/qr", ""); w.Code != 403 {
		t.Fatalf("user issue: %d %s", w.Code, w.Body)
	}
}
