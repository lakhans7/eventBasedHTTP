// Command server runs the single-page storefront + its Razorpay integration.
package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"github.com/lakhans7/eventbasedhttp/internal/config"
	"github.com/lakhans7/eventbasedhttp/internal/orders"
	"github.com/lakhans7/eventbasedhttp/internal/razorpay"
	"github.com/lakhans7/eventbasedhttp/internal/shop"
)

func main() {
	_ = godotenv.Load() // no-op if .env doesn't exist (e.g. in production, where env vars are injected)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	rzpClient := razorpay.NewClient(cfg.RazorpayKeyID, cfg.RazorpayKeySecret)
	store := orders.NewStore(cfg.OrdersLogPath)
	handlers := shop.NewHandlers(cfg, rzpClient, store)

	app := fiber.New(fiber.Config{AppName: "single-product-shop"})
	app.Use(recover.New())

	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })

	api := app.Group("/api", limiter.New(limiter.Config{Max: 60, Expiration: 60}))
	api.Get("/product", handlers.GetProduct)
	api.Post("/order", handlers.PostOrder)
	api.Post("/verify", handlers.PostVerify)
	api.Post("/webhook/razorpay", handlers.PostWebhook)

	app.Static("/", "./web")

	log.Printf("shop server listening on :%s (env=%s)", cfg.Port, cfg.Env)
	log.Fatal(app.Listen(":" + cfg.Port))
}
