// Package config loads application configuration from environment variables.
// See .env.example for the full list of variables and their defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env  string
	Port string

	DatabaseURL string
	// RedisURL accepts a standard redis:// or rediss:// (TLS) URL, with
	// optional username/password — so the same config works against a bare
	// local Redis (docker-compose) and an authenticated, TLS-only managed
	// Redis (Upstash, Redis Cloud, Fly Redis, ...) in production. See
	// internal/jobs.RedisClientOpt for how it's parsed.
	RedisURL string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CookieDomain    string
	CookieSecure    bool
	FrontendOrigin  string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	FiverrTokenEncKey string // 32-byte key, base64 or raw; used only if a real OAuth integration is ever added.

	AnthropicAPIKey   string
	AIModel           string
	AIMaxOutputTokens int
	AIDailyUserBudget float64 // USD, soft cap enforced by internal/ai/usage.go

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:                getenv("APP_ENV", "development"),
		Port:               getenv("PORT", "3000"),
		DatabaseURL:        getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/fiverr_saas?sslmode=disable"),
		RedisURL:           getenv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:          getenv("JWT_SECRET", ""),
		CookieDomain:       getenv("COOKIE_DOMAIN", "localhost"),
		FrontendOrigin:     getenv("FRONTEND_ORIGIN", "http://localhost:3000"),
		GoogleClientID:     getenv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getenv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getenv("GOOGLE_REDIRECT_URL", "http://localhost:3000/api/v1/auth/google/callback"),
		FiverrTokenEncKey:  getenv("FIVERR_TOKEN_ENC_KEY", ""),
		AnthropicAPIKey:    getenv("ANTHROPIC_API_KEY", ""),
		AIModel:            getenv("AI_MODEL", "claude-sonnet-5"),
		SMTPHost:           getenv("SMTP_HOST", ""),
		SMTPPort:           getenv("SMTP_PORT", "587"),
		SMTPUser:           getenv("SMTP_USER", ""),
		SMTPPass:           getenv("SMTP_PASS", ""),
		SMTPFrom:           getenv("SMTP_FROM", "no-reply@example.com"),
	}

	cfg.CookieSecure = getenvBool("COOKIE_SECURE", cfg.Env == "production")

	accessTTL, err := time.ParseDuration(getenv("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
	}
	cfg.AccessTokenTTL = accessTTL

	refreshTTL, err := time.ParseDuration(getenv("REFRESH_TOKEN_TTL", "720h"))
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TOKEN_TTL: %w", err)
	}
	cfg.RefreshTokenTTL = refreshTTL

	maxTokens, err := strconv.Atoi(getenv("AI_MAX_OUTPUT_TOKENS", "1024"))
	if err != nil {
		return nil, fmt.Errorf("invalid AI_MAX_OUTPUT_TOKENS: %w", err)
	}
	cfg.AIMaxOutputTokens = maxTokens

	budget, err := strconv.ParseFloat(getenv("AI_DAILY_USER_BUDGET_USD", "2.00"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid AI_DAILY_USER_BUDGET_USD: %w", err)
	}
	cfg.AIDailyUserBudget = budget

	if cfg.Env == "production" && cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET must be set in production")
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "dev-only-insecure-secret-change-me"
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
