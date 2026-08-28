// Command server runs the HTTP API described in docs/api.md.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	aiapi "github.com/lakhans7/eventbasedhttp/internal/ai"
	"github.com/lakhans7/eventbasedhttp/internal/analytics"
	"github.com/lakhans7/eventbasedhttp/internal/api"
	"github.com/lakhans7/eventbasedhttp/internal/audit"
	authapi "github.com/lakhans7/eventbasedhttp/internal/auth"
	"github.com/lakhans7/eventbasedhttp/internal/config"
	"github.com/lakhans7/eventbasedhttp/internal/db"
	"github.com/lakhans7/eventbasedhttp/internal/jobs"
	"github.com/lakhans7/eventbasedhttp/internal/logger"
	"github.com/lakhans7/eventbasedhttp/internal/mailer"
	notifyapi "github.com/lakhans7/eventbasedhttp/internal/notification"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

func main() {
	_ = godotenv.Load() // no-op if .env doesn't exist (e.g. in production, where env vars are injected)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	logger.Init(cfg.Env)
	l := logger.Get()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		cancel()
		l.Fatal().Err(err).Msg("failed to connect to database")
	}
	if err := db.Migrate(ctx, pool, "./migrations"); err != nil {
		cancel()
		l.Fatal().Err(err).Msg("failed to run database migrations")
	}
	cancel()
	defer pool.Close()

	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		l.Fatal().Err(err).Msg("invalid REDIS_URL")
	}
	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	asynqRedisOpt, err := jobs.RedisClientOpt(cfg.RedisURL)
	if err != nil {
		l.Fatal().Err(err).Msg("invalid REDIS_URL")
	}
	asynqClient := asynq.NewClient(asynqRedisOpt)
	defer asynqClient.Close()

	st := store.New(pool)
	auditSvc := audit.NewService(pool)
	mail := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)

	jwtIssuer := authapi.NewJWTIssuer(cfg.JWTSecret, cfg.AccessTokenTTL)
	authRepo := authapi.NewRepository(pool)
	googleOAuth := authapi.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	authSvc := authapi.NewService(authRepo, jwtIssuer, mail, auditSvc, cfg.FrontendOrigin, cfg.RefreshTokenTTL)
	authHandlers := authapi.NewHandlers(authSvc, googleOAuth, auditSvc, cfg.CookieDomain, cfg.CookieSecure, cfg.RefreshTokenTTL)

	fiverrHandlers := api.NewFiverrHandlers(st, auditSvc, asynqClient)

	var llm aiapi.LLMClient = aiapi.MockClient{}
	if cfg.AnthropicAPIKey != "" {
		llm = aiapi.NewAnthropicClient(cfg.AnthropicAPIKey, cfg.AIModel)
	} else {
		l.Warn().Msg("ANTHROPIC_API_KEY not set — AI Assistant will return mock responses only")
	}
	// Placeholder per-token pricing until configured to match your actual Anthropic plan (docs/security.md cost control).
	pricing := aiapi.Pricing{InputPer1KUSD: 0.003, OutputPer1KUSD: 0.015}
	aiSvc := aiapi.NewService(llm, st, cfg.AIMaxOutputTokens, cfg.AIDailyUserBudget, pricing)
	aiHandlers := aiapi.NewHandlers(aiSvc, st, auditSvc)

	notifySvc := notifyapi.NewService(st, asynqClient, func(ctx context.Context, userID string) (string, error) {
		u, err := authRepo.GetUserByID(ctx, userID)
		if err != nil {
			return "", err
		}
		return u.Email, nil
	})
	notifyHandlers := notifyapi.NewHandlers(st)

	analyticsSvc := analytics.NewService(pool)
	analyticsHandlers := analytics.NewHandlers(analyticsSvc)

	auditHandlers := audit.NewHandlers(auditSvc)
	resourceHandlers := api.NewResourceHandlers(st)
	metrics := api.NewMetrics()

	_ = notifySvc // wired for use by background jobs / future event-driven notifications

	app := api.NewApp(api.Deps{
		Config:            cfg,
		Pool:              pool,
		Redis:             redisClient,
		JWTIssuer:         jwtIssuer,
		AuthHandlers:      authHandlers,
		AIHandlers:        aiHandlers,
		FiverrHandlers:    fiverrHandlers,
		NotifyHandlers:    notifyHandlers,
		AnalyticsHandlers: analyticsHandlers,
		AuditHandlers:     auditHandlers,
		Resources:         resourceHandlers,
		Metrics:           metrics,
	})

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			l.Fatal().Err(err).Msg("server stopped")
		}
	}()
	l.Info().Str("port", cfg.Port).Str("env", cfg.Env).Msg("server started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	l.Info().Msg("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		l.Error().Err(err).Msg("error during shutdown")
	}
}
