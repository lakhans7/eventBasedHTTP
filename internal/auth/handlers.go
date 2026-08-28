package auth

import (
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/lakhans7/eventbasedhttp/internal/audit"
)

var emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const (
	refreshCookieName  = "refresh_token"
	csrfCookieName     = "csrf_token"
	stateCookieName    = "oauth_state"
	verifierCookieName = "oauth_pkce_verifier"
)

type Handlers struct {
	svc          *Service
	google       *GoogleOAuth
	audit        *audit.Service
	cookieDomain string
	cookieSecure bool
	refreshTTL   time.Duration
}

func NewHandlers(svc *Service, google *GoogleOAuth, auditSvc *audit.Service, cookieDomain string, cookieSecure bool, refreshTTL time.Duration) *Handlers {
	return &Handlers{svc: svc, google: google, audit: auditSvc, cookieDomain: cookieDomain, cookieSecure: cookieSecure, refreshTTL: refreshTTL}
}

func errJSON(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}

func (h *Handlers) setSessionCookies(c *fiber.Ctx, tp *TokenPair) {
	csrfToken, err := GenerateOpaqueToken()
	if err != nil {
		csrfToken = tp.SessionID // extremely unlikely fallback; still unique per session
	}
	c.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    tp.RefreshToken,
		HTTPOnly: true,
		Secure:   h.cookieSecure,
		SameSite: "Strict",
		Domain:   h.cookieDomain,
		Path:     "/api/v1/auth",
		Expires:  time.Now().Add(h.refreshTTL),
	})
	c.Cookie(&fiber.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		HTTPOnly: false,
		Secure:   h.cookieSecure,
		SameSite: "Strict",
		Domain:   h.cookieDomain,
		Path:     "/api/v1/auth",
		Expires:  time.Now().Add(h.refreshTTL),
	})
}

func (h *Handlers) clearSessionCookies(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{Name: refreshCookieName, Value: "", Expires: time.Now().Add(-time.Hour), Path: "/api/v1/auth"})
	c.Cookie(&fiber.Cookie{Name: csrfCookieName, Value: "", Expires: time.Now().Add(-time.Hour), Path: "/api/v1/auth"})
}

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (h *Handlers) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Could not parse request body.")
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if !emailRE.MatchString(req.Email) {
		return errJSON(c, fiber.StatusBadRequest, "invalid_email", "A valid email address is required.")
	}
	if len(req.Password) < 8 {
		return errJSON(c, fiber.StatusBadRequest, "weak_password", "Password must be at least 8 characters.")
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = strings.Split(req.Email, "@")[0]
	}

	user, err := h.svc.Register(c.Context(), req.Email, req.Name, req.Password, c.IP(), c.Get("User-Agent"))
	if err == ErrEmailAlreadyRegistered {
		return errJSON(c, fiber.StatusConflict, "email_taken", err.Error())
	}
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not create account.")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": user})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Could not parse request body.")
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	user, err := h.svc.Authenticate(c.Context(), req.Email, req.Password)
	if err == ErrInvalidCredentials {
		return errJSON(c, fiber.StatusUnauthorized, "invalid_credentials", err.Error())
	}
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not log in.")
	}

	tp, err := h.svc.IssueTokenPair(c.Context(), user.ID, c.IP(), c.Get("User-Agent"))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not create session.")
	}
	h.setSessionCookies(c, tp)
	h.audit.Log(c.Context(), audit.Entry{UserID: &user.ID, Action: "user.logged_in", ResourceType: "user", ResourceID: user.ID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})

	return c.JSON(fiber.Map{"access_token": tp.AccessToken, "user": user})
}

func (h *Handlers) Refresh(c *fiber.Ctx) error {
	refreshToken := c.Cookies(refreshCookieName)
	if refreshToken == "" {
		return errJSON(c, fiber.StatusUnauthorized, "no_session", "No active session.")
	}
	if !ValidCSRF(c.Cookies(csrfCookieName), c.Get("X-CSRF-Token")) {
		return errJSON(c, fiber.StatusForbidden, "csrf_mismatch", "CSRF token missing or invalid.")
	}

	tp, err := h.svc.Refresh(c.Context(), refreshToken, c.IP(), c.Get("User-Agent"))
	if err == ErrSessionRevokedOrExpired {
		h.clearSessionCookies(c)
		return errJSON(c, fiber.StatusUnauthorized, "session_expired", err.Error())
	}
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not refresh session.")
	}
	h.setSessionCookies(c, tp)
	return c.JSON(fiber.Map{"access_token": tp.AccessToken})
}

func (h *Handlers) Logout(c *fiber.Ctx) error {
	sessionID := SessionIDFromContext(c)
	if sessionID != "" {
		_ = h.svc.Logout(c.Context(), sessionID)
	}
	h.clearSessionCookies(c)
	return c.JSON(fiber.Map{"status": "logged_out"})
}

func (h *Handlers) LogoutAll(c *fiber.Ctx) error {
	userID := UserIDFromContext(c)
	if err := h.svc.LogoutAllDevices(c.Context(), userID); err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not log out of all devices.")
	}
	h.clearSessionCookies(c)
	h.audit.Log(c.Context(), audit.Entry{UserID: &userID, Action: "user.logged_out_all_devices", ResourceType: "user", ResourceID: userID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})
	return c.JSON(fiber.Map{"status": "logged_out_all_devices"})
}

