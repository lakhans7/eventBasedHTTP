-- Initial schema. See docs/database.md for the full data dictionary and rationale.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email               citext UNIQUE NOT NULL,
    name                text NOT NULL DEFAULT '',
    password_hash       text,
    avatar_url          text,
    status              text NOT NULL DEFAULT 'active',
    email_verified_at   timestamptz,
    last_login_at       timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE TABLE auth_identities (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider            text NOT NULL,
    provider_user_id    text NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_user_id)
);
CREATE INDEX idx_auth_identities_user ON auth_identities(user_id);

CREATE TABLE sessions (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash  text NOT NULL,
    user_agent          text,
    ip_address          text,
    expires_at          timestamptz NOT NULL,
    revoked_at          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE UNIQUE INDEX idx_sessions_refresh_hash ON sessions(refresh_token_hash);

CREATE TABLE email_verification_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_evt_user ON email_verification_tokens(user_id);
CREATE UNIQUE INDEX idx_evt_hash ON email_verification_tokens(token_hash);

CREATE TABLE password_reset_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_prt_user ON password_reset_tokens(user_id);
CREATE UNIQUE INDEX idx_prt_hash ON password_reset_tokens(token_hash);

-- Fiverr has no public API today (docs/fiverr-api-capabilities.md). Token columns
-- are schema-forward-compatible but unused while connection_method = 'manual'.
CREATE TABLE fiverr_accounts (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_account_id     text,
    username                 text NOT NULL,
    connection_method        text NOT NULL DEFAULT 'manual' CHECK (connection_method IN ('manual','oauth')),
    access_token_encrypted   bytea,
    refresh_token_encrypted  bytea,
    token_expires_at         timestamptz,
    scopes                   text[] NOT NULL DEFAULT '{}',
    status                   text NOT NULL DEFAULT 'connected' CHECK (status IN ('connected','disconnected')),
    connected_at             timestamptz NOT NULL DEFAULT now(),
    last_sync_at             timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    deleted_at               timestamptz
);
CREATE INDEX idx_fiverr_accounts_user ON fiverr_accounts(user_id);

CREATE TABLE customers (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fiverr_account_id   uuid NOT NULL REFERENCES fiverr_accounts(id) ON DELETE CASCADE,
    external_ref        text NOT NULL,
    display_name        text NOT NULL DEFAULT '',
    notes               text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE(fiverr_account_id, external_ref)
);
CREATE INDEX idx_customers_account ON customers(fiverr_account_id);

CREATE TABLE gigs (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fiverr_account_id   uuid NOT NULL REFERENCES fiverr_accounts(id) ON DELETE CASCADE,
    external_ref        text,
    title               text NOT NULL,
    description         text NOT NULL DEFAULT '',
    category            text,
    status              text NOT NULL DEFAULT 'unknown',
    base_price_cents    bigint NOT NULL DEFAULT 0,
    currency            text NOT NULL DEFAULT 'usd',
    packages            jsonb NOT NULL DEFAULT '[]',
    metrics             jsonb,
    source              text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','csv_import')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);
CREATE INDEX idx_gigs_account ON gigs(fiverr_account_id);

CREATE TABLE orders (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fiverr_account_id   uuid NOT NULL REFERENCES fiverr_accounts(id) ON DELETE CASCADE,
    gig_id              uuid REFERENCES gigs(id) ON DELETE SET NULL,
    customer_id         uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    external_ref        text,
    amount_cents        bigint NOT NULL DEFAULT 0,
    currency            text NOT NULL DEFAULT 'usd',
    status              text NOT NULL DEFAULT 'active',
    stage               text NOT NULL DEFAULT 'created',
    due_at              timestamptz,
    delivered_at        timestamptz,
    completed_at        timestamptz,
    cancelled_at        timestamptz,
    source              text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','csv_import')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);
CREATE INDEX idx_orders_account ON orders(fiverr_account_id);
CREATE INDEX idx_orders_status_due ON orders(status, due_at);
CREATE INDEX idx_orders_customer ON orders(customer_id);

CREATE TABLE order_requirements (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    raw_text    text NOT NULL,
    extracted   jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_requirements_order ON order_requirements(order_id);

CREATE TABLE conversations (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fiverr_account_id   uuid NOT NULL REFERENCES fiverr_accounts(id) ON DELETE CASCADE,
    customer_id         uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    gig_id              uuid REFERENCES gigs(id) ON DELETE SET NULL,
    last_message_at     timestamptz NOT NULL DEFAULT now(),
    unread_count        integer NOT NULL DEFAULT 0,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_conversations_account ON conversations(fiverr_account_id);
CREATE INDEX idx_conversations_last_message ON conversations(last_message_at DESC);

CREATE TABLE messages (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id     uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    direction           text NOT NULL CHECK (direction IN ('inbound','outbound')),
    body                text NOT NULL,
    sent_at             timestamptz NOT NULL DEFAULT now(),
    source              text NOT NULL DEFAULT 'manual_paste' CHECK (source IN ('manual_paste','csv_import')),
    ai_generation_id    uuid,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_conversation_sent ON messages(conversation_id, sent_at);

CREATE TABLE reviews (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fiverr_account_id   uuid NOT NULL REFERENCES fiverr_accounts(id) ON DELETE CASCADE,
    gig_id              uuid REFERENCES gigs(id) ON DELETE SET NULL,
    order_id            uuid REFERENCES orders(id) ON DELETE SET NULL,
    rating              smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
    body                text,
    sentiment           text,
    posted_at           timestamptz NOT NULL DEFAULT now(),
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_reviews_account ON reviews(fiverr_account_id);

CREATE TABLE notifications (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            text NOT NULL,
    title           text NOT NULL,
    body            text NOT NULL DEFAULT '',
    resource_type   text,
    resource_id     uuid,
    read_at         timestamptz,
    channels        text[] NOT NULL DEFAULT '{in_app}',
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_user_created ON notifications(user_id, created_at DESC);

CREATE TABLE ai_generations (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind                text NOT NULL,
    context_ref         jsonb NOT NULL DEFAULT '{}',
    model               text NOT NULL,
    input_tokens        integer NOT NULL DEFAULT 0,
    output_tokens       integer NOT NULL DEFAULT 0,
    estimated_cost_usd  numeric(10,4) NOT NULL DEFAULT 0,
    prompt_redacted     text NOT NULL DEFAULT '',
    draft_output        text NOT NULL DEFAULT '',
    risk_flags          text[] NOT NULL DEFAULT '{}',
    status              text NOT NULL DEFAULT 'pending_review',
    approved_by         uuid REFERENCES users(id),
    approved_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_generations_user ON ai_generations(user_id, created_at DESC);

ALTER TABLE messages ADD CONSTRAINT fk_messages_ai_generation
    FOREIGN KEY (ai_generation_id) REFERENCES ai_generations(id) ON DELETE SET NULL;

CREATE TABLE ai_feedback (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    ai_generation_id    uuid NOT NULL REFERENCES ai_generations(id) ON DELETE CASCADE,
    user_id             uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating              smallint NOT NULL CHECK (rating IN (-1, 1)),
    comment             text,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_feedback_generation ON ai_feedback(ai_generation_id);

CREATE TABLE seller_preferences (
    user_id                       uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    skills                        text[] NOT NULL DEFAULT '{}',
    services                      text[] NOT NULL DEFAULT '{}',
    tone                          text NOT NULL DEFAULT 'professional but friendly',
    min_project_usd               integer NOT NULL DEFAULT 0,
    typical_delivery_days_min     integer NOT NULL DEFAULT 3,
    typical_delivery_days_max     integer NOT NULL DEFAULT 7,
    faqs                          jsonb NOT NULL DEFAULT '[]',
    portfolio_links               text[] NOT NULL DEFAULT '{}',
    terms                         text,
    restrictions                  text,
    updated_at                    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sync_jobs (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    fiverr_account_id   uuid REFERENCES fiverr_accounts(id) ON DELETE CASCADE,
    job_type            text NOT NULL,
    status              text NOT NULL DEFAULT 'queued',
    attempt             integer NOT NULL DEFAULT 0,
    error               text,
    started_at          timestamptz,
    finished_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sync_jobs_account ON sync_jobs(fiverr_account_id, created_at DESC);

CREATE TABLE audit_logs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid REFERENCES users(id) ON DELETE SET NULL,
    action          text NOT NULL,
    resource_type   text,
    resource_id     text,
    metadata        jsonb NOT NULL DEFAULT '{}',
    ip_address      text,
    user_agent      text,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_user_created ON audit_logs(user_id, created_at DESC);
