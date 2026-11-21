package httpapi

import (
	"strings"
	"testing"
	"time"

	"ticketing/api/internal/domain"
)

func TestWriteTicketsCSV_EscapesFormulas(t *testing.T) {
	var b strings.Builder
	created := time.Date(2026, 11, 21, 7, 0, 0, 0, time.UTC)
	err := writeTicketsCSV(&b, []domain.Ticket{{
		TicketNumber: "TKT-000001", Subject: "=HYPERLINK(\"x\")", Type: "billing", Severity: "high", Status: "open",
		Requester: domain.UserRef{Name: "Ann"}, CreatedAt: created, UpdatedAt: created,
	}}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], `"'=HYPERLINK(""x"")"`) || !strings.Contains(lines[1], "2026-11-21 07:00:00 UTC") {
		t.Fatalf("unexpected csv:\n%s", b.String())
	}
}
