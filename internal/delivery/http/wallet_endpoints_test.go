package httpdelivery_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

// walletMemory returns three ledger entries and records the requested limit.
type walletMemory struct{ limit int }

func (m *walletMemory) Wallet(context.Context, string) (entity.Wallet, error) {
	return entity.Wallet{Balance: 450}, nil
}
func (m *walletMemory) PointHistory(_ context.Context, _ string, limit int, after *service.Cursor) ([]entity.PointHistoryEntry, error) {
	m.limit = limit
	out := []entity.PointHistoryEntry{}
	for i := range 3 {
		id := []string{"70000000-0000-4000-8000-000000000003", "70000000-0000-4000-8000-000000000002", "70000000-0000-4000-8000-000000000001"}[i]
		if after != nil && id >= after.ID {
			continue
		}
		out = append(out, entity.PointHistoryEntry{ID: id, Amount: 150, Source: "deposit", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *walletMemory) PartnerDeposits(context.Context, string, *string, int, *service.Cursor) ([]entity.PartnerDepositSummary, error) {
	return []entity.PartnerDepositSummary{}, nil
}

func TestWalletHTTP(t *testing.T) {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "u"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	store := &walletMemory{}
	httpdelivery.RegisterWalletRoutes(mux, verifier, handler.NewWalletHandler(service.NewWalletService(store)))

	type pageBody struct {
		Data       []entity.PointHistoryEntry `json:"data"`
		Pagination struct {
			Limit      int     `json:"limit"`
			NextCursor *string `json:"next_cursor"`
		} `json:"pagination"`
	}
	get := func(path string, want int) pageBody {
		t.Helper()
		w := accountRequest(mux, "GET", path, "")
		var b pageBody
		if w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		_ = json.Unmarshal(w.Body.Bytes(), &b)
		return b
	}
	if b := get("/api/v1/me/point-history", 200); store.limit != 21 || len(b.Data) != 3 || b.Pagination.NextCursor != nil || b.Pagination.Limit != 20 {
		t.Fatalf("default page: limit=%d %+v", store.limit, b)
	}
	first := get("/api/v1/me/point-history?limit=2", 200)
	if len(first.Data) != 2 || first.Pagination.NextCursor == nil {
		t.Fatalf("first page: %+v", first)
	}
	second := get("/api/v1/me/point-history?limit=2&cursor="+*first.Pagination.NextCursor, 200)
	if len(second.Data) != 1 || second.Data[0].ID != "70000000-0000-4000-8000-000000000001" || second.Pagination.NextCursor != nil {
		t.Fatalf("second page: %+v", second)
	}
	for _, path := range []string{
		"/api/v1/me/point-history?limit=0",
		"/api/v1/me/point-history?limit=x",
		"/api/v1/me/point-history?limit=1&limit=2",
		"/api/v1/me/point-history?uid=other",
		"/api/v1/me/point-history?status=claimed",
		"/api/v1/me/point-history?cursor=bad!",
		"/api/v1/partner/deposits?status=cancelled",
		"/api/v1/me/wallet?uid=other",
	} {
		get(path, 400)
	}
	get("/api/v1/partner/deposits?status=claimed&limit=5", 200)
	if w := accountRequest(mux, "GET", "/api/v1/me/wallet", ""); w.Code != 200 {
		t.Fatalf("wallet: %d %s", w.Code, w.Body)
	}
}
