# Architecture

## Read this first

Per `docs/fiverr-api-capabilities.md`, Fiverr has no public API. Every diagram below shows the **real** data flow: a seller-initiated manual/CSV import, not a live Fiverr connection. The abstractions are built so a future official Fiverr API only requires a new `MarketplaceProvider` implementation.

## Layers

```
                         ┌────────────────────────┐
                         │        Frontend         │   web/ (static HTML/CSS/JS)
                         │  Dashboard / Inbox / …  │
                         └───────────┬─────────────┘
                                     │ HTTPS (fetch → JSON)
                         ┌───────────▼─────────────┐
                         │      API (Fiber)         │   internal/api
                         │ auth / rate-limit / CORS │
                         │      / audit middleware  │
                         └───────────┬─────────────┘
              ┌──────────────┬───────┼───────────────┬───────────────┐
              ▼              ▼       ▼               ▼               ▼
        ┌──────────┐  ┌───────────┐ ┌──────────┐ ┌──────────┐ ┌────────────┐
        │   Auth   │  │Marketplace│ │    AI    │ │Notificat.│ │  Analytics │
        │ Layer    │  │ Layer     │ │  Layer   │ │  Layer   │ │   Layer    │
        └────┬─────┘  └─────┬─────┘ └────┬─────┘ └────┬─────┘ └─────┬──────┘
             │              │            │            │             │
             └──────────────┴─────┬──────┴────────────┴─────────────┘
                                   ▼
                         ┌───────────────────┐
                         │     PostgreSQL     │   internal/db + migrations/
                         └───────────────────┘
                                   ▲
                                   │ enqueue / dequeue
                         ┌───────────────────┐        ┌─────────────┐
                         │   Redis (asynq)    │◄──────►│   Worker    │  cmd/worker
                         └───────────────────┘        └─────────────┘
```

### A. Fiverr Integration Layer (`internal/marketplace`)

- `MarketplaceProvider` interface: `GetAccount`, `GetGigs`, `GetOrders`, `GetConversations`, `GetMessages`, `GetReviews`, plus a `Capabilities()` method every caller must check before invoking a method.
- `fiverr.Provider` is the only implementation today. Every read method returns `marketplace.ErrNotSupportedByProvider` because there is no live Fiverr API (see capability doc). The only real functionality it exposes is `ImportCSV` / `ImportManual`, which parses seller-uploaded exports into the normalized domain models via `fiverr.Normalizer`.
- Future `upwork.Provider`, `freelancer.Provider` can implement the same interface without touching any other layer.

### B. Application Backend (`internal/api`, `cmd/server`)
REST API over Fiber. Stateless; all state lives in Postgres/Redis so it can be horizontally scaled.

### C. AI Layer (`internal/ai`)
`LLMClient` interface, with `AnthropicClient` (calls the Messages API directly over HTTPS — no unofficial SDKs) and `MockClient` (deterministic, used in tests and when `ANTHROPIC_API_KEY` is unset so the app still runs). All generation goes through the safety pipeline described in `docs/security.md` before reaching a human for approval. Nothing in this layer can send a message to Fiverr — there is no such API.

### D. Database (`internal/db`, `migrations/`)
PostgreSQL via `pgx`. Plain SQL migrations applied with `golang-migrate`. Schema in `docs/database.md`.

### E. Notification System (`internal/notification`)
`Notifier` interface with `InAppNotifier` (writes to `notifications` table, read by the frontend) and `EmailNotifier` (SMTP; a `NoopEmailNotifier`/console logger is used when SMTP env vars are absent so local dev doesn't require a mail server).

### F. Frontend (`web/`)
Static HTML/CSS/vanilla JS, served by Fiber's static middleware. No build step, deliberately — this is an MVP UI (see `README.md` limitations), not a production design-system build. It never sees a Fiverr token because none exists; it only ever talks to our own `/api/v1/*`.

### G. Background Jobs (`internal/jobs`, `cmd/worker`)
`asynq` (Redis-backed) task queue. Tasks: `send_email`, `process_import` (CSV parsing off the request path), `refresh_analytics`, `dispatch_notification`, `cleanup_expired_tokens`, `refresh_fiverr_token` (no-op today — kept for forward compatibility, logs a warning that no provider supports refreshable tokens yet). All handlers are idempotent (keyed by a stable task ID) and retried with `asynq`'s exponential backoff.

### H. Analytics (`internal/analytics`)
Pure SQL aggregation over the domain tables (revenue, order counts, ratings, response time) computed from whatever data has actually been imported. Every metric the UI shows is labeled with its data source and "as of" timestamp so a partially-imported account never shows a silently wrong number.

### I. Security/Audit (`internal/audit`, `internal/api/middleware`)
Every mutating request and every AI generation/approval passes through `audit.Service.Log`, writing to `audit_logs`. See `docs/security.md` for the full threat model.

## Repository structure

```
cmd/
  server/main.go        # API process
  worker/main.go        # asynq worker process
internal/
  ai/                    # LLM client, safety pipeline, handlers
  analytics/             # aggregation queries
  api/                   # router + middleware
  audit/                 # audit log service
  auth/                  # password, JWT, Google OAuth, handlers
  config/                # env-based config loader
  db/                    # pgx pool + migration runner
  domain/                # normalized internal models (Gig, Order, Message, ...)
  jobs/                  # asynq task definitions + handlers
  logger/                # zerolog setup
  marketplace/           # MarketplaceProvider interface
    fiverr/              # FiverrProvider (manual/CSV import) + normalizer
  notification/          # Notifier interface + implementations
migrations/              # SQL migrations (golang-migrate format)
web/                     # static frontend
docs/                    # this documentation set
```

## Why manual import instead of a fake sync engine

Section 6 of the original brief describes a live "Fiverr → Fiverr API → Integration Service → Normalizer → Database" pipeline. That pipeline is real and fully implemented **except the leftmost box**: there is no Fiverr API to poll. The `Normalizer` and everything downstream of it work identically whether the input came from a live API or a CSV upload, which is exactly the point of the abstraction — the moment Fiverr publishes an API, only `fiverr.Provider`'s data-source changes.

## Multi-marketplace extensibility

Because nothing outside `internal/marketplace/fiverr` knows about Fiverr specifically, adding Upwork (which *does* have a public API) later is additive: implement `marketplace.Provider`, register it, and the existing dashboard/AI/analytics code works unmodified against the same `domain.Order`, `domain.Message`, etc.
