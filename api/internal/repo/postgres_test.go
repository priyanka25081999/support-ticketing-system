package repo

import (
	"reflect"
	"testing"

	"ticketing/api/internal/domain"
)

func TestBuildWhere(t *testing.T) {
	where, args := buildWhere(domain.TicketFilter{RequesterID: "u1", Status: []string{"open", "on_hold"}, Search: "50%_off"})

	wantWhere := " WHERE t.requester_id = $1::uuid AND t.status::text = ANY($2::text[]) AND " +
		"(t.subject ILIKE $3 OR t.ticket_number ILIKE $3 OR replace(t.type::text, '_', ' ') ILIKE $3)"
	if where != wantWhere {
		t.Fatalf("where:\n got %q\nwant %q", where, wantWhere)
	}
	wantArgs := []any{"u1", []string{"open", "on_hold"}, `%50\%\_off%`}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args: got %v want %v", args, wantArgs)
	}
}

func TestBuildWhere_Empty(t *testing.T) {
	if where, args := buildWhere(domain.TicketFilter{}); where != "" || len(args) != 0 {
		t.Fatalf("expected empty where, got %q %v", where, args)
	}
}
