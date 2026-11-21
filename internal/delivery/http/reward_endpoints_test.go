package httpdelivery_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

const testReward = "80000000-0000-4000-8000-000000000001"

// rewardMemory: one reward costing 100; balance 150; keys replay.
type rewardMemory struct {
	balance int64
	keys    map[string]entity.Redemption
}

func (m *rewardMemory) Rewards(context.Context) ([]entity.Reward, error) {
	return []entity.Reward{{ID: testReward, CostPoints: 100, Stock: 1, Available: true}}, nil
}
func (m *rewardMemory) Reward(_ context.Context, id string) (entity.Reward, error) {
	if id != testReward {
		return entity.Reward{}, service.ErrRewardNotFound
	}
	return entity.Reward{ID: id}, nil
}
func (m *rewardMemory) Redeem(_ context.Context, in service.NewRedemption) (entity.Redemption, bool, error) {
	if r, ok := m.keys[in.IdempotencyKey]; ok {
		return r, false, nil
	}
	if in.RewardID != testReward {
		return entity.Redemption{}, false, service.ErrRewardNotFound
	}
	if m.balance < 100 {
		return entity.Redemption{}, false, service.ErrInsufficientPoints
	}
	m.balance -= 100
	r := entity.Redemption{Voucher: entity.Voucher{Code: in.VoucherCode, Status: "issued"}, WalletBalance: m.balance}
	m.keys[in.IdempotencyKey] = r
	return r, true, nil
}
func (m *rewardMemory) Vouchers(context.Context, string, *string, int, *service.Cursor) ([]entity.Voucher, error) {
	return []entity.Voucher{}, nil
}
func (m *rewardMemory) Voucher(context.Context, string, string) (entity.Voucher, error) {
	return entity.Voucher{}, service.ErrVoucherNotFound
}
func (m *rewardMemory) UseVoucher(context.Context, string, string) (entity.Voucher, error) {
	return entity.Voucher{}, service.ErrVoucherExpired
}

func TestRewardHTTP(t *testing.T) {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "u"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterRewardRoutes(mux, verifier, handler.NewRewardHandler(service.NewRewardService(&rewardMemory{balance: 150, keys: map[string]entity.Redemption{}})))
	redeem := func(key, id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/rewards/"+id+"/redeem", nil)
		r.Header.Set("Authorization", "Bearer test")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	for _, tt := range []struct {
		name, key, id string
		want          int
		body          string
	}{
		{"created", "redeem-0001", testReward, 201, `"code":"ECO-`},
		{"replay", "redeem-0001", testReward, 200, `"wallet_balance":50`},
		{"insufficient", "redeem-0002", testReward, 409, "insufficient_points"},
		{"missing key", "", testReward, 400, "invalid_idempotency_key"},
		{"bad id", "redeem-0003", "not-a-uuid", 404, "reward_not_found"},
	} {
		if w := redeem(tt.key, tt.id); w.Code != tt.want || !strings.Contains(w.Body.String(), tt.body) {
			t.Fatalf("%s: %d %s", tt.name, w.Code, w.Body)
		}
	}
	for path, want := range map[string]int{
		"/api/v1/rewards":                     200,
		"/api/v1/rewards?uid=x":               400,
		"/api/v1/me/vouchers?status=expired":  200,
		"/api/v1/me/vouchers?status=refunded": 400,
		"/api/v1/me/vouchers/" + testReward:   404,
	} {
		if w := accountRequest(mux, "GET", path, ""); w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	if w := accountRequest(mux, "POST", "/api/v1/me/vouchers/"+testReward+"/use", ""); w.Code != http.StatusGone {
		t.Fatalf("expired use: %d %s", w.Code, w.Body)
	}
}
