# Single-Product Shop

A single-page storefront for one product, with a real Razorpay checkout integration:
server-decided pricing (₹799, fixed), size selection (S/M/L/XL/XXL/XXXL), and signature-verified
payment confirmation.

## ⚠️ Before you sell anything: product photos

The two product photos in `web/images/` are cropped from a screenshot of a wholesale/ODM
supplier listing (visible watermark-free supplier branding, "MOQ: 100", "Send inquiry" —
consistent with an Alibaba-style B2B listing). Those are almost certainly the **manufacturer's
marketing photography**, not photos you have a license to use on your own storefront. Before
this goes live:

- Replace them with photos you own the rights to — your own product photography, or images
  explicitly licensed to you by the supplier (some wholesale suppliers do grant resale/marketing
  rights when you place an order — confirm this with them in writing, don't assume it).
- Using someone else's commercial photography without permission is a real copyright exposure,
  independent of anything about the product itself.

Nothing about *selling lingerie* is unusual here — it's a completely mainstream retail category
in India (Clovia, Zivame, Myntra, etc. all sell similar sheer/lace styles) and a normal Razorpay
merchant category ("Apparel & Fashion" / "Clothing"). The photo-licensing issue above is the only
thing to actually resolve before launch.

## How it works

1. Buyer picks a size and quantity, fills in shipping details, and submits the form.
2. The **server** — never the browser — computes the amount (`₹799 × quantity`) and creates a
   Razorpay Order via Razorpay's Orders API (`internal/razorpay`). This is what stops someone
   from tampering with the price in their browser's dev tools.
3. Razorpay's own Checkout modal (loaded from `checkout.razorpay.com`, per their official
   integration docs) collects card/UPI/etc. details — this app never sees or stores payment
   details itself.
4. On success, the browser calls back to `/api/verify`, which checks Razorpay's HMAC-SHA256
   signature before marking the order paid. A payment is never trusted just because the browser
   *says* it succeeded.
5. Every order's shipping details are attached to its Razorpay Order's `notes` field, so the
   **Razorpay dashboard itself is enough to fulfill orders** — no separate admin panel needed.
   `orders.jsonl` (see "Persistence" below) is a local backup, not the primary record.

## Setup

### 1. Get Razorpay keys

- Sign up at [razorpay.com](https://razorpay.com) (Indian business/bank details required for a
  real, live account — but **test mode needs neither**).
- Dashboard → **Test Mode** (toggle, top right) → Settings → API Keys → generate a test key pair.
- Test mode lets you run the entire flow, including Checkout, with no real money moving —
  Razorpay's test cards are documented at
  https://razorpay.com/docs/payments/payments/test-card-upi-details/.

### 2. Configure

```bash
cp .env.example .env
# fill in RAZORPAY_KEY_ID / RAZORPAY_KEY_SECRET with your test keys
```

Customize `STORE_NAME` and `PRODUCT_NAME` in `.env` too. The price (₹799) is intentionally not
an env var — see the comment in `internal/config/config.go` — edit `PriceCents` there directly
if it ever needs to change, so a missing/misconfigured env var can never accidentally undercharge
a customer.

### 3. Run

```bash
go run ./cmd/server
```

Open http://localhost:3000. Use a Razorpay test card (e.g. `4111 1111 1111 1111`, any future
expiry, any CVV) to complete a full test purchase.

### 4. Go live

- Complete Razorpay's KYC/business verification (Dashboard → Account & Settings).
- Switch `.env` to your **live** key pair.
- Set `APP_ENV=production` (the app refuses to start in production without Razorpay keys set).
- Optional but recommended: Dashboard → Settings → Webhooks → add
  `https://<your-domain>/api/webhook/razorpay` for the `payment.captured` event, and set
  `RAZORPAY_WEBHOOK_SECRET` to the secret Razorpay gives you — this is a safety net that marks
  an order paid even if the buyer's browser closes right after paying, before it can call
  `/api/verify`.

## Persistence

`orders.jsonl` (path configurable via `ORDERS_LOG_PATH`) is an append-only local file — see
`internal/orders`. It's a convenience backup, not the source of truth (the Razorpay dashboard is)
— which matters because:

- On most container platforms (Fly.io, Render, Railway, etc.), local disk is **ephemeral** by
  default — it's wiped on every redeploy and isn't shared across multiple machine instances.
- If you want the local backup to actually persist, mount a real volume at the directory
  `ORDERS_LOG_PATH` points to (the `Dockerfile` defaults it to `/app/data/orders.jsonl`).
- If you don't set up a volume, that's fine — just rely on the Razorpay dashboard for order
  fulfillment, which is why every order's details are written into the Razorpay Order's `notes`
  in the first place.

A real database was deliberately not added for one SKU at low volume — see the comment in
`internal/orders/store.go`.

## Deploying

Any container host works — `Dockerfile` is a standard single-binary Go image with no database
dependency, so there's no Postgres/Redis to provision (unlike the sibling `fiverTest` branch's
Fiverr platform, which does need those). `fly.toml` is set up for Fly.io specifically since it's
the least setup for an app this size; Render/Railway/a VPS with `docker run` all work too — just
set the same variables from `.env.example` as secrets/env vars there instead.

### Fly.io

```bash
curl -L https://fly.io/install.sh | sh   # installs flyctl, if you don't have it
fly auth login
```

1. Pick a unique app name and edit `fly.toml`: change `app = "..."` and `FRONTEND_ORIGIN` to match.
2. `fly apps create <your-app-name>` (or `fly launch --no-deploy` and let it pick up `fly.toml`'s name).
3. `fly volumes create shop_data --region bom --size 1` — backs the `orders.jsonl` local order
   backup so it survives redeploys (see "Persistence" above). Skip this if you're relying on the
   Razorpay dashboard alone for fulfillment; then also delete the `[mounts]` block in `fly.toml`.
4. Set secrets:
   ```bash
   fly secrets set --app <your-app-name> \
     RAZORPAY_KEY_ID="rzp_test_..." \
     RAZORPAY_KEY_SECRET="..."
   ```
   (Switch to live keys and re-run once you're actually ready to take real payments — see "Go live" above.)
5. `fly deploy --app <your-app-name>`
6. `curl https://<your-app-name>.fly.dev/health` → `{"status":"ok"}`, then open it in a browser.

I can't run this step myself or hand you back a link — it needs your Fly account, and this
sandboxed session has no public networking of its own (see the "public link" conversation this
came out of). Paste me the deploy output if anything fails and I'll fix it.

## Testing

```bash
go vet ./...
go test ./...
```

All tests are self-contained (no network, no real Razorpay account needed): HMAC signature
verification tested against an independently-computed reference (`internal/razorpay`), the local
order log's append/idempotency behavior (`internal/orders`), and the checkout API's input
validation (`internal/shop`) — including confirming a tampered/invalid request is always rejected
with `400` before the app would ever call Razorpay.

## Security notes

- The Razorpay **key secret** never reaches the browser — only the public **key id** does
  (Razorpay's own model: the key id is meant to be public, similar to a Stripe publishable key).
- The amount charged is always server-computed from the fixed price and a bounds-checked quantity
  (1–10) — never taken from the client request.
- Every payment is confirmed via Razorpay's documented HMAC-SHA256 signature check
  (`internal/razorpay.VerifyPaymentSignature`), not merely because the browser's success callback
  fired.
- Input (phone, pincode, size, quantity) is validated server-side — the HTML form's `required`
  attributes are a UX nicety, not a security boundary, since anyone can call `/api/order` directly.
