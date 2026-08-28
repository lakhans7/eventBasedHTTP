# Deploying to Fly.io

This gets you a real, persistent `https://<your-app>.fly.dev` URL — unlike the sandboxed
session this was built in, Fly.io machines have public networking. You need a Fly.io account
and the `flyctl` CLI; everything below runs from your own machine, not this session (I have no
way to reach Fly's API or your account from here).

## 0. Prerequisites

```bash
curl -L https://fly.io/install.sh | sh   # installs flyctl
fly auth login                            # opens a browser to log in / sign up
```

## 1. Pick an app name and update fly.toml

Fly app names are globally unique. Edit `fly.toml`:
- `app = "..."` — pick something unused (fly will tell you if it's taken when you deploy)
- `FRONTEND_ORIGIN` under `[env]` — update to `https://<the same name>.fly.dev`

```bash
fly apps create <your-app-name>   # or just run `fly launch --no-deploy` and let it create fly.toml's app name for you
```

## 2. Provision Postgres

```bash
fly postgres create --name <your-app-name>-db --region iad --vm-size shared-cpu-1x --initial-cluster-size 1
fly postgres attach <your-app-name>-db --app <your-app-name>
```

`attach` automatically sets the `DATABASE_URL` secret on your app to point at the new
Postgres — you don't need to construct it by hand. Migrations run automatically the moment
the server process starts (`internal/db.Migrate`, called from `cmd/server/main.go`), so there's
no separate migration step.

## 3. Provision Redis

```bash
fly redis create --name <your-app-name>-redis --region iad
```

This provisions a managed Upstash Redis and prints a `rediss://...` connection string with
a password baked in. Set it as a secret (this app reads `REDIS_URL`, not the `UPSTASH_REDIS_URL`
name `fly redis create` prints — copy the value it gives you):

```bash
fly secrets set --app <your-app-name> REDIS_URL="rediss://default:<password>@<host>:<port>"
```

`internal/jobs.RedisClientOpt` and the health-check Redis client both parse this URL directly,
including the TLS (`rediss://`) and password — no code changes needed for a managed Redis vs.
the bare one in `docker-compose.yml`.

## 4. Set the remaining secrets

```bash
fly secrets set --app <your-app-name> \
  JWT_SECRET="$(openssl rand -hex 32)" \
  SMTP_HOST="" \
  ANTHROPIC_API_KEY=""   # optional — leave blank and the AI Assistant runs on mock responses
```

Anything not set as a secret falls back to `fly.toml`'s `[env]` block or this app's built-in
defaults (see `internal/config/config.go`). At minimum you need `JWT_SECRET` — the server
refuses to start in production without one (see docs/security.md).

Optional, if you want them:
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_REDIRECT_URL` — set `GOOGLE_REDIRECT_URL`
  to `https://<your-app-name>.fly.dev/api/v1/auth/google/callback` and add that same URL to the
  OAuth client's allowed redirect URIs in Google Cloud Console.
- `SMTP_HOST` / `SMTP_PORT` / `SMTP_USER` / `SMTP_PASS` / `SMTP_FROM` — without these, email
  (verification, password reset) is logged instead of sent (`internal/mailer`), which is fine
  for a demo but not for real users.
- `ANTHROPIC_API_KEY` / `AI_MODEL` — without a key, `/ai/*` endpoints return a clearly-labeled
  mock response (`internal/ai.MockClient`) instead of a real model completion.

## 5. Deploy

```bash
fly deploy --app <your-app-name>
```

This builds `Dockerfile` (one image with both `./server` and `./worker` binaries) and starts
the two process groups defined in `fly.toml`'s `[processes]` — `app` gets the public HTTPS
service on port 443 (proxied to the container's port 3000, with TLS terminated by Fly), `worker`
runs with no public port at all, exactly like `docker-compose.yml`'s `app`/`worker` services.

## 6. Verify

```bash
curl https://<your-app-name>.fly.dev/health   # {"status":"ok"}
curl https://<your-app-name>.fly.dev/ready    # {"status":"ready"} once Postgres+Redis are reachable
fly logs --app <your-app-name>
```

Then open `https://<your-app-name>.fly.dev` in a browser — it redirects to `/login.html`.

## Scaling / cost notes

- `min_machines_running = 0` on the `app` process means Fly stops the machine when idle and
  cold-starts it on the next request — cheapest option for a demo, adds a few seconds of
  latency to the first request after idle. Set it to `1` in `fly.toml` if you want it always warm.
- The `worker` process has no `http_service`, so it always runs (Fly doesn't auto-stop
  processes with no public service) — it's what picks up CSV import jobs and outbound email.
  If you're not actively using imports, `fly scale count worker=0 --app <your-app-name>` stops
  it; imports will just sit `queued` until you scale it back up.
- Fly Postgres and Fly Redis (Upstash) both have their own free/hobby tiers — check current
  pricing on fly.io before provisioning at a larger size than you need.

## What I couldn't verify from this session

This session has no way to reach Fly's API or your account, and the sandbox's Docker daemon
isn't available to me either (no privileged access), so I could not run `docker build` or
`fly deploy` myself end-to-end. I did verify: the Go binaries build cleanly
(`go build ./...`), `fly.toml` parses as valid TOML, and the full application (server + worker +
Postgres + Redis) runs correctly when started directly in this sandbox. The Dockerfile is a
standard multi-stage Go build, but do check `fly deploy`'s build output on your first real run
in case something in this environment's image differs from Fly's builders.
