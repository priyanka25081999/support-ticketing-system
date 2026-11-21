package repo

import (
	"reflect"
	"testing"

	"ticketing/api/internal/domain"
)

func TestBuildWhere(t *testing.T) {
	where, args := buildWhere(domain.TicketFilter{RequesterID: "u1", Status: "open", Search: "50%_off"})

	wantWhere := " WHERE t.requester_id = $1::uuid AND t.status = $2::ticket_status AND (t.subject ILIKE $3 OR t.ticket_number ILIKE $3)"
	if where != wantWhere {
		t.Fatalf("where:\n got %q\nwant %q", where, wantWhere)
	}
	wantArgs := []any{"u1", "open", `%50\%\_off%`}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args: got %v want %v", args, wantArgs)
	}
}

func TestBuildWhere_Empty(t *testing.T) {
	if where, args := buildWhere(domain.TicketFilter{}); where != "" || len(args) != 0 {
		t.Fatalf("expected empty where, got %q %v", where, args)
	}
}
