# Database Schema

PostgreSQL. Migrations live in `migrations/` (golang-migrate format: `NNNN_name.up.sql` / `.down.sql`). All tables use `uuid` primary keys (`gen_random_uuid()`, pgcrypto), `created_at`/`updated_at` timestamps, and soft-deletion (`deleted_at`) where records may need to be hidden without losing audit history.

## Entity overview

```
users ──< sessions
  │  ╲
  │   ╲──< fiverr_accounts ──< gigs ──< reviews
  │                        ├──< customers ──< orders ──< order_requirements
  │                        │                        └──< reviews
  │                        └──< conversations ──< messages
  ├──< seller_preferences (1:1)
  ├──< notifications
  ├──< ai_generations ──< ai_feedback
  ├──< sync_jobs
  └──< audit_logs
```

## Tables

### users
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| email | citext, unique, not null | |
| name | text | |
| password_hash | text, nullable | null for Google-only accounts |
| avatar_url | text | |
| status | text | `active`, `suspended`, `pending_deletion` |
| email_verified_at | timestamptz | |
| last_login_at | timestamptz | |
| created_at / updated_at | timestamptz | |

### auth_identities
Google OAuth (and future providers) linked to a user.
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| user_id | uuid FK → users | |
| provider | text | `google` |
| provider_user_id | text | |
| unique(provider, provider_user_id) | | |

### sessions
Supports "logout from all devices."
| column | type | notes |
|---|---|---|
| id | uuid PK | also the JWT `jti` |
| user_id | uuid FK → users | |
| refresh_token_hash | text | sha256, never store raw |
| user_agent / ip_address | text | |
| expires_at | timestamptz | |
| revoked_at | timestamptz nullable | |

### email_verification_tokens / password_reset_tokens
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| user_id | uuid FK | |
| token_hash | text | sha256 of the emailed token |
| expires_at | timestamptz | |
| used_at | timestamptz nullable | |

### fiverr_accounts
Per `docs/fiverr-api-capabilities.md`, `access_token_encrypted`/`refresh_token_encrypted`/`scopes`/`token_expires_at` are schema-forward-compatible but **unused** (`NULL`) while `connection_method = 'manual'`.
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| user_id | uuid FK → users | |
| external_account_id | text nullable | unused until a real API exists |
| username | text | seller's public Fiverr username, self-declared |
| connection_method | text | `manual` (only value supported today) or `oauth` (reserved) |
| access_token_encrypted | bytea nullable | AES-256-GCM, unused today |
| refresh_token_encrypted | bytea nullable | unused today |
| token_expires_at | timestamptz nullable | unused today |
| scopes | text[] | unused today |
| status | text | `connected` (manual), `disconnected` |
| connected_at / last_sync_at | timestamptz | `last_sync_at` = last successful import |
| created_at / updated_at | timestamptz | |

### customers
Normalized buyer, deduplicated per Fiverr account by `external_ref` (buyer username as typed/imported by the seller — best-effort, not a Fiverr ID).
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| fiverr_account_id | uuid FK | |
| external_ref | text | buyer username, seller-supplied |
| display_name | text | |
| notes | text | seller's private notes |
| unique(fiverr_account_id, external_ref) | | |

### gigs
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| fiverr_account_id | uuid FK | |
| external_ref | text nullable | gig URL/slug if the seller provided one |
| title, description, category | text | |
| status | text | `active`, `paused`, `denied`, `unknown` |
| base_price_cents | integer | |
| currency | text | default `usd` |
| packages | jsonb | `[{name, price_cents, delivery_days, description}]` |
| metrics | jsonb | seller-entered/imported views/orders/rating snapshot; always carries `as_of` |
| source | text | `manual` \| `csv_import` |

### orders
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| fiverr_account_id | uuid FK | |
| gig_id | uuid FK nullable | |
| customer_id | uuid FK | |
| external_ref | text nullable | Fiverr order number, if the seller entered it |
| amount_cents | integer | |
| currency | text | |
| status | text | `active`, `delivered`, `revision_requested`, `completed`, `cancelled`, `late` |
| stage | text | order-timeline stage: `created`, `requirements_received`, `in_progress`, `delivered`, `revision_requested`, `revision_submitted`, `completed` |
| due_at | timestamptz nullable | |
| delivered_at / completed_at / cancelled_at | timestamptz nullable | |
| source | text | `manual` \| `csv_import` |

