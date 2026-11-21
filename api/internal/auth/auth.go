// Package auth handles passwords, JWT access tokens, refresh tokens and
// the HTTP middleware that authenticates and authorizes requests.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"ticketing/api/internal/domain"
)

// BcryptCost is a variable so tests can lower it for speed.
var BcryptCost = 12

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NewRefreshToken returns a random token for the client and its hash for storage.
func NewRefreshToken() (raw, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken hashes a refresh token; only the hash is stored in the DB.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type claims struct {
	Role domain.Role `json:"role"`
	Name string      `json:"name"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies HS256 JWT access tokens.
type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), ttl: ttl}
}

func (m *TokenManager) Issue(u domain.User) (string, error) {
	now := time.Now()
	c := claims{
		Role: u.Role,
		Name: u.FullName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
}

func (m *TokenManager) Parse(token string) (domain.Actor, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return domain.Actor{}, fmt.Errorf("invalid token: %w", err)
	}
	if c.Subject == "" || c.Role == "" {
		return domain.Actor{}, errors.New("invalid token: missing claims")
	}
	return domain.Actor{ID: c.Subject, Role: c.Role, Name: c.Name}, nil
}
