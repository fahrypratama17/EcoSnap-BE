package bootstrap

import (
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/config"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
)

func NewServer(cfg config.Config, verifier middleware.TokenVerifier, pool *pgxpool.Pool) *http.Server {
	mux := httpdelivery.NewRouter(verifier)
	accounts := handler.NewAccountHandler(service.NewAccountService(postgres.NewAccountRepository(pool)))
	httpdelivery.RegisterAccountRoutes(mux, verifier, accounts)
	partners := handler.NewPartnerHandler(service.NewPartnerService(postgres.NewPartnerRepository(pool)))
	httpdelivery.RegisterPartnerRoutes(mux, verifier, partners)
	favorites := handler.NewFavoriteHandler(service.NewFavoriteService(postgres.NewFavoriteRepository(pool)))
	httpdelivery.RegisterFavoriteRoutes(mux, verifier, favorites)
	estimates := handler.NewPointEstimationHandler(service.NewPointEstimationService(postgres.NewPointRateRepository(pool)))
	httpdelivery.RegisterPointEstimateRoutes(mux, verifier, estimates)
	depositRepo := postgres.NewDepositRepository(pool)
	deposits := handler.NewDepositHandler(service.NewDepositService(depositRepo))
	httpdelivery.RegisterDepositRoutes(mux, verifier, deposits)
	httpdelivery.RegisterDepositQRRoutes(mux, verifier, handler.NewDepositQRHandler(service.NewDepositQRService(depositRepo)))
	httpdelivery.RegisterWalletRoutes(mux, verifier, handler.NewWalletHandler(service.NewWalletService(postgres.NewWalletRepository(pool))))
	httpdelivery.RegisterRewardRoutes(mux, verifier, handler.NewRewardHandler(service.NewRewardService(postgres.NewRewardRepository(pool))))
	httpdelivery.RegisterArticleRoutes(mux, verifier, handler.NewArticleHandler(service.NewArticleService(postgres.NewArticleRepository(pool))))
	gamification := handler.NewGamificationHandler(service.NewGamificationService(postgres.NewGamificationRepository(pool)))
	httpdelivery.RegisterGamificationRoutes(mux, verifier, gamification)
	httpdelivery.RegisterSettingsRoutes(mux, verifier, handler.NewSettingsHandler(service.NewSettingsService(postgres.NewSettingsRepository(pool))))
	httpdelivery.RegisterActivityReportRoutes(mux, verifier, handler.NewActivityReportHandler(service.NewActivityReportService(postgres.NewActivityReportRepository(pool))))
	httpdelivery.RegisterNotificationRoutes(mux, verifier, handler.NewNotificationHandler(service.NewNotificationService(postgres.NewNotificationRepository(pool))))
	httpdelivery.RegisterSupportRoutes(mux, verifier, handler.NewSupportHandler(service.NewSupportService(postgres.NewSupportRepository(pool))))
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpdelivery.NewHandler(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		MaxHeaderBytes:    16 * 1024,
		IdleTimeout:       60 * time.Second,
	}
}
