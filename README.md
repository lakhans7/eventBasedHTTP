# Fiverr Seller Hub

A SaaS platform that helps a Fiverr seller manage and get AI assistance with their business —
orders, gigs, buyer conversations, reviews, and analytics — from one dashboard.

**Read this first:** Fiverr does not publish a public API or OAuth program for third-party
apps to connect a seller's account. See [`docs/fiverr-api-capabilities.md`](docs/fiverr-api-capabilities.md)
for the research behind that conclusion and exactly what this changes about the design. In short:
there is no "Connect Fiverr" OAuth button here, because Fiverr doesn't offer one. Instead, sellers
label a Fiverr profile and bring their own data in via CSV import (exported from their own Fiverr
dashboard) or by pasting messages/requirements manually. Everything downstream — the dashboard, AI
assistant, analytics, notifications — works the same regardless of how the data got in, so a future
official Fiverr API (or another marketplace's API) can be added later with no application-level
rewrite (see `internal/marketplace`).

## Documentation

| Doc | What's in it |
|---|---|
| [`docs/fiverr-api-capabilities.md`](docs/fiverr-api-capabilities.md) | Fiverr API research and capability matrix — read this first |
| [`docs/architecture.md`](docs/architecture.md) | System architecture, layers, repository structure |
| [`docs/database.md`](docs/database.md) | Full schema and data dictionary |
| [`docs/api.md`](docs/api.md) | REST API specification |
| [`docs/security.md`](docs/security.md) | Threat model, AI safety pipeline, prompt-injection defenses |

## Stack

- **Backend**: Go + [Fiber](https://gofiber.io) (kept from this repo's original stack)
- **Database**: PostgreSQL via `pgx`, plain SQL migrations (no ORM, no migration framework — see `internal/db`)
- **Queue**: Redis + [asynq](https://github.com/hibiken/asynq) for background jobs (CSV import processing, email)
- **AI**: direct HTTPS calls to the Anthropic Messages API (`internal/ai`), with a mock client so the app runs without an API key
- **Frontend**: plain HTML/CSS/vanilla JS in `web/`, no build step, served as static files by the API server
- **Auth**: email/password (bcrypt) + Google OAuth (PKCE), JWT access tokens + rotating opaque refresh tokens

## Repository structure

```
cmd/server/      API process (HTTP)
cmd/worker/      asynq background job processor
internal/        see docs/architecture.md for the full layer breakdown
migrations/      SQL migrations, applied automatically on server startup
web/             static frontend (no build step)
docs/            architecture, database, API, and security documentation
```

## Local development

### Prerequisites
- Go 1.24+
- Docker (for Postgres/Redis via `docker-compose`) — or your own local Postgres 16+ / Redis 7+

### Quick start with Docker

```bash
cp .env.example .env
docker compose up --build
```

This starts Postgres, Redis, the API server (`:3000`), and the background worker. Migrations run
automatically when the server starts. Open http://localhost:3000 — it redirects to the login page.

### Running without Docker

```bash
cp .env.example .env   # edit DATABASE_URL / REDIS_ADDR if not using the defaults
go run ./cmd/server     # API on :3000, runs migrations on startup
go run ./cmd/worker     # in a second terminal — required for CSV imports to complete
```

Without `ANTHROPIC_API_KEY` set, the AI Assistant still works end-to-end but returns a clearly
labeled mock response instead of a real model completion — useful for developing/testing the rest
of the app without an API key. Without `SMTP_HOST` set, outbound email (verification, password
reset) is logged to the console instead of sent.

### Environment variables

See [`.env.example`](.env.example) for the full list with defaults and comments. Never commit `.env`.

## Testing

```bash
go vet ./...
go test ./...
```

Pure unit tests (password hashing, JWT, CSRF, the AI prompt-injection corpus, CSV field parsing,
token encryption) run with no external dependencies. Database-backed tests (in `internal/store` and
`internal/marketplace/fiverr`) connect to `$DATABASE_URL` (default
`postgres://postgres:postgres@localhost:5432/fiverr_saas_test?sslmode=disable`) and automatically
`t.Skip` if no database is reachable, so `go test ./...` is safe to run with nothing else set up —
CI (`.github/workflows/ci.yml`) provides a real Postgres/Redis and runs everything, including the
database-backed tests.

## Known limitations (see docs for full detail)

- **No live Fiverr sync.** Every number in the dashboard reflects the last CSV import or manual
  entry, not Fiverr in real time. This is a Fiverr platform limitation, not a bug — see
  `docs/fiverr-api-capabilities.md`.
- **No programmatic message sending.** The AI Assistant only ever produces a draft; the seller
  copies it into Fiverr's own inbox and confirms via "mark as sent" for record-keeping.
- **Gig edits are local only.** There is no Fiverr gig-write API, so editing a gig here never
  changes the live Fiverr listing.
- **AI cost estimates use placeholder per-token pricing** (`cmd/server/main.go`) until you update it
  to match your actual Anthropic plan.
- The frontend is a functional MVP (plain HTML/CSS/JS), not a polished, fully responsive design-system
  build — see product section 20 in the original brief for the long-term bar.

## Roadmap

- Phase 2: richer analytics, in-app + email notification triggers wired to more events, seller AI
  knowledge base UI polish, order-timeline automation.
- Phase 3: if/when Fiverr (or another marketplace, e.g. Upwork) ships a public API, implement a new
  `marketplace.Provider` and enable live sync + programmatic sending behind explicit, auditable
  opt-in — the rest of the application does not need to change.