func (h *Handlers) VerifyEmail(c *fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil || req.Token == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "A token is required.")
	}
	if err := h.svc.VerifyEmail(c.Context(), req.Token); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_token", err.Error())
	}
	return c.JSON(fiber.Map{"status": "verified"})
}

func (h *Handlers) ForgotPassword(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Email is required.")
	}
	h.svc.ForgotPassword(c.Context(), strings.ToLower(strings.TrimSpace(req.Email)))
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *Handlers) ResetPassword(c *fiber.Ctx) error {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Token == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_body", "Token and new_password are required.")
	}
	if len(req.NewPassword) < 8 {
		return errJSON(c, fiber.StatusBadRequest, "weak_password", "Password must be at least 8 characters.")
	}
	if err := h.svc.ResetPassword(c.Context(), req.Token, req.NewPassword); err != nil {
		return errJSON(c, fiber.StatusBadRequest, "invalid_token", err.Error())
	}
	return c.JSON(fiber.Map{"status": "password_reset"})
}

func (h *Handlers) DeleteAccount(c *fiber.Ctx) error {
	userID := UserIDFromContext(c)
	if err := h.svc.DeleteAccount(c.Context(), userID); err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not delete account.")
	}
	h.clearSessionCookies(c)
	h.audit.Log(c.Context(), audit.Entry{UserID: &userID, Action: "user.deleted_account", ResourceType: "user", ResourceID: userID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})
	return c.JSON(fiber.Map{"status": "pending_deletion"})
}

func (h *Handlers) Me(c *fiber.Ctx) error {
	user, err := h.svc.GetUserByID(c.Context(), UserIDFromContext(c))
	if err != nil {
		return errJSON(c, fiber.StatusNotFound, "not_found", "User not found.")
	}
	return c.JSON(fiber.Map{"user": user})
}

// --- Google OAuth ---

func (h *Handlers) GoogleStart(c *fiber.Ctx) error {
	if !h.google.Enabled() {
		return errJSON(c, fiber.StatusServiceUnavailable, "google_oauth_disabled", "Google sign-in is not configured on this server.")
	}
	state, err := GenerateOpaqueToken()
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not start Google sign-in.")
	}
	verifier, challenge, err := NewPKCEPair()
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not start Google sign-in.")
	}

	c.Cookie(&fiber.Cookie{Name: stateCookieName, Value: state, HTTPOnly: true, Secure: h.cookieSecure, SameSite: "Lax", Expires: time.Now().Add(10 * time.Minute)})
	c.Cookie(&fiber.Cookie{Name: verifierCookieName, Value: verifier, HTTPOnly: true, Secure: h.cookieSecure, SameSite: "Lax", Expires: time.Now().Add(10 * time.Minute)})

	return c.Redirect(h.google.AuthCodeURL(state, challenge), fiber.StatusFound)
}

func (h *Handlers) GoogleCallback(c *fiber.Ctx) error {
	if !h.google.Enabled() {
		return errJSON(c, fiber.StatusServiceUnavailable, "google_oauth_disabled", "Google sign-in is not configured on this server.")
	}
	state := c.Query("state")
	code := c.Query("code")
	cookieState := c.Cookies(stateCookieName)
	verifier := c.Cookies(verifierCookieName)

	if state == "" || cookieState == "" || !ValidCSRF(cookieState, state) {
		return errJSON(c, fiber.StatusBadRequest, "invalid_state", "OAuth state validation failed.")
	}
	if code == "" || verifier == "" {
		return errJSON(c, fiber.StatusBadRequest, "invalid_request", "Missing authorization code.")
	}

	token, err := h.google.Exchange(c.Context(), code, verifier)
	if err != nil {
		return errJSON(c, fiber.StatusBadGateway, "google_exchange_failed", "Could not complete Google sign-in.")
	}
	info, err := h.google.FetchUserInfo(c.Context(), token)
	if err != nil {
		return errJSON(c, fiber.StatusBadGateway, "google_userinfo_failed", "Could not fetch Google profile.")
	}

	user, err := h.svc.LoginOrRegisterWithGoogle(c.Context(), info)
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not sign in with Google.")
	}

	tp, err := h.svc.IssueTokenPair(c.Context(), user.ID, c.IP(), c.Get("User-Agent"))
	if err != nil {
		return errJSON(c, fiber.StatusInternalServerError, "internal_error", "Could not create session.")
	}
	h.setSessionCookies(c, tp)
	h.audit.Log(c.Context(), audit.Entry{UserID: &user.ID, Action: "user.logged_in_google", ResourceType: "user", ResourceID: user.ID, IPAddress: c.IP(), UserAgent: c.Get("User-Agent")})

	// Hand the access token to the SPA via a one-time URL fragment (never logged, never sent to a server).
	return c.Redirect(h.svc.frontendOrigin+"/oauth-callback.html#access_token="+tp.AccessToken, fiber.StatusFound)
}
