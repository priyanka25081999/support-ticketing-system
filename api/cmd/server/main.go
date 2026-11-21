// Command server starts the ticketing REST API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embedded zone data so report timezones work in any container

	"ticketing/api/internal/auth"
	"ticketing/api/internal/config"
	"ticketing/api/internal/database"
	"ticketing/api/internal/httpapi"
	"ticketing/api/internal/repo"
	"ticketing/api/internal/service"
)

const autoCloseInterval = time.Hour

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Database layer: connect, then apply migrations.
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := database.Migrate(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return err
	}
	slog.Info("database ready, migrations applied")

	// Service layer.
	store := repo.NewPostgres(pool)
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	authSvc := service.NewAuthService(store, tokens, cfg.RefreshTokenTTL)
	ticketSvc := service.NewTicketService(store)

	if err := authSvc.EnsureDefaultAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword, cfg.AdminName); err != nil {
		return err
	}
	go runAutoClose(ctx, ticketSvc)

	// HTTP layer.
	srv := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(httpapi.Deps{
			AuthService:   authSvc,
			TicketService: ticketSvc,
			Tokens:        tokens,
			CORSOrigin:    cfg.CORSOrigin,
			RefreshTTL:    cfg.RefreshTokenTTL,
			CookieSecure:  cfg.CookieSecure,
			HealthCheck:   pool.Ping,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			// net/http negotiates HTTP/2 automatically over TLS (ALPN "h2").
			slog.Info("API listening (HTTPS, HTTP/2 enabled)", "addr", srv.Addr)
			err = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			slog.Info("API listening (HTTP/1.1; set TLS_CERT_FILE/TLS_KEY_FILE for HTTP/2)", "addr", srv.Addr)
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// runAutoClose closes solved tickets after the reopen window (on start, then hourly).
func runAutoClose(ctx context.Context, svc *service.TicketService) {
	ticker := time.NewTicker(autoCloseInterval)
	defer ticker.Stop()
	for {
		if n, err := svc.AutoClose(ctx); err != nil {
			slog.Error("auto-close failed", "err", err)
		} else if n > 0 {
			slog.Info("auto-closed solved tickets", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
