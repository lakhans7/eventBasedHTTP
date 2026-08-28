package jobs

import (
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// RedisClientOpt builds an asynq.RedisClientOpt from a standard redis:// or
// rediss:// (TLS) URL — including embedded username/password — so the same
// REDIS_URL works against a bare local Redis (docker-compose) and an
// authenticated, TLS-only managed Redis (Upstash, Redis Cloud, Fly Redis,
// ...) in production without any code change, only a config change.
func RedisClientOpt(redisURL string) (asynq.RedisClientOpt, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return asynq.RedisClientOpt{}, err
	}
	return asynq.RedisClientOpt{
		Addr:      opt.Addr,
		Username:  opt.Username,
		Password:  opt.Password,
		DB:        opt.DB,
		TLSConfig: opt.TLSConfig,
	}, nil
}
