# API Specification

Base path: `/api/v1`. JSON in/out. Auth via `Authorization: Bearer <JWT>` (access token, short-lived) obtained from `/api/v1/auth/login`; refresh via httpOnly secure cookie holding the opaque refresh token (never exposed to JS). All mutating endpoints require CSRF header `X-CSRF-Token` matching the value bound to the session when the request also carries the refresh cookie.

Errors are structured:
```json
{ "error": { "code": "invalid_credentials", "message": "Email or password is incorrect." } }
```

## Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | /auth/register | none | Create account (email/password). Sends verification email. |
| POST | /auth/login | none | Returns access token + sets refresh cookie. |
| POST | /auth/logout | session | Revokes current session. |
| POST | /auth/logout-all | session | Revokes all sessions for the user. |
| POST | /auth/refresh | refresh cookie | Rotates refresh token, issues new access token. |
| POST | /auth/verify-email | none | Body: `{token}`. |
| POST | /auth/forgot-password | none | Body: `{email}`. Always 202, never leaks whether the email exists. |
| POST | /auth/reset-password | none | Body: `{token, new_password}`. |
| GET | /auth/google/start | none | Redirects to Google's OAuth consent screen. |
| GET | /auth/google/callback | none | Validates `state`, exchanges code, creates/links user. |
| DELETE | /auth/account | session | Soft-deletes the account (30-day grace period job purges it). |

## Fiverr accounts (manual connection — see capability doc)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | /fiverr/accounts | session | List the user's `FiverrAccount` records. |
| POST | /fiverr/accounts | session | Body: `{username}`. Creates a manual `FiverrAccount` (`connection_method=manual`). No OAuth redirect exists. |
| DELETE | /fiverr/accounts/:id | session | Disconnects (soft) an account. |
| GET | /fiverr/accounts/:id/health | session | Returns connection status + `last_sync_at`; always `connection_method: manual` today. |
| POST | /fiverr/accounts/:id/import | session | `multipart/form-data` CSV upload. `type=gigs\|orders\|reviews`. Enqueues `process_import` job, returns `sync_job_id`. |
| POST | /fiverr/accounts/:id/messages | session | Body: `{customer_username, gig_id?, direction, body, sent_at}` — manual paste of a buyer/seller message into a conversation. |
| GET | /fiverr/accounts/:id/sync-jobs | session | List recent `sync_jobs` with status, for the "connection health" UI. |

## Gigs / Orders / Customers / Conversations (read + limited manual write)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | /gigs | session | Query: `fiverr_account_id`. |
| GET | /gigs/:id | session | |
| PATCH | /gigs/:id | session | Edits our local copy only — never writes to Fiverr (no write API exists). |
| GET | /orders | session | Query: `status`, `fiverr_account_id`, pagination. |
| GET | /orders/:id | session | Includes requirements, timeline stage, linked conversation. |
| PATCH | /orders/:id | session | Updates local `stage`/`status`/`due_at` — bookkeeping only. |
| GET | /customers | session | |
| GET | /conversations | session | |
| GET | /conversations/:id/messages | session | Paginated. |

## AI Assistant

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | /ai/generate-response | session | Body: `{conversation_id, instruction?}`. Runs full safety pipeline, stores `ai_generations` row with `status=pending_review`, returns the draft + risk flags. Never sends anything. |
| POST | /ai/summarize-order | session | Body: `{order_id}`. |
| POST | /ai/extract-requirements | session | Body: `{order_requirement_id}` or raw `{text}`. |
| POST | /ai/delivery-message | session | Body: `{order_id}`. |
| POST | /ai/analyze-review | session | Body: `{review_id}`. |
| POST | /ai/chat | session | Freeform AI workspace (section 21): `{context: {conversation_id?|order_id?|gig_id?}, question}`. |
| GET | /ai/generations/:id | session | |
| PATCH | /ai/generations/:id | session | Body: `{edited_output?}`. Seller edits before approving. |
| POST | /ai/generations/:id/approve | session | Marks `approved`. If the account has no send capability (always true today), the response instructs the seller to copy the text into Fiverr and then call `/mark-sent`. |
| POST | /ai/generations/:id/mark-sent | session | Seller confirms they manually sent it in Fiverr's own UI; creates the corresponding `messages` row (`direction=outbound`, `source=manual_paste`) for record-keeping. |
| POST | /ai/generations/:id/reject | session | |
| POST | /ai/generations/:id/feedback | session | Body: `{rating, comment?}`. |

## Analytics / Notifications / Audit

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | /analytics/overview | session | Revenue, orders, ratings, response time — computed only from imported data; response includes `data_completeness` per metric. |
| GET | /analytics/revenue-over-time | session | |
| GET | /analytics/orders-over-time | session | |
| GET | /notifications | session | |
| POST | /notifications/:id/read | session | |
| GET | /audit-logs | session (self only) | The user's own audit trail. |

## Seller preferences (AI knowledge base)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | /me/preferences | session | |
| PUT | /me/preferences | session | Full replace of the seller AI knowledge base (skills, tone, min project, FAQs, terms). |

## Ops

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | /health | none | Liveness — process is up. |
| GET | /ready | none | Readiness — DB + Redis reachable. |
| GET | /metrics | internal network only | Prometheus exposition format. |

## Versioning & validation

- Path-based versioning (`/api/v1`); breaking changes ship as `/api/v2` alongside v1 until clients migrate.
- Every handler validates its body against a struct with `validate` tags before touching the database; validation failures return `400` with a field-level error map.
- Authorization middleware checks the resource's owning `user_id`/`fiverr_account_id` against the caller on every request — no endpoint trusts a client-supplied user id.
