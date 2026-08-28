package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	aiapi "github.com/lakhans7/eventbasedhttp/internal/ai"
	"github.com/lakhans7/eventbasedhttp/internal/analytics"
	"github.com/lakhans7/eventbasedhttp/internal/audit"
	authapi "github.com/lakhans7/eventbasedhttp/internal/auth"
	"github.com/lakhans7/eventbasedhttp/internal/config"
	notifyapi "github.com/lakhans7/eventbasedhttp/internal/notification"
)

type Deps struct {
	Config            *config.Config
	Pool              *pgxpool.Pool
	Redis             *redis.Client
	JWTIssuer         *authapi.JWTIssuer
	AuthHandlers      *authapi.Handlers
	AIHandlers        *aiapi.Handlers
	FiverrHandlers    *FiverrHandlers
	NotifyHandlers    *notifyapi.Handlers
	AnalyticsHandlers *analytics.Handlers
	AuditHandlers     *audit.Handlers
	Resources         *ResourceHandlers
	Metrics           *Metrics
}

func NewApp(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "fiverr-seller-platform",
		ErrorHandler: errorHandler,
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(RequestLogger())
	app.Use(d.Metrics.Middleware())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     d.Config.FrontendOrigin,
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-CSRF-Token",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	app.Get("/ready", func(c *fiber.Ctx) error { return readyHandler(c, d) })
	app.Get("/metrics", d.Metrics.Handler())

	app.Static("/", "./web")

	v1 := app.Group("/api/v1")

	// Tighter rate limits on auth and AI endpoints, per docs/security.md.
	authLimiter := limiter.New(limiter.Config{Max: 20, Expiration: time.Minute})
	aiLimiter := limiter.New(limiter.Config{Max: 30, Expiration: time.Minute})
	defaultLimiter := limiter.New(limiter.Config{Max: 300, Expiration: time.Minute})
	v1.Use(defaultLimiter)

	authGroup := v1.Group("/auth", authLimiter)
	authGroup.Post("/register", d.AuthHandlers.Register)
	authGroup.Post("/login", d.AuthHandlers.Login)
	authGroup.Post("/refresh", d.AuthHandlers.Refresh)
	authGroup.Post("/verify-email", d.AuthHandlers.VerifyEmail)
	authGroup.Post("/forgot-password", d.AuthHandlers.ForgotPassword)
	authGroup.Post("/reset-password", d.AuthHandlers.ResetPassword)
	authGroup.Get("/google/start", d.AuthHandlers.GoogleStart)
	authGroup.Get("/google/callback", d.AuthHandlers.GoogleCallback)

	requireAuth := authapi.RequireAuth(d.JWTIssuer)

	authGroup.Post("/logout", requireAuth, d.AuthHandlers.Logout)
	authGroup.Post("/logout-all", requireAuth, d.AuthHandlers.LogoutAll)
	authGroup.Delete("/account", requireAuth, d.AuthHandlers.DeleteAccount)
	authGroup.Get("/me", requireAuth, d.AuthHandlers.Me)

	fv := v1.Group("/fiverr", requireAuth)
	fv.Get("/accounts", d.FiverrHandlers.ListAccounts)
	fv.Post("/accounts", d.FiverrHandlers.CreateAccount)
	fv.Delete("/accounts/:id", d.FiverrHandlers.DeleteAccount)
	fv.Get("/accounts/:id/health", d.FiverrHandlers.Health)
	fv.Post("/accounts/:id/import", d.FiverrHandlers.Import)
	fv.Post("/accounts/:id/messages", d.FiverrHandlers.PostMessage)
	fv.Get("/accounts/:id/sync-jobs", d.FiverrHandlers.ListSyncJobs)

	gigs := v1.Group("/gigs", requireAuth)
	gigs.Get("/", d.Resources.ListGigs)
	gigs.Post("/", d.Resources.CreateGig)
	gigs.Get("/:id", d.Resources.GetGig)
	gigs.Patch("/:id", d.Resources.PatchGig)

	orders := v1.Group("/orders", requireAuth)
	orders.Get("/", d.Resources.ListOrders)
	orders.Get("/:id", d.Resources.GetOrder)
	orders.Patch("/:id", d.Resources.PatchOrder)
	orders.Post("/:id/requirements", d.Resources.CreateOrderRequirement)

	v1.Get("/customers", requireAuth, d.Resources.ListCustomers)
	v1.Get("/reviews", requireAuth, d.Resources.ListReviews)

	conversations := v1.Group("/conversations", requireAuth)
	conversations.Get("/", d.Resources.ListConversations)
	conversations.Get("/:id", d.Resources.GetConversation)
	conversations.Get("/:id/messages", d.Resources.ListMessages)
	conversations.Post("/:id/read", d.Resources.MarkConversationRead)

	ai := v1.Group("/ai", requireAuth, aiLimiter)
	ai.Post("/generate-response", d.AIHandlers.GenerateResponse)
	ai.Post("/summarize-order", d.AIHandlers.SummarizeOrder)
	ai.Post("/extract-requirements", d.AIHandlers.ExtractRequirements)
	ai.Post("/delivery-message", d.AIHandlers.DeliveryMessage)
	ai.Post("/analyze-review", d.AIHandlers.AnalyzeReview)
	ai.Post("/chat", d.AIHandlers.Chat)
	ai.Get("/generations/:id", d.AIHandlers.GetGeneration)
	ai.Patch("/generations/:id", d.AIHandlers.PatchGeneration)
	ai.Post("/generations/:id/approve", d.AIHandlers.Approve)
	ai.Post("/generations/:id/reject", d.AIHandlers.Reject)
	ai.Post("/generations/:id/mark-sent", d.AIHandlers.MarkSent)
	ai.Post("/generations/:id/feedback", d.AIHandlers.Feedback)

	analyticsGroup := v1.Group("/analytics", requireAuth)
	analyticsGroup.Get("/overview", d.AnalyticsHandlers.Overview)
	analyticsGroup.Get("/revenue-over-time", d.AnalyticsHandlers.RevenueOverTime)
	analyticsGroup.Get("/orders-over-time", d.AnalyticsHandlers.OrdersOverTime)

	notifications := v1.Group("/notifications", requireAuth)
	notifications.Get("/", d.NotifyHandlers.List)
	notifications.Post("/:id/read", d.NotifyHandlers.MarkRead)

	v1.Get("/audit-logs", requireAuth, d.AuditHandlers.List)

	me := v1.Group("/me", requireAuth)
	me.Get("/preferences", d.Resources.GetPreferences)
	me.Put("/preferences", d.Resources.PutPreferences)

	return app
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if fe, ok := err.(*fiber.Error); ok {
		code = fe.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": fiber.Map{"code": "request_error", "message": err.Error()}})
}

func readyHandler(c *fiber.Ctx, d Deps) error {
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	if err := d.Pool.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not_ready", "reason": "database unreachable"})
	}
	if err := d.Redis.Ping(ctx).Err(); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not_ready", "reason": "redis unreachable"})
	}
	return c.JSON(fiber.Map{"status": "ready"})
}