### order_requirements
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| order_id | uuid FK | |
| raw_text | text | buyer's requirement text, as pasted by the seller |
| extracted | jsonb nullable | AI-extracted structure `{technologies:[], features:[], missing:[]}` |

### conversations
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| fiverr_account_id | uuid FK | |
| customer_id | uuid FK | |
| gig_id | uuid FK nullable | |
| last_message_at | timestamptz | |
| unread_count | integer | seller-maintained, since we cannot read real unread state from Fiverr |

### messages
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| conversation_id | uuid FK | |
| direction | text | `inbound` (buyer) or `outbound` (seller) |
| body | text | |
| sent_at | timestamptz | |
| source | text | `manual_paste` \| `csv_import` |
| ai_generation_id | uuid FK nullable | set when an outbound message originated from an approved AI draft |

### reviews
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| fiverr_account_id | uuid FK | |
| gig_id | uuid FK nullable | |
| order_id | uuid FK nullable | |
| rating | smallint | 1–5 |
| body | text | |
| sentiment | text nullable | AI-computed |
| posted_at | timestamptz | |

### notifications
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| user_id | uuid FK | |
| type | text | `new_message`, `new_order`, `deadline`, `revision_requested`, `delivery`, `sync_failure`, `connection_failure` |
| title, body | text | |
| resource_type, resource_id | text/uuid nullable | |
| read_at | timestamptz nullable | |
| channels | text[] | e.g. `{in_app,email}` |

### ai_generations
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| user_id | uuid FK | |
| kind | text | `message_reply`, `requirement_summary`, `delivery_message`, `review_analysis`, `insight` |
| context_ref | jsonb | e.g. `{conversation_id, order_id}` |
| model | text | |
| input_tokens, output_tokens | integer | |
| estimated_cost_usd | numeric(10,4) | |
| prompt_redacted | text | prompt with buyer PII/system prompt redacted before storage |
| draft_output | text | |
| risk_flags | text[] | from the safety pipeline |
| status | text | `pending_review`, `edited`, `approved`, `rejected`, `sent_externally` (seller confirms they pasted it into Fiverr) |
| approved_by | uuid FK nullable | |
| approved_at | timestamptz nullable | |

### ai_feedback
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| ai_generation_id | uuid FK | |
| user_id | uuid FK | |
| rating | smallint | thumbs up/down (1/-1) |
| comment | text nullable | |

### seller_preferences
1:1 with `users`. The AI knowledge base from section 13.
| column | type | notes |
|---|---|---|
| user_id | uuid PK/FK | |
| skills | text[] | |
| services | text[] | |
| tone | text | free text, e.g. "professional but friendly" |
| min_project_usd | integer | |
| typical_delivery_days_min/max | integer | |
| faqs | jsonb | `[{q,a}]` |
| portfolio_links | text[] | |
| terms | text | |
| restrictions | text | e.g. "never offer > 20% discount" — enforced by the AI risk detector |

### sync_jobs
Audit trail of import/background job runs (not live "syncs," since there's nothing to poll).
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| fiverr_account_id | uuid FK nullable | |
| job_type | text | `csv_import`, `analytics_refresh`, `token_refresh_noop`, ... |
| status | text | `queued`, `running`, `succeeded`, `failed` |
| attempt | integer | |
| error | text nullable | |
| started_at / finished_at | timestamptz | |

### audit_logs
| column | type | notes |
|---|---|---|
| id | uuid PK | |
| user_id | uuid FK nullable | |
| action | text | e.g. `fiverr_account.connected`, `ai.generated`, `ai.approved`, `message.marked_sent`, `order.viewed`, `settings.updated` |
| resource_type, resource_id | text/uuid nullable | |
| metadata | jsonb | never contains secrets/tokens — enforced by `audit.Service` redaction |
| ip_address, user_agent | text | |
| created_at | timestamptz | |

## Constraints & indexes

- Foreign keys `ON DELETE CASCADE` from account-owned tables (`gigs`, `orders`, `conversations`, ...) down from `fiverr_accounts`, which cascades from `users`.
- Unique constraints: `users.email`, `auth_identities(provider, provider_user_id)`, `customers(fiverr_account_id, external_ref)`.
- Indexes on all foreign keys, plus `orders(status, due_at)` for the deadlines dashboard, `messages(conversation_id, sent_at)` for inbox pagination, `audit_logs(user_id, created_at)`.
- All money stored as integer cents to avoid floating-point rounding.
