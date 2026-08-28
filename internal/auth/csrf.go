package auth

import "crypto/subtle"

// CSRF protection strategy (see docs/security.md): every authenticated,
// state-changing API call must carry `Authorization: Bearer <access token>`,
// which a cross-site form/fetch cannot attach (no CORS credentials, no
// ambient cookie auto-attach for a custom header) — this alone defeats
// classic CSRF for the bulk of the API. The one endpoint that authenticates
// via an ambient cookie only (POST /auth/refresh, which reads the HttpOnly
// refresh cookie) additionally requires a double-submit CSRF token: a
// non-HttpOnly `csrf_token` cookie whose value must be echoed back in the
// `X-CSRF-Token` header. An attacker's page can trigger the cookie to be
// sent, but cannot read its value to also set the header (same-origin
// policy), so the two values won't match.
func ValidCSRF(cookieValue, headerValue string) bool {
	if cookieValue == "" || headerValue == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieValue), []byte(headerValue)) == 1
}
