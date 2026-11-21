package service

import (
	"errors"
	"testing"
	"time"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
)

func TestCheckTransition(t *testing.T) {
	now := time.Now()
	recent := now.Add(-24 * time.Hour)
	expired := now.Add(-8 * 24 * time.Hour)

	tests := []struct {
		name     string
		role     domain.Role
		from, to domain.Status
		solvedAt *time.Time
		wantCode string // "" means allowed
	}{
		{"agent open->pending", domain.RoleAgent, "open", "pending", nil, ""},
		{"agent pending->solved", domain.RoleAgent, "pending", "solved", nil, ""},
		{"agent cannot close", domain.RoleAgent, "solved", "closed", &recent, "FORBIDDEN"},
		{"admin closes solved", domain.RoleAdmin, "solved", "closed", &recent, ""},
		{"same status rejected", domain.RoleAdmin, "open", "open", nil, "INVALID_TRANSITION"},
		{"open cannot jump to closed", domain.RoleAdmin, "open", "closed", nil, "INVALID_TRANSITION"},
		{"closed is final", domain.RoleAdmin, "closed", "open", nil, "INVALID_TRANSITION"},
		{"customer reopens within window", domain.RoleCustomer, "solved", "open", &recent, ""},
		{"customer reopen window expired", domain.RoleCustomer, "solved", "open", &expired, "INVALID_TRANSITION"},
		{"customer cannot solve", domain.RoleCustomer, "open", "solved", nil, "FORBIDDEN"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckTransition(tc.role, tc.from, tc.to, tc.solvedAt, now)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("expected allowed, got %v", err)
				}
				return
			}
			var appErr *apperr.Error
			if !errors.As(err, &appErr) || appErr.Code != tc.wantCode {
				t.Fatalf("expected code %s, got %v", tc.wantCode, err)
			}
		})
	}
}
