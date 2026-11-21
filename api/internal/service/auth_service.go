package service

import (
	"context"
	"errors"
	"time"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/auth"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/repo"
)

// AuthService handles registration, login and token rotation.
type AuthService struct {
	repo       repo.Repository
	tokens     *auth.TokenManager
	refreshTTL time.Duration
	now        func() time.Time
}

func NewAuthService(r repo.Repository, tokens *auth.TokenManager, refreshTTL time.Duration) *AuthService {
	return &AuthService{repo: r, tokens: tokens, refreshTTL: refreshTTL, now: time.Now}
}

// AuthResult is returned on login/refresh. RefreshToken goes into an HttpOnly cookie.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	User         domain.User
}

type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"fullName"`
}

// Register creates a customer account.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (domain.User, error) {
	return s.createUser(ctx, in, domain.RoleCustomer)
}

// CreateAgent lets an admin add a new agent account.
func (s *AuthService) CreateAgent(ctx context.Context, actor domain.Actor, in RegisterInput) (domain.User, error) {
	if actor.Role != domain.RoleAdmin {
		return domain.User{}, apperr.Forbidden("Only admins can create agents")
	}
	return s.createUser(ctx, in, domain.RoleAgent)
}

func (s *AuthService) createUser(ctx context.Context, in RegisterInput, role domain.Role) (domain.User, error) {
	email := normalizeEmail(in.Email)
	if err := validateEmail(email); err != nil {
		return domain.User{}, err
	}
	if err := validatePassword(in.Password); err != nil {
		return domain.User{}, err
	}
	name, err := requireLength("Full name", in.FullName, 1, maxNameLen)
	if err != nil {
		return domain.User{}, err
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return domain.User{}, err
	}
	user, err := s.repo.CreateUser(ctx, domain.User{Email: email, PasswordHash: hash, FullName: name, Role: role})
	if errors.Is(err, repo.ErrDuplicate) {
		return domain.User{}, apperr.Conflict("An account with this email already exists")
	}
	return user, err
}

func (s *AuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, repo.ErrNotFound) || (err == nil && !auth.CheckPassword(user.PasswordHash, password)) {
		return AuthResult{}, apperr.Unauthorized("Invalid email or password")
	}
	if err != nil {
		return AuthResult{}, err
	}
	return s.issueTokens(ctx, user)
}

// Refresh rotates the refresh token: the old one is revoked and a new pair is issued.
func (s *AuthService) Refresh(ctx context.Context, rawToken string) (AuthResult, error) {
	if rawToken == "" {
		return AuthResult{}, apperr.Unauthorized("Missing refresh token")
	}
	stored, err := s.repo.GetRefreshToken(ctx, auth.HashToken(rawToken))
	if errors.Is(err, repo.ErrNotFound) {
		return AuthResult{}, apperr.Unauthorized("Invalid refresh token")
	}
	if err != nil {
		return AuthResult{}, err
	}
	if stored.Revoked || s.now().After(stored.ExpiresAt) {
		return AuthResult{}, apperr.Unauthorized("Session expired, please log in again")
	}

	user, err := s.repo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return AuthResult{}, err
	}
	var result AuthResult
	err = s.repo.InTx(ctx, func(tx repo.Repository) error {
		if err := tx.RevokeRefreshToken(ctx, stored.ID); err != nil {
			return err
		}
		result, err = s.issueTokensWith(ctx, tx, user)
		return err
	})
	return result, err
}

// Logout revokes the refresh token. Unknown tokens are ignored.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	stored, err := s.repo.GetRefreshToken(ctx, auth.HashToken(rawToken))
	if errors.Is(err, repo.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.repo.RevokeRefreshToken(ctx, stored.ID)
}

func (s *AuthService) Me(ctx context.Context, userID string) (domain.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if errors.Is(err, repo.ErrNotFound) {
		return domain.User{}, apperr.Unauthorized("User no longer exists")
	}
	return user, err
}

// EnsureDefaultAdmin seeds the default admin at startup.
func (s *AuthService) EnsureDefaultAdmin(ctx context.Context, email, password, name string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.EnsureDefaultAdmin(ctx, normalizeEmail(email), hash, name)
}

func (s *AuthService) issueTokens(ctx context.Context, user domain.User) (AuthResult, error) {
	return s.issueTokensWith(ctx, s.repo, user)
}

func (s *AuthService) issueTokensWith(ctx context.Context, r repo.Repository, user domain.User) (AuthResult, error) {
	access, err := s.tokens.Issue(user)
	if err != nil {
		return AuthResult{}, err
	}
	raw, hash, err := auth.NewRefreshToken()
	if err != nil {
		return AuthResult{}, err
	}
	if err := r.CreateRefreshToken(ctx, user.ID, hash, s.now().Add(s.refreshTTL)); err != nil {
		return AuthResult{}, err
	}
	return AuthResult{AccessToken: access, RefreshToken: raw, User: user}, nil
}
