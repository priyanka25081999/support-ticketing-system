package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ticketing/api/internal/domain"
)

const secret = "test-secret-test-secret-test-secret"

func TestTokenRoundTrip(t *testing.T) {
	tm := NewTokenManager(secret, time.Minute)
	token, err := tm.Issue(domain.User{ID: "u1", Role: domain.RoleAdmin, FullName: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	actor, err := tm.Parse(token)
	if err != nil || actor.ID != "u1" || actor.Role != domain.RoleAdmin || actor.Name != "Ada" {
		t.Fatalf("unexpected actor %+v (%v)", actor, err)
	}
}

func TestParseRejectsBadTokens(t *testing.T) {
	token, _ := NewTokenManager(secret, time.Minute).Issue(domain.User{ID: "u1", Role: domain.RoleAdmin})
	if _, err := NewTokenManager("another-secret-another-secret-xx", time.Minute).Parse(token); err == nil {
		t.Fatal("token signed with a different secret must be rejected")
	}
	expired, _ := NewTokenManager(secret, -time.Minute).Issue(domain.User{ID: "u1", Role: domain.RoleAdmin})
	if _, err := NewTokenManager(secret, time.Minute).Parse(expired); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestMiddleware(t *testing.T) {
	tm := NewTokenManager(secret, time.Minute)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := Authenticate(tm)(RequireRole(domain.RoleAdmin)(ok))

	call := func(header string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	adminToken, _ := tm.Issue(domain.User{ID: "a", Role: domain.RoleAdmin})
	customerToken, _ := tm.Issue(domain.User{ID: "c", Role: domain.RoleCustomer})

	if got := call(""); got != http.StatusUnauthorized {
		t.Errorf("no token: got %d", got)
	}
	if got := call("Bearer garbage"); got != http.StatusUnauthorized {
		t.Errorf("bad token: got %d", got)
	}
	if got := call("Bearer " + customerToken); got != http.StatusForbidden {
		t.Errorf("wrong role: got %d", got)
	}
	if got := call("Bearer " + adminToken); got != http.StatusOK {
		t.Errorf("admin: got %d", got)
	}
}

func TestRefreshTokenHashing(t *testing.T) {
	raw, hash, err := NewRefreshToken()
	if err != nil || raw == "" || hash != HashToken(raw) || hash == raw {
		t.Fatalf("unexpected refresh token output: %v", err)
	}
}
