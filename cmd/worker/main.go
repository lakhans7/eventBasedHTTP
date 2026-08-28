// Command worker runs the asynq background job processor (docs/architecture.md section G).
package main

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"

	"github.com/lakhans7/eventbasedhttp/internal/config"
	"github.com/lakhans7/eventbasedhttp/internal/db"
	"github.com/lakhans7/eventbasedhttp/internal/jobs"
	"github.com/lakhans7/eventbasedhttp/internal/logger"
	"github.com/lakhans7/eventbasedhttp/internal/mailer"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger.Init(cfg.Env)
	l := logger.Get()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		l.Fatal().Err(err).Msg("worker: failed to connect to database")
	}
	defer pool.Close()

	st := store.New(pool)
	mail := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)

	redisOpt, err := jobs.RedisClientOpt(cfg.RedisURL)
	if err != nil {
		l.Fatal().Err(err).Msg("worker: invalid REDIS_URL")
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			// Fiverr has no API to hammer, but downstream services (SMTP, Anthropic)
			// still benefit from asynq's default exponential backoff on retry.
			Queues: map[string]int{"default": 1},
		},
	)

	mux := asynq.NewServeMux()
	jobs.RegisterHandlers(mux, jobs.Deps{Store: st, Mailer: mail})

	l.Info().Msg("worker started")
	if err := srv.Run(mux); err != nil {
		l.Fatal().Err(err).Msg("worker: server stopped")
	}
}
