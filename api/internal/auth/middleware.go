package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/httpx"
)

type ctxKey struct{}

// WithActor stores the authenticated actor in the context.
func WithActor(ctx context.Context, a domain.Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// ActorFrom returns the authenticated actor set by Authenticate.
func ActorFrom(ctx context.Context) (domain.Actor, bool) {
	a, ok := ctx.Value(ctxKey{}).(domain.Actor)
	return a, ok
}

// Authenticate requires a valid "Authorization: Bearer <token>" header.
func Authenticate(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				httpx.WriteError(w, apperr.Unauthorized("Missing access token"))
				return
			}
			actor, err := tm.Parse(token)
			if err != nil {
				httpx.WriteError(w, apperr.Unauthorized("Invalid or expired access token"))
				return
			}
			next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), actor)))
		})
	}
}

// RequireRole allows the request only if the actor has one of the roles.
func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ActorFrom(r.Context())
			if !ok {
				httpx.WriteError(w, apperr.Unauthorized("Not authenticated"))
				return
			}
			if !slices.Contains(roles, actor.Role) {
				httpx.WriteError(w, apperr.Forbidden("You do not have permission for this action"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
