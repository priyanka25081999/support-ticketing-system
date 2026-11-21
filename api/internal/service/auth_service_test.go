package service

import (
	"context"
	"testing"
	"time"

	"ticketing/api/internal/auth"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/repo/memrepo"
)

func newAuthService() *AuthService {
	return NewAuthService(memrepo.New(), auth.NewTokenManager("test-secret-test-secret-test-secret", time.Minute), time.Hour)
}

func TestRegisterAndLogin(t *testing.T) {
	svc := newAuthService()
	ctx := context.Background()

	user, err := svc.Register(ctx, RegisterInput{Email: " Jane@Example.com ", Password: "password1", FullName: "Jane"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if user.Email != "jane@example.com" || user.Role != domain.RoleCustomer {
		t.Fatalf("unexpected user: %+v", user)
	}

	_, err = svc.Register(ctx, RegisterInput{Email: "jane@example.com", Password: "password1", FullName: "Jane"})
	assertCode(t, err, "CONFLICT")

	_, err = svc.Register(ctx, RegisterInput{Email: "x@example.com", Password: "short", FullName: "X"})
	assertCode(t, err, "VALIDATION_ERROR")

	_, err = svc.Login(ctx, "jane@example.com", "wrong-password")
	assertCode(t, err, "UNAUTHORIZED")

	res, err := svc.Login(ctx, "JANE@example.com", "password1")
	if err != nil || res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("login failed: %v", err)
	}
}

func TestRefresh_RotatesAndRevokesOldToken(t *testing.T) {
	svc := newAuthService()
	ctx := context.Background()
	svc.Register(ctx, RegisterInput{Email: "a@example.com", Password: "password1", FullName: "A"})
	first, _ := svc.Login(ctx, "a@example.com", "password1")

	second, err := svc.Refresh(ctx, first.RefreshToken)
	if err != nil || second.RefreshToken == first.RefreshToken {
		t.Fatalf("refresh should rotate the token: %v", err)
	}
	_, err = svc.Refresh(ctx, first.RefreshToken)
	assertCode(t, err, "UNAUTHORIZED")

	if err := svc.Logout(ctx, second.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	_, err = svc.Refresh(ctx, second.RefreshToken)
	assertCode(t, err, "UNAUTHORIZED")
}

func TestCreateAgent_AdminOnly(t *testing.T) {
	svc := newAuthService()
	ctx := context.Background()
	in := RegisterInput{Email: "agent@example.com", Password: "password1", FullName: "Agent"}

	_, err := svc.CreateAgent(ctx, domain.Actor{Role: domain.RoleCustomer}, in)
	assertCode(t, err, "FORBIDDEN")

	agent, err := svc.CreateAgent(ctx, domain.Actor{Role: domain.RoleAdmin}, in)
	if err != nil || agent.Role != domain.RoleAgent {
		t.Fatalf("create agent: %v %+v", err, agent)
	}
}
