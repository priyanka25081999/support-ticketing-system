package service

import (
	"fmt"
	"slices"
	"time"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
)

// ReopenWindow is how long a customer can reopen a solved ticket.
// Solved tickets older than this are auto-closed.
const ReopenWindow = 7 * 24 * time.Hour

// transitions is the ticket state machine (README §4).
var transitions = map[domain.Status][]domain.Status{
	domain.StatusOpen:       {domain.StatusInProgress, domain.StatusOnHold, domain.StatusSolved},
	domain.StatusInProgress: {domain.StatusOpen, domain.StatusOnHold, domain.StatusSolved},
	domain.StatusOnHold:     {domain.StatusInProgress, domain.StatusOpen, domain.StatusSolved},
	domain.StatusSolved:     {domain.StatusOpen, domain.StatusClosed},
	domain.StatusClosed:     {}, // final
}

// CheckTransition validates a status change for the given role.
//   - customer: may only reopen their solved ticket within ReopenWindow
//   - agent:    any valid transition except closing
//   - admin:    any valid transition
func CheckTransition(role domain.Role, from, to domain.Status, solvedAt *time.Time, now time.Time) error {
	if from == to {
		return apperr.InvalidTransition(fmt.Sprintf("Ticket is already %s", to))
	}
	if !slices.Contains(transitions[from], to) {
		return apperr.InvalidTransition(fmt.Sprintf("Cannot change status from %s to %s", from, to))
	}

	switch role {
	case domain.RoleCustomer:
		if from != domain.StatusSolved || to != domain.StatusOpen {
			return apperr.Forbidden("Customers can only reopen solved tickets")
		}
		if solvedAt != nil && now.Sub(*solvedAt) > ReopenWindow {
			return apperr.InvalidTransition("The reopen window has expired; please create a new ticket and link this one")
		}
	case domain.RoleAgent:
		if to == domain.StatusClosed {
			return apperr.Forbidden("Only admins can close tickets")
		}
	case domain.RoleAdmin:
		// admins may perform any valid transition
	default:
		return apperr.Forbidden("Unknown role")
	}
	return nil
}
