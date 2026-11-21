// Package config loads application settings from environment variables.
// Values are read from a .env file (current dir or repo root) when present;
// real environment variables always take precedence.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime settings for the API.
type Config struct {
	Port            string
	DatabaseURL     string
	MigrationsDir   string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CORSOrigin      string
	CookieSecure    bool
	AdminEmail      string
	AdminPassword   string
	AdminName       string
	// Optional TLS files. When both are set the API serves HTTPS, which also
	// enables HTTP/2 (browsers only use HTTP/2 over TLS).
	TLSCertFile string
	TLSKeyFile  string
}

// Load reads the configuration and validates required values.
func Load() (*Config, error) {
	// godotenv never overrides variables that are already set.
	for _, file := range []string{".env", "../.env"} {
		_ = godotenv.Load(file)
	}

	accessTTL, err := durationEnv("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refreshTTL, err := durationEnv("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:            getEnv("API_PORT", "8080"),
		MigrationsDir:   getEnv("MIGRATIONS_DIR", "../db/migrations"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		CORSOrigin:      getEnv("CORS_ORIGIN", "http://localhost:5180"),
		CookieSecure:    getEnv("COOKIE_SECURE", "false") == "true",
		AdminEmail:      os.Getenv("ADMIN_EMAIL"),
		AdminPassword:   os.Getenv("ADMIN_PASSWORD"),
		AdminName:       getEnv("ADMIN_NAME", "Default Admin"),
		TLSCertFile:     os.Getenv("TLS_CERT_FILE"),
		TLSKeyFile:      os.Getenv("TLS_KEY_FILE"),
		DatabaseURL: BuildDatabaseURL(
			os.Getenv("DB_HOST"), getEnv("DB_PORT", "5432"),
			os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"), os.Getenv("DB_SSL"),
		),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	var missing []string
	for _, key := range []string{"DB_HOST", "DB_USER", "DB_NAME", "JWT_SECRET", "ADMIN_EMAIL", "ADMIN_PASSWORD"} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if len(c.AdminPassword) < 8 {
		return fmt.Errorf("ADMIN_PASSWORD must be at least 8 characters")
	}
	return nil
}

// BuildDatabaseURL builds a postgres URL; credentials are URL-escaped so
// passwords with special characters work.
func BuildDatabaseURL(host, port, user, password, name, ssl string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + name,
		RawQuery: url.Values{"sslmode": {SSLMode(ssl)}}.Encode(),
	}
	return u.String()
}

// SSLMode converts DB_SSL values (true/false/require/...) into a libpq sslmode.
func SSLMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false", "disable", "off":
		return "disable"
	case "true", "on", "require":
		return "require"
	default:
		return value // e.g. verify-full, prefer
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return d, nil
}
