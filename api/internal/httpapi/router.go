// Package httpapi wires HTTP routes, middleware and handlers.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/auth"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/httpx"
	"ticketing/api/internal/service"
)

// Deps are the dependencies needed to build the router.
type Deps struct {
	AuthService   *service.AuthService
	TicketService *service.TicketService
	Tokens        *auth.TokenManager
	CORSOrigin    string
	RefreshTTL    time.Duration
	CookieSecure  bool
	// HealthCheck reports database health; nil means always healthy.
	HealthCheck func(ctx context.Context) error
}

func NewRouter(d Deps) http.Handler {
	authH := &AuthHandler{svc: d.AuthService, refreshTTL: d.RefreshTTL, cookieSecure: d.CookieSecure}
	ticketH := &TicketHandler{svc: d.TicketService}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, recoverJSON)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{d.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, apperr.NotFound("Route not found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, apperr.New(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed"))
	})
	r.Get("/healthz", healthHandler(d.HealthCheck))

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Use(httprate.LimitByIP(30, time.Minute)) // brute-force protection
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)
			r.Post("/refresh", authH.Refresh)
			r.Post("/logout", authH.Logout)
		})

		// Everything below requires a valid access token.
		r.Group(func(r chi.Router) {
			r.Use(auth.Authenticate(d.Tokens))

			r.Get("/me", authH.Me)
			r.Get("/meta/enums", ticketH.Enums)
			r.Get("/reports/summary", ticketH.Summary)

			r.Route("/tickets", func(r chi.Router) {
				r.With(auth.RequireRole(domain.RoleCustomer)).Post("/", ticketH.Create)
				r.Get("/", ticketH.List)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", ticketH.Get)
					r.Get("/timeline", ticketH.Timeline)
					r.Post("/messages", ticketH.AddMessage)
					r.Patch("/status", ticketH.ChangeStatus)
					r.With(auth.RequireRole(domain.RoleAdmin, domain.RoleAgent)).Patch("/assignee", ticketH.Assign)
					r.Post("/links", ticketH.AddLink)
					r.Delete("/links/{linkedId}", ticketH.RemoveLink)
					r.Post("/attachments", ticketH.UploadAttachment)
					r.Get("/attachments/{attachmentId}", ticketH.DownloadAttachment)
				})
			})

			r.Route("/agents", func(r chi.Router) {
				// Staff can list agents (for the reassign dropdown); only admins create them.
				r.With(auth.RequireRole(domain.RoleAdmin, domain.RoleAgent)).Get("/", ticketH.ListAgents)
				r.With(auth.RequireRole(domain.RoleAdmin)).Post("/", authH.CreateAgent)
			})
		})
	})
	return r
}

func healthHandler(check func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := check(ctx); err != nil {
				httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "db": "down"})
				return
			}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "up"})
	}
}

// recoverJSON turns panics into a JSON 500 instead of dropping the connection.
func recoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				slog.Error("panic recovered", "panic", rec, "stack", string(debug.Stack()))
				httpx.WriteError(w, apperr.New(http.StatusInternalServerError, "INTERNAL", "Something went wrong. Please try again."))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
