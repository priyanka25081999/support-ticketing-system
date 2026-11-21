package httpapi

import (
	"net/http"
	"time"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/auth"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/httpx"
	"ticketing/api/internal/service"
)

const refreshCookieName = "refresh_token"

// AuthHandler serves /auth/*, /me and /agents.
type AuthHandler struct {
	svc          *service.AuthService
	refreshTTL   time.Duration
	cookieSecure bool
}

type authResponse struct {
	AccessToken string      `json:"accessToken"`
	User        domain.User `json:"user"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var in service.RegisterInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	user, err := h.svc.Register(r.Context(), in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	res, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.setRefreshCookie(w, res.RefreshToken, h.refreshTTL)
	httpx.WriteJSON(w, http.StatusOK, authResponse{AccessToken: res.AccessToken, User: res.User})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Refresh(r.Context(), readCookie(r))
	if err != nil {
		h.setRefreshCookie(w, "", -1) // clear an invalid cookie
		httpx.WriteError(w, err)
		return
	}
	h.setRefreshCookie(w, res.RefreshToken, h.refreshTTL)
	httpx.WriteJSON(w, http.StatusOK, authResponse{AccessToken: res.AccessToken, User: res.User})
}

// Logout works with only the cookie so users can log out after the access token expires.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(r.Context(), readCookie(r)); err != nil {
		httpx.WriteError(w, err)
		return
	}
	h.setRefreshCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	actor := mustActor(r)
	user, err := h.svc.Me(r.Context(), actor.ID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var in service.RegisterInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	user, err := h.svc.CreateAgent(r.Context(), mustActor(r), in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, user)
}

// setRefreshCookie stores the refresh token in an HttpOnly cookie scoped to /api/v1/auth.
// A negative maxAge deletes the cookie.
func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, value string, maxAge time.Duration) {
	cookie := &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(maxAge.Seconds()),
	}
	if maxAge < 0 {
		cookie.MaxAge = -1
	}
	http.SetCookie(w, cookie)
}

func readCookie(r *http.Request) string {
	c, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// mustActor returns the actor set by the auth middleware (always present on protected routes).
func mustActor(r *http.Request) domain.Actor {
	actor, ok := auth.ActorFrom(r.Context())
	if !ok {
		panic(apperr.Unauthorized("missing actor")) // programming error: route not protected
	}
	return actor
}
