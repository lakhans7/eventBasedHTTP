package jobs

import "testing"

func TestRedisClientOptParsesPlainURL(t *testing.T) {
	opt, err := RedisClientOpt("redis://localhost:6379")
	if err != nil {
		t.Fatalf("RedisClientOpt: %v", err)
	}
	if opt.Addr != "localhost:6379" {
		t.Errorf("expected addr localhost:6379, got %q", opt.Addr)
	}
	if opt.Password != "" || opt.TLSConfig != nil {
		t.Errorf("plain redis:// URL should have no password/TLS, got password=%q tls=%v", opt.Password, opt.TLSConfig)
	}
}

// TestRedisClientOptParsesTLSURLWithAuth guards a real deployment need: managed
// Redis providers (Upstash, Redis Cloud, Fly Redis) hand out rediss:// URLs
// with an embedded username/password, and both the asynq client/server and
// the plain go-redis health-check client must authenticate over TLS with no
// code change — only the REDIS_URL value changes (docs/deployment.md).
func TestRedisClientOptParsesTLSURLWithAuth(t *testing.T) {
	opt, err := RedisClientOpt("rediss://default:s3cret@my-redis.upstash.io:6380")
	if err != nil {
		t.Fatalf("RedisClientOpt: %v", err)
	}
	if opt.Addr != "my-redis.upstash.io:6380" {
		t.Errorf("expected addr my-redis.upstash.io:6380, got %q", opt.Addr)
	}
	if opt.Username != "default" || opt.Password != "s3cret" {
		t.Errorf("expected username=default password=s3cret, got username=%q password=%q", opt.Username, opt.Password)
	}
	if opt.TLSConfig == nil {
		t.Error("expected a non-nil TLSConfig for a rediss:// URL")
	}
}

func TestRedisClientOptRejectsInvalidURL(t *testing.T) {
	if _, err := RedisClientOpt("not-a-url::garbage"); err == nil {
		t.Fatal("expected an error for a malformed REDIS_URL")
	}
}
