// Package httpapi wires HTTP routes, middleware and handlers.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
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
	r.Use(timeoutExceptStreams(30 * time.Second))
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
	health := healthHandler(d.HealthCheck)
	r.Get("/healthz", health)
	r.Get("/healthcheck", health)
	r.Get("/api/v1/healthcheck", health) // reachable through the web dev proxy too

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
			r.With(auth.RequireRole(domain.RoleAdmin, domain.RoleAgent)).Get("/reports/export", ticketH.Export)
			r.Get("/notifications", ticketH.Notifications)
			r.Post("/notifications/read", ticketH.MarkNotificationsRead)

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
					r.Get("/chat", ticketH.ChatThread)
					r.Post("/chat", ticketH.SendChat)
					r.Get("/chat/stream", ticketH.ChatStream)
				})
			})

			r.Route("/agents", func(r chi.Router) {
				// Staff can list agents (for the reassign dropdown); only admins create them.
				r.With(auth.RequireRole(domain.RoleAdmin, domain.RoleAgent)).Get("/", ticketH.ListAgents)
				r.With(auth.RequireRole(domain.RoleAdmin)).Post("/", authH.CreateAgent)
				r.With(auth.RequireRole(domain.RoleAdmin)).Delete("/{id}", ticketH.RemoveAgent)
			})
		})
	})
	return r
}

var startedAt = time.Now()

// healthHandler reports API and database health (200 when healthy, 503 when the DB is down).
func healthHandler(check func(ctx context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{"status": "ok", "db": "up", "uptimeSeconds": int(time.Since(startedAt).Seconds())}
		if check != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			start := time.Now()
			if err := check(ctx); err != nil {
				body["status"], body["db"] = "degraded", "down"
				httpx.WriteJSON(w, http.StatusServiceUnavailable, body)
				return
			}
			body["dbLatencyMs"] = time.Since(start).Milliseconds()
		}
		httpx.WriteJSON(w, http.StatusOK, body)
	}
}

// timeoutExceptStreams applies a request timeout to everything except
// long-lived Server-Sent Events streams.
func timeoutExceptStreams(d time.Duration) func(http.Handler) http.Handler {
	timeout := middleware.Timeout(d)
	return func(next http.Handler) http.Handler {
		withTimeout := timeout(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/chat/stream") {
				next.ServeHTTP(w, r)
				return
			}
			withTimeout.ServeHTTP(w, r)
		})
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
