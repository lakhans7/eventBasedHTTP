# Fiverr API Capabilities — Research Findings

**Status: RESEARCHED — Last checked 2026-08-28**
**Conclusion: Fiverr does not publish a general-purpose, public developer API/OAuth platform that lets a third-party application connect a seller's account, or read/write gigs, orders, messages, or reviews. No integration in this project may assume otherwise.**

## Methodology

This finding is based on:

- Direct attempts to reach Fiverr's own properties (`www.fiverr.com`, `developers.fiverr.com`, `community.fiverr.com`) from this environment. `developers.fiverr.com` does not resolve (DNS failure — no such host is published), and direct fetches of `fiverr.com` pages are blocked by this environment's network egress policy specifically because unauthenticated bulk fetching of Fiverr pages is indistinguishable from scraping, which this project must never do.
- Web search across Fiverr's public help center, community forum, press pages, and third-party developer-tracker sites (`apitracker.io`, `dlthub.com`) that catalog which companies expose public APIs.
- Cross-referencing with what is publicly known about Fiverr's product surface: Fiverr sells API *integration services* (freelancers building integrations for *other* platforms like Stripe/Shopify/WordPress on Fiverr's marketplace) and has an **Affiliate API** for affiliate marketers who promote Fiverr gigs for commission. Neither is a seller-data or account-connection API.
- Fiverr's own official seller analytics product is **Fiverr Seller Plus**, a first-party dashboard Fiverr operates itself — not a third-party-accessible API.
- Multiple third-party tools that claim to expose "Fiverr data" (Chrome extensions, PyPI packages, scraper-as-a-service products like Apify's "Fiverr Scraper") are explicitly scraping/browser-automation based, not official APIs. This project is contractually and ethically barred from using or wrapping any of them.

No official Fiverr documentation describing OAuth, seller-scoped access tokens, gig/order/message/review endpoints, or webhooks for third-party SaaS products was found. If Fiverr publishes such a program in the future, this document must be re-verified against the live documentation before any code changes here are made, per the "official documentation wins" rule.

## Capability Matrix

| Feature | Officially Supported | API / Method | Authentication | Scope | Limitations | Source |
|---|---|---|---|---|---|---|
| Third-party OAuth / "Connect your Fiverr account" | **NOT SUPPORTED** | None published | N/A | N/A | No public OAuth/app-registration portal exists for seller-account delegated access. | No `developers.fiverr.com` (DNS does not resolve); no OAuth docs found in search. |
| Seller account/profile read | **NOT SUPPORTED** | None published | N/A | N/A | Sellers can view their own profile only via fiverr.com UI. | Same as above. |
| Gig list/details read | **NOT SUPPORTED** | None published | N/A | N/A | No gig-read endpoint documented for third parties. | Same as above. |
| Gig create/update (write) | **NOT SUPPORTED** | None published | N/A | N/A | Gig management is UI-only via fiverr.com. | Same as above. |
| Order read (status, buyer, amount, deadline) | **NOT SUPPORTED** | None published | N/A | N/A | No third-party order API. Sellers can export/view orders only in Fiverr's own dashboard. | Same as above. |
| Order actions (deliver, request revision, cancel) | **NOT SUPPORTED** | None published | N/A | N/A | All order actions are Fiverr-UI-only. | Same as above. |
| Messaging read (buyer conversations) | **NOT SUPPORTED** | None published | N/A | N/A | No inbox/messages API. | Same as above. |
| Messaging send (programmatic reply) | **NOT SUPPORTED** | None published | N/A | N/A | Sending messages as a seller is only possible by the seller typing into Fiverr's own UI. Any automated sending would require unofficial access and is explicitly prohibited by this project's rules regardless of technical feasibility. | Same as above. |
| Reviews/ratings read | **NOT SUPPORTED** | None published | N/A | N/A | No reviews API. | Same as above. |
| Webhooks / real-time events | **NOT SUPPORTED** | None published | N/A | N/A | No webhook registration mechanism found. | Same as above. |
| Affiliate link/gig promotion tracking | **PARTIALLY SUPPORTED — REQUIRES APPROVAL** | Fiverr Affiliate Program / Affiliate API (via CJ Affiliate / Fiverr's own affiliate dashboard, program-specific) | Affiliate partner credentials issued after affiliate program approval | Affiliate tracking/commission data only — not seller account, order, or message data | Only relevant to affiliate marketers, not sellers managing their own gigs/orders. Out of scope for this product's seller-management use case. Requires a separate approved affiliate partnership; no public self-serve docs found. | Search results consistently describe this as the only named official Fiverr API surface, with no public auth docs. |
| Fiverr rate limits / quotas | **UNKNOWN** | N/A | N/A | N/A | Cannot be determined — no API exists to rate-limit. | N/A |
| Production app approval process | **UNKNOWN / NOT APPLICABLE** | N/A | N/A | N/A | No app-approval portal was found for a general seller-data API. | N/A |
| Data storage / retention terms for Fiverr data | **UNKNOWN — must re-check ToS at implementation time if/when an API appears** | N/A | N/A | N/A | Fiverr's Terms of Service prohibit unauthorized data collection and require third-party integrations to respect Fiverr's own terms; with no API, this is moot today. | Fiverr Terms of Service / Community Standards (public pages; full text not fetchable from this environment, see Methodology). |
| Manual/self-exported data import (CSV export from Fiverr's own dashboard, pasted by the seller) | **SUPPORTED (compliant fallback, not a Fiverr API)** | N/A — this is the seller using Fiverr's own UI/export tools and voluntarily uploading the result to our app | Our app's own auth (the seller is uploading their own already-exported data) | N/A | Data freshness depends on the seller re-exporting/re-pasting; no live sync is possible. This is the officially compliant alternative used throughout this project. | N/A — architectural decision, not a Fiverr capability. |
| Google OAuth (for *our own* app login, not Fiverr) | **SUPPORTED** | Google Identity Services / OAuth 2.0 | Standard Google OAuth 2.0 | `openid email profile` | Unrelated to Fiverr; used only for authenticating into our own SaaS. | Google's own public OAuth documentation (well-established, unrelated to Fiverr access question). |

## What this means for the product

Every feature in the original product vision that assumes a live, programmatic Fiverr connection (OAuth "Connect Fiverr" with real tokens, automatic gig/order/message/review sync, or programmatic message sending) is **not implementable today** without violating Fiverr's Terms of Service (which bar unauthorized/automated access) and this project's explicit constraints (no scraping, no browser automation, no reverse-engineered private APIs).

The system is therefore designed around a **compliant fallback that satisfies the same product goals**:

1. **"Connect Fiverr" becomes "Add Fiverr Profile (Manual)"** — the seller labels a `FiverrAccount` record with their public Fiverr username for organization purposes only. No token exchange occurs because there is nothing to exchange with.
2. **Data sync becomes seller-initiated import** — the seller exports or copies their own data from their own Fiverr dashboard (orders, gig text, buyer messages, reviews) and imports it via CSV upload or paste-to-form in our app. This is the seller using Fiverr's own tools on their own account; our app never talks to Fiverr's servers.
3. **The `MarketplaceProvider` interface and `FiverrAccount` schema retain the fields a real OAuth integration would need** (`access_token_encrypted`, `refresh_token_encrypted`, `scopes`, etc.), but they are unused (`NULL`) under the current, manual-only implementation. If Fiverr ever ships an official API, only the `FiverrProvider` implementation needs to change — the rest of the application (AI, analytics, notifications, UI) is written against the internal normalized domain models and does not know or care where the data came from.
4. **Programmatic message sending is disabled entirely.** The AI assistant only ever produces a *draft* the seller must copy into Fiverr's own inbox themselves, or (if a future official send API exists and the seller opts in) an explicit human-approved send action gated behind a feature flag that defaults to off.
5. Every place in the codebase that would call a real Fiverr endpoint is written behind the `MarketplaceProvider` interface and returns `ErrNotSupportedByProvider` today, with a comment pointing back to this document.

## Reassessment trigger

Re-run this research and update this document before implementing any change that assumes new Fiverr API capability. Specifically check:

- `https://developers.fiverr.com` resolves and publishes OAuth/API docs.
- Fiverr's Help Center / Community announcements for a "Fiverr API" or "Fiverr for Developers" program.
- Whether Fiverr Pro or Fiverr Enterprise now offers partner APIs (recheck via web search — direct fetch of fiverr.com is not possible from this environment).

If official documentation ever contradicts this document, **the official documentation wins** and this file must be updated first, before any code changes.
