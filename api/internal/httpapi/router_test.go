package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ticketing/api/internal/auth"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/repo/memrepo"
	"ticketing/api/internal/service"
)

// newTestServer builds the full router on top of the in-memory repo.
func newTestServer(t *testing.T) (*httptest.Server, *auth.TokenManager, *memrepo.Repo) {
	t.Helper()
	auth.BcryptCost = 4
	r := memrepo.New()
	tokens := auth.NewTokenManager("test-secret-test-secret-test-secret", time.Minute)
	router := NewRouter(Deps{
		AuthService:   service.NewAuthService(r, tokens, time.Hour),
		TicketService: service.NewTicketService(r),
		Tokens:        tokens,
		CORSOrigin:    "http://localhost:5180",
		RefreshTTL:    time.Hour,
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, tokens, r
}

func doJSON(t *testing.T, method, url, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

func TestErrorResponsesUseStandardShape(t *testing.T) {
	srv, _, _ := newTestServer(t)

	res, body := doJSON(t, http.MethodGet, srv.URL+"/api/v1/tickets", "", nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok || errObj["code"] != "UNAUTHORIZED" || errObj["message"] == "" {
		t.Fatalf("unexpected error body: %v", body)
	}

	res, _ = doJSON(t, http.MethodGet, srv.URL+"/does-not-exist", "", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}
}

func TestLoginSetsHttpOnlyRefreshCookie(t *testing.T) {
	srv, _, _ := newTestServer(t)
	doJSON(t, http.MethodPost, srv.URL+"/api/v1/auth/register", "",
		map[string]string{"email": "c@example.com", "password": "password1", "fullName": "C"})

	res, body := doJSON(t, http.MethodPost, srv.URL+"/api/v1/auth/login", "",
		map[string]string{"email": "c@example.com", "password": "password1"})
	if res.StatusCode != http.StatusOK || body["accessToken"] == "" {
		t.Fatalf("login failed: %d %v", res.StatusCode, body)
	}
	var cookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == refreshCookieName {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.Value == "" {
		t.Fatalf("expected HttpOnly refresh cookie, got %+v", cookie)
	}
}

func TestCreateAndListTickets(t *testing.T) {
	srv, tokens, r := newTestServer(t)
	admin := r.AddUser(domain.User{Email: "admin@x.com", FullName: "Admin", Role: domain.RoleAdmin}, true)
	customer := r.AddUser(domain.User{Email: "c@x.com", FullName: "Cust", Role: domain.RoleCustomer}, false)
	adminToken, _ := tokens.Issue(admin)
	customerToken, _ := tokens.Issue(customer)

	ticket := map[string]any{"subject": "Cannot login", "description": "500 error", "type": "technical", "severity": "high", "notifyEmails": "c@x.com; boss@x.com"}

	res, _ := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tickets", adminToken, ticket)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("admin create: expected 403, got %d", res.StatusCode)
	}

	res, body := doJSON(t, http.MethodPost, srv.URL+"/api/v1/tickets", customerToken, ticket)
	if res.StatusCode != http.StatusCreated || body["ticketNumber"] != "TKT-000001" {
		t.Fatalf("create: %d %v", res.StatusCode, body)
	}

	res, body = doJSON(t, http.MethodGet, srv.URL+"/api/v1/tickets?status=open&severity=high", adminToken, nil)
	if res.StatusCode != http.StatusOK || body["total"].(float64) != 1 {
		t.Fatalf("list: %d %v", res.StatusCode, body)
	}

	res, _ = doJSON(t, http.MethodGet, srv.URL+"/api/v1/tickets?status=bogus", adminToken, nil)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid filter: expected 422, got %d", res.StatusCode)
	}
}
