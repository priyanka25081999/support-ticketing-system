package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/repo"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// TicketService holds all ticket business rules: access control,
// auto-assignment, status transitions and link validation.
type TicketService struct {
	repo repo.Repository
	now  func() time.Time
	chat *chatHub // in-process pub/sub for live chat streams
}

func NewTicketService(r repo.Repository) *TicketService {
	return &TicketService{repo: r, now: time.Now, chat: newChatHub()}
}

type CreateTicketInput struct {
	Subject         string   `json:"subject"`
	Description     string   `json:"description"`
	Type            string   `json:"type"`
	Severity        string   `json:"severity"`
	LinkedTicketIDs []string `json:"linkedTicketIds"`
	// NotifyEmails is a ";"-separated list, e.g. "a@x.com; b@y.com".
	NotifyEmails string `json:"notifyEmails"`
}

// Create opens a ticket for a customer and auto-assigns it to the default admin.
func (s *TicketService) Create(ctx context.Context, actor domain.Actor, in CreateTicketInput) (domain.TicketDetail, error) {
	if actor.Role != domain.RoleCustomer {
		return domain.TicketDetail{}, apperr.Forbidden("Only customers can create tickets")
	}
	subject, err := requireLength("Subject", in.Subject, minSubjectLen, maxSubjectLen)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	description, err := requireLength("Description", in.Description, 1, maxDescriptionLen)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	if !domain.IsOneOf(in.Type, domain.Types) {
		return domain.TicketDetail{}, apperr.Validation("Please choose a valid ticket type")
	}
	severity := cmp.Or(in.Severity, "medium")
	if !domain.IsOneOf(severity, domain.Severities) {
		return domain.TicketDetail{}, apperr.Validation("Please choose a valid severity")
	}
	notifyEmails, err := ParseEmailList(in.NotifyEmails)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	if len(in.LinkedTicketIDs) > maxLinksPerCreate {
		return domain.TicketDetail{}, apperr.Validation("Too many linked tickets")
	}

	var ticketID string
	err = s.repo.InTx(ctx, func(tx repo.Repository) error {
		assignee, err := tx.GetDefaultAssignee(ctx)
		if err != nil {
			return errors.Join(errors.New("no default assignee configured"), err)
		}
		ticketID, err = tx.CreateTicket(ctx, repo.NewTicket{
			Subject: subject, Description: description, Type: in.Type, Severity: severity,
			RequesterID: actor.ID, AssigneeID: assignee.ID, NotifyEmails: notifyEmails,
		})
		if err != nil {
			return err
		}
		if err := tx.CreateEvent(ctx, ticketID, actor.ID, domain.EventCreated, nil); err != nil {
			return err
		}
		if err := tx.CreateEvent(ctx, ticketID, actor.ID, domain.EventAssigned,
			map[string]any{"assigneeId": assignee.ID, "assigneeName": assignee.FullName}); err != nil {
			return err
		}
		for _, linkedID := range slices.Compact(slices.Sorted(slices.Values(in.LinkedTicketIDs))) {
			if err := s.link(ctx, tx, actor, ticketID, actor.ID, linkedID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.TicketDetail{}, err
	}
	return s.Get(ctx, actor, ticketID)
}

// List returns tickets visible to the actor: customers see their own,
// agents see tickets assigned to them, admins see everything.
func (s *TicketService) List(ctx context.Context, actor domain.Actor, f domain.TicketFilter) (domain.Page[domain.Ticket], error) {
	if err := validateFilter(&f); err != nil {
		return domain.Page[domain.Ticket]{}, err
	}
	applyScope(actor, &f)
	items, total, err := s.repo.ListTickets(ctx, f)
	if err != nil {
		return domain.Page[domain.Ticket]{}, err
	}
	return domain.Page[domain.Ticket]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Get returns a ticket with its linked tickets.
func (s *TicketService) Get(ctx context.Context, actor domain.Actor, idOrNumber string) (domain.TicketDetail, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	return s.detail(ctx, t)
}

// detail loads links and attachments for a ticket (no access check).
func (s *TicketService) detail(ctx context.Context, t domain.Ticket) (domain.TicketDetail, error) {
	links, err := s.repo.ListLinks(ctx, t.ID)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	attachments, err := s.repo.ListAttachments(ctx, t.ID)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	return domain.TicketDetail{Ticket: t, LinkedTickets: links, Attachments: attachments}, nil
}

// Timeline merges messages and events by time. Internal notes are hidden from customers.
func (s *TicketService) Timeline(ctx context.Context, actor domain.Actor, idOrNumber string) ([]domain.TimelineItem, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return nil, err
	}
	msgs, err := s.repo.ListMessages(ctx, t.ID, actor.Role.IsStaff())
	if err != nil {
		return nil, err
	}
	events, err := s.repo.ListEvents(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	items := make([]domain.TimelineItem, 0, len(msgs)+len(events))
	for i := range msgs {
		items = append(items, domain.TimelineItem{Kind: "message", CreatedAt: msgs[i].CreatedAt, Message: &msgs[i]})
	}
	for i := range events {
		items = append(items, domain.TimelineItem{Kind: "event", CreatedAt: events[i].CreatedAt, Event: &events[i]})
	}
	slices.SortStableFunc(items, func(a, b domain.TimelineItem) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return items, nil
}

type AddMessageInput struct {
	Body       string `json:"body"`
	IsInternal bool   `json:"isInternal"`
}

// AddMessage adds a comment. A customer reply on an on-hold ticket moves it back to in progress.
func (s *TicketService) AddMessage(ctx context.Context, actor domain.Actor, idOrNumber string, in AddMessageInput) (domain.Message, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.Message{}, err
	}
	body, err := requireLength("Message", in.Body, 1, maxMessageLen)
	if err != nil {
		return domain.Message{}, err
	}
	if in.IsInternal && !actor.Role.IsStaff() {
		return domain.Message{}, apperr.Forbidden("Only agents can add internal notes")
	}
	if t.Status == domain.StatusClosed {
		return domain.Message{}, apperr.Conflict("This ticket is closed. Please create a new ticket and link this one")
	}

	var msg domain.Message
	err = s.repo.InTx(ctx, func(tx repo.Repository) error {
		msg, err = tx.CreateMessage(ctx, repo.NewMessage{TicketID: t.ID, AuthorID: actor.ID, Body: body, IsInternal: in.IsInternal})
		if err != nil {
			return err
		}
		if actor.Role == domain.RoleCustomer && t.Status == domain.StatusOnHold {
			return s.setStatus(ctx, tx, actor, t, domain.StatusInProgress)
		}
		return tx.TouchTicket(ctx, t.ID)
	})
	if err != nil {
		return domain.Message{}, err
	}
	msg.Author.Name = actor.Name
	msg.AuthorRole = actor.Role
	return msg, nil
}

// ChangeStatus moves a ticket to a new status, enforcing the state machine.
func (s *TicketService) ChangeStatus(ctx context.Context, actor domain.Actor, idOrNumber string, status domain.Status) (domain.TicketDetail, error) {
	if !domain.IsOneOf(string(status), domain.Statuses) {
		return domain.TicketDetail{}, apperr.Validation("Invalid status")
	}
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	if err := CheckTransition(actor.Role, t.Status, status, t.SolvedAt, s.now()); err != nil {
		return domain.TicketDetail{}, err
	}
	err = s.repo.InTx(ctx, func(tx repo.Repository) error {
		return s.setStatus(ctx, tx, actor, t, status)
	})
	if err != nil {
		return domain.TicketDetail{}, err
	}
	return s.Get(ctx, actor, t.ID)
}

// Assign reassigns a ticket to another agent or admin. Admins can reassign any
// ticket; agents can hand off tickets currently assigned to them (enforced by
// loadAccessible, which hides other tickets from agents).
func (s *TicketService) Assign(ctx context.Context, actor domain.Actor, idOrNumber, assigneeID string) (domain.TicketDetail, error) {
	if !actor.Role.IsStaff() {
		return domain.TicketDetail{}, apperr.Forbidden("Only agents and admins can reassign tickets")
	}
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	if t.Status == domain.StatusClosed {
		return domain.TicketDetail{}, apperr.Conflict("Closed tickets cannot be reassigned")
	}
	assignee, err := s.repo.GetUserByID(ctx, strings.TrimSpace(assigneeID))
	if errors.Is(err, repo.ErrNotFound) || (err == nil && (!assignee.Role.IsStaff() || assignee.DeactivatedAt != nil)) {
		return domain.TicketDetail{}, apperr.Validation("Assignee must be an active agent or admin")
	}
	if err != nil {
		return domain.TicketDetail{}, err
	}
	if t.Assignee != nil && t.Assignee.ID == assignee.ID {
		return s.Get(ctx, actor, t.ID) // nothing to change
	}

	err = s.repo.InTx(ctx, func(tx repo.Repository) error {
		if err := tx.UpdateTicketAssignee(ctx, t.ID, assignee.ID); err != nil {
			return err
		}
		return tx.CreateEvent(ctx, t.ID, actor.ID, domain.EventAssigned,
			map[string]any{"assigneeId": assignee.ID, "assigneeName": assignee.FullName})
	})
	if err != nil {
		return domain.TicketDetail{}, err
	}
	// Load without the access check: an agent who handed the ticket off
	// can no longer view it, but should still get the result of the action.
	updated, err := s.repo.GetTicket(ctx, t.ID)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	return s.detail(ctx, updated)
}

// AddLink links one of the requester's previous solved/closed tickets.
func (s *TicketService) AddLink(ctx context.Context, actor domain.Actor, idOrNumber, linkedID string) (domain.TicketDetail, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.TicketDetail{}, err
	}
	if t.Requester.ID != actor.ID {
		return domain.TicketDetail{}, apperr.Forbidden("Only the ticket owner can link tickets")
	}
	err = s.repo.InTx(ctx, func(tx repo.Repository) error {
		if err := s.link(ctx, tx, actor, t.ID, t.Requester.ID, linkedID); err != nil {
			return err
		}
		return tx.TouchTicket(ctx, t.ID)
	})
	if err != nil {
		return domain.TicketDetail{}, err
	}
	return s.Get(ctx, actor, t.ID)
}

// RemoveLink deletes a link (owner only).
func (s *TicketService) RemoveLink(ctx context.Context, actor domain.Actor, idOrNumber, linkedID string) error {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return err
	}
	if t.Requester.ID != actor.ID {
		return apperr.Forbidden("Only the ticket owner can remove links")
	}
	linked, err := s.repo.GetTicket(ctx, linkedID)
	if errors.Is(err, repo.ErrNotFound) {
		return apperr.NotFound("Link not found")
	}
	if err != nil {
		return err
	}
	if err := s.repo.DeleteLink(ctx, t.ID, linked.ID); errors.Is(err, repo.ErrNotFound) {
		return apperr.NotFound("Link not found")
	} else if err != nil {
		return err
	}
	return nil
}

// ListAgents returns active staff with their open workload.
func (s *TicketService) ListAgents(ctx context.Context) ([]domain.StaffMember, error) {
	return s.repo.ListStaff(ctx)
}

// RemoveAgent deactivates an agent (admin only). Their unclosed tickets go back
// to the default admin so nothing is left unattended.
func (s *TicketService) RemoveAgent(ctx context.Context, actor domain.Actor, agentID string) (int, error) {
	if actor.Role != domain.RoleAdmin {
		return 0, apperr.Forbidden("Only admins can remove agents")
	}
	var moved int
	err := s.repo.InTx(ctx, func(tx repo.Repository) error {
		admin, err := tx.GetDefaultAssignee(ctx)
		if err != nil {
			return errors.Join(errors.New("no default assignee configured"), err)
		}
		moved, err = tx.DeactivateAgent(ctx, strings.TrimSpace(agentID), admin.ID, admin.FullName, actor.ID)
		if errors.Is(err, repo.ErrNotFound) {
			return apperr.NotFound("Agent not found")
		}
		return err
	})
	return moved, err
}

// AutoClose closes solved tickets older than ReopenWindow.
func (s *TicketService) AutoClose(ctx context.Context) (int64, error) {
	return s.repo.AutoCloseSolved(ctx, s.now().Add(-ReopenWindow))
}

// ---------- helpers ----------

// loadAccessible fetches a ticket and returns 404 if the actor may not see it,
// so ticket existence is never leaked.
func (s *TicketService) loadAccessible(ctx context.Context, actor domain.Actor, idOrNumber string) (domain.Ticket, error) {
	t, err := s.repo.GetTicket(ctx, strings.TrimSpace(idOrNumber))
	if errors.Is(err, repo.ErrNotFound) {
		return domain.Ticket{}, apperr.NotFound("Ticket not found")
	}
	if err != nil {
		return domain.Ticket{}, err
	}
	if !canView(actor, t) {
		return domain.Ticket{}, apperr.NotFound("Ticket not found")
	}
	return t, nil
}

func canView(actor domain.Actor, t domain.Ticket) bool {
	switch actor.Role {
	case domain.RoleAdmin:
		return true
	case domain.RoleAgent:
		return t.Assignee != nil && t.Assignee.ID == actor.ID
	default:
		return t.Requester.ID == actor.ID
	}
}

func applyScope(actor domain.Actor, f *domain.TicketFilter) {
	switch actor.Role {
	case domain.RoleCustomer:
		f.RequesterID = actor.ID
	case domain.RoleAgent:
		f.AssigneeID = actor.ID
	}
}

func validateFilter(f *domain.TicketFilter) error {
	for _, check := range []struct {
		name    string
		values  []string
		allowed []string
	}{{"status", f.Status, domain.Statuses}, {"type", f.Type, domain.Types}, {"severity", f.Severity, domain.Severities}} {
		for _, v := range check.values {
			if !domain.IsOneOf(v, check.allowed) {
				return apperr.Validation(fmt.Sprintf("Invalid %s filter: %q", check.name, v))
			}
		}
	}
	f.Search = strings.TrimSpace(f.Search)
	if len(f.Search) > 100 {
		return apperr.Validation("Search text is too long")
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = defaultPageSize
	}
	f.PageSize = min(f.PageSize, maxPageSize)
	if f.CreatedFrom != nil && f.CreatedTo != nil && !f.CreatedFrom.Before(*f.CreatedTo) {
		return apperr.Validation("'from' must be before 'to'")
	}
	// Role scope is applied by the service, never taken from the client.
	f.RequesterID, f.AssigneeID = "", ""
	return nil
}

// setStatus updates status + solved_at and records a status_changed event.
func (s *TicketService) setStatus(ctx context.Context, tx repo.Repository, actor domain.Actor, t domain.Ticket, to domain.Status) error {
	solvedAt := t.SolvedAt
	switch to {
	case domain.StatusSolved:
		now := s.now()
		solvedAt = &now
	case domain.StatusOpen, domain.StatusInProgress, domain.StatusOnHold:
		solvedAt = nil
	}
	if err := tx.UpdateTicketStatus(ctx, t.ID, to, solvedAt); err != nil {
		return err
	}
	return tx.CreateEvent(ctx, t.ID, actor.ID, domain.EventStatusChanged,
		map[string]any{"from": string(t.Status), "to": string(to)})
}

// link validates and creates a link from ticketID to linkedID.
// The linked ticket must belong to the same requester and be solved or closed.
func (s *TicketService) link(ctx context.Context, tx repo.Repository, actor domain.Actor, ticketID, requesterID, linkedID string) error {
	linked, err := tx.GetTicket(ctx, strings.TrimSpace(linkedID))
	if errors.Is(err, repo.ErrNotFound) || (err == nil && linked.Requester.ID != requesterID) {
		return apperr.Validation("Linked ticket not found")
	}
	if err != nil {
		return err
	}
	if linked.ID == ticketID {
		return apperr.Validation("A ticket cannot be linked to itself")
	}
	if linked.Status != domain.StatusSolved && linked.Status != domain.StatusClosed {
		return apperr.Validation("Only solved or closed tickets can be linked")
	}
	if err := tx.CreateLink(ctx, ticketID, linked.ID); errors.Is(err, repo.ErrDuplicate) {
		return apperr.Conflict("These tickets are already linked")
	} else if err != nil {
		return err
	}
	return tx.CreateEvent(ctx, ticketID, actor.ID, domain.EventLinked,
		map[string]any{"linkedTicketId": linked.ID, "linkedTicketNumber": linked.TicketNumber})
}

func zeroCounts(keys []string) map[string]int {
	m := make(map[string]int, len(keys))
	for _, k := range keys {
		m[k] = 0
	}
	return m
}
