package httpdelivery

import (
	"net/http"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
)

func NewRouter(verifier middleware.TokenVerifier) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.Handle("GET /api/v1/auth/me", middleware.RequireAuth(verifier)(http.HandlerFunc(handler.AuthMe)))
	return mux
}

func RegisterAccountRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, accounts *handler.AccountHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("POST /api/v1/accounts", protect(http.HandlerFunc(accounts.Register)))
	mux.Handle("GET /api/v1/me", protect(http.HandlerFunc(accounts.Profile)))
	mux.Handle("PATCH /api/v1/me", protect(http.HandlerFunc(accounts.UpdateProfile)))
}

func RegisterPartnerRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, partners *handler.PartnerHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/partners", protect(http.HandlerFunc(partners.List)))
	mux.Handle("GET /api/v1/partners/{partnerID}", protect(http.HandlerFunc(partners.Detail)))
	mux.Handle("GET /api/v1/waste-categories", protect(http.HandlerFunc(partners.Categories)))
}

func RegisterFavoriteRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, favorites *handler.FavoriteHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/me/favorites", protect(http.HandlerFunc(favorites.List)))
	mux.Handle("PUT /api/v1/me/favorites/{partnerID}", protect(http.HandlerFunc(favorites.Set)))
	mux.Handle("DELETE /api/v1/me/favorites/{partnerID}", protect(http.HandlerFunc(favorites.Set)))
}

func RegisterPointEstimateRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, estimates *handler.PointEstimationHandler) {
	mux.Handle("POST /api/v1/point-estimates", middleware.RequireAuth(verifier)(http.HandlerFunc(estimates.Estimate)))
}

func RegisterDepositRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, deposits *handler.DepositHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("POST /api/v1/deposits", protect(http.HandlerFunc(deposits.Create)))
	mux.Handle("GET /api/v1/deposits/{depositID}", protect(http.HandlerFunc(deposits.Detail)))
}

func RegisterDepositQRRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, qr *handler.DepositQRHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("POST /api/v1/deposits/{depositID}/qr", protect(http.HandlerFunc(qr.Issue)))
	mux.Handle("POST /api/v1/deposit-claims/preview", protect(http.HandlerFunc(qr.Preview)))
	mux.Handle("POST /api/v1/deposit-claims", middleware.RequireFreshAuth(verifier)(http.HandlerFunc(qr.Claim)))
}

func RegisterWalletRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, wallets *handler.WalletHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/me/wallet", protect(http.HandlerFunc(wallets.Wallet)))
	mux.Handle("GET /api/v1/me/point-history", protect(http.HandlerFunc(wallets.PointHistory)))
	mux.Handle("GET /api/v1/partner/deposits", protect(http.HandlerFunc(wallets.PartnerDeposits)))
	mux.Handle("GET /api/v1/me/deposits/{depositID}", protect(http.HandlerFunc(wallets.ClaimedDeposit)))
	mux.Handle("GET /api/v1/me/home", protect(http.HandlerFunc(wallets.UserHome)))
	mux.Handle("GET /api/v1/partner/home", protect(http.HandlerFunc(wallets.PartnerHome)))
}

func RegisterRewardRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, rewards *handler.RewardHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/rewards", protect(http.HandlerFunc(rewards.List)))
	mux.Handle("GET /api/v1/rewards/{rewardID}", protect(http.HandlerFunc(rewards.Detail)))
	mux.Handle("POST /api/v1/rewards/{rewardID}/redeem", middleware.RequireFreshAuth(verifier)(http.HandlerFunc(rewards.Redeem)))
	mux.Handle("GET /api/v1/me/vouchers", protect(http.HandlerFunc(rewards.Vouchers)))
	mux.Handle("GET /api/v1/me/vouchers/{voucherID}", protect(http.HandlerFunc(rewards.Voucher)))
	mux.Handle("POST /api/v1/me/vouchers/{voucherID}/use", protect(http.HandlerFunc(rewards.UseVoucher)))
}

func RegisterArticleRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, articles *handler.ArticleHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/articles", protect(http.HandlerFunc(articles.List)))
	mux.Handle("GET /api/v1/articles/{articleID}", protect(http.HandlerFunc(articles.Detail)))
}

func RegisterGamificationRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, gamification *handler.GamificationHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/me/missions", protect(http.HandlerFunc(gamification.Missions)))
	mux.Handle("POST /api/v1/me/missions/{missionID}/claim", protect(http.HandlerFunc(gamification.ClaimMission)))
	mux.Handle("GET /api/v1/me/badges", protect(http.HandlerFunc(gamification.Badges)))
	mux.Handle("GET /api/v1/leaderboard", protect(http.HandlerFunc(gamification.Leaderboard)))
	mux.Handle("PATCH /api/v1/me/leaderboard-visibility", protect(http.HandlerFunc(gamification.SetLeaderboardVisibility)))
}
