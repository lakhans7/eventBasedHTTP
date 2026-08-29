// Package config loads the shop's configuration from environment variables.
// See .env.example for the full list.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port string
	Env  string

	StoreName      string
	ProductName    string
	PriceCents     int64 // fixed price regardless of size — the server, never the client, decides the amount charged
	Currency       string
	FrontendOrigin string

	RazorpayKeyID         string
	RazorpayKeySecret     string
	RazorpayWebhookSecret string // optional; enables /api/webhook/razorpay

	// OrdersLogPath is a local backup of orders (see internal/orders) — the
	// Razorpay dashboard is the real source of truth (every order's buyer
	// details are attached to its Razorpay Order's `notes`). Point this at a
	// mounted volume in production or the file is lost on every redeploy —
	// see README.md "Persistence".
	OrdersLogPath string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                  getenv("PORT", "3000"),
		Env:                   getenv("APP_ENV", "development"),
		StoreName:             getenv("STORE_NAME", "Your Store Name"),
		ProductName:           getenv("PRODUCT_NAME", "Red Lace Nightdress"),
		Currency:              getenv("CURRENCY", "INR"),
		FrontendOrigin:        getenv("FRONTEND_ORIGIN", "http://localhost:3000"),
		RazorpayKeyID:         getenv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:     getenv("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret: getenv("RAZORPAY_WEBHOOK_SECRET", ""),
		OrdersLogPath:         getenv("ORDERS_LOG_PATH", "orders.jsonl"),
	}
	cfg.PriceCents = 79900 // ₹799, fixed — see docs in README on why this isn't an env var

	if cfg.Env == "production" {
		if cfg.RazorpayKeyID == "" || cfg.RazorpayKeySecret == "" {
			return nil, fmt.Errorf("RAZORPAY_KEY_ID and RAZORPAY_KEY_SECRET must be set in production")
		}
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
