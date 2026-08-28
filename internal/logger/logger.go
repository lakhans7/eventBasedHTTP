// Package logger provides a process-wide structured (zerolog) logger.
package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

// secretFieldNames is a denylist of field names that must never reach a log line,
// even if a caller accidentally attaches one to an event (see docs/security.md).
var secretFieldNames = map[string]struct{}{
	"password":                {},
	"password_hash":           {},
	"access_token":            {},
	"refresh_token":           {},
	"access_token_encrypted":  {},
	"refresh_token_encrypted": {},
	"token":                   {},
	"secret":                  {},
	"authorization":           {},
}

// Init configures the global logger. env is typically "development" or "production";
// development gets a human-readable console writer, production emits JSON.
func Init(env string) {
	zerolog.TimeFieldFormat = time.RFC3339

	var w = os.Stdout
	if strings.EqualFold(env, "development") {
		log = zerolog.New(zerolog.ConsoleWriter{Out: w, TimeFormat: time.RFC3339}).
			With().Timestamp().Logger()
		return
	}
	log = zerolog.New(w).With().Timestamp().Logger()
}

// Get returns the global logger. Init must be called first (main() does this at startup).
func Get() *zerolog.Logger {
	return &log
}

// IsSecretField reports whether a field name must be redacted before logging or auditing.
func IsSecretField(name string) bool {
	_, ok := secretFieldNames[strings.ToLower(name)]
	return ok
}
