// Package memrepo is an in-memory repo.Repository used by unit tests.
package memrepo

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"ticketing/api/internal/domain"
	"ticketing/api/internal/repo"
)

var _ repo.Repository = (*Repo)(nil)

type link struct{ from, to string }

// Repo is a simple, mutex-protected in-memory store. InTx does not roll back.
type Repo struct {
	mu          sync.Mutex
	seq         int
	users       map[string]domain.User
	defaultID   string
	tokens      map[string]repo.RefreshToken // keyed by hash
	tickets     map[string]domain.Ticket
	messages    []domain.Message
	events      []domain.Event
	links       []link
	attachments []domain.Attachment
	Now         func() time.Time
}

func New() *Repo {
	return &Repo{
		users:   map[string]domain.User{},
		tokens:  map[string]repo.RefreshToken{},
		tickets: map[string]domain.Ticket{},
		Now:     time.Now,
	}
}

func (r *Repo) nextID() string {
	r.seq++
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", r.seq)
}

func (r *Repo) InTx(_ context.Context, fn func(repo.Repository) error) error { return fn(r) }

// AddUser inserts a user directly (test helper).
func (r *Repo) AddUser(u domain.User, isDefault bool) domain.User {
	r.mu.Lock()
	defer r.mu.Unlock()
	u.ID = r.nextID()
	r.users[u.ID] = u
	if isDefault {
		r.defaultID = u.ID
	}
	return u
}

// SetTicket overwrites a stored ticket (test helper).
func (r *Repo) SetTicket(t domain.Ticket) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tickets[t.ID] = t
}

func (r *Repo) CreateUser(_ context.Context, u domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if strings.EqualFold(existing.Email, u.Email) {
			return domain.User{}, repo.ErrDuplicate
		}
	}
	u.ID, u.CreatedAt = r.nextID(), r.Now()
	r.users[u.ID] = u
	return u, nil
}

func (r *Repo) GetUserByEmail(_ context.Context, email string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return domain.User{}, repo.ErrNotFound
}

func (r *Repo) GetUserByID(_ context.Context, id string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return domain.User{}, repo.ErrNotFound
}

func (r *Repo) ListStaff(_ context.Context) ([]domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.User
	for _, u := range r.users {
		if u.Role.IsStaff() {
			out = append(out, u)
		}
	}
	return out, nil
}

func (r *Repo) GetDefaultAssignee(ctx context.Context) (domain.User, error) {
	if r.defaultID == "" {
		return domain.User{}, repo.ErrNotFound
	}
	return r.GetUserByID(ctx, r.defaultID)
}

func (r *Repo) EnsureDefaultAdmin(ctx context.Context, email, hash, name string) error {
	if _, err := r.GetUserByEmail(ctx, email); err == nil {
		return nil
	}
	u := r.AddUser(domain.User{Email: email, PasswordHash: hash, FullName: name, Role: domain.RoleAdmin}, false)
	if r.defaultID == "" {
		r.defaultID = u.ID
	}
	return nil
}

func (r *Repo) CreateRefreshToken(_ context.Context, userID, hash string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[hash] = repo.RefreshToken{ID: r.nextID(), UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (r *Repo) GetRefreshToken(_ context.Context, hash string) (repo.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[hash]; ok {
		return t, nil
	}
	return repo.RefreshToken{}, repo.ErrNotFound
}

func (r *Repo) RevokeRefreshToken(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for hash, t := range r.tokens {
		if t.ID == id {
			t.Revoked = true
			r.tokens[hash] = t
		}
	}
	return nil
}

func (r *Repo) CreateTicket(_ context.Context, nt repo.NewTicket) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, now := r.nextID(), r.Now()
	t := domain.Ticket{
		ID: id, TicketNumber: fmt.Sprintf("TKT-%06d", len(r.tickets)+1),
		Subject: nt.Subject, Description: nt.Description, Type: nt.Type, Severity: nt.Severity,
		Status:       domain.StatusOpen,
		Requester:    domain.UserRef{ID: nt.RequesterID, Name: r.users[nt.RequesterID].FullName},
		Assignee:     &domain.UserRef{ID: nt.AssigneeID, Name: r.users[nt.AssigneeID].FullName},
		NotifyEmails: nt.NotifyEmails,
		CreatedAt:    now, UpdatedAt: now,
	}
	r.tickets[id] = t
	return id, nil
}

func (r *Repo) GetTicket(_ context.Context, idOrNumber string) (domain.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tickets {
		if t.ID == idOrNumber || t.TicketNumber == idOrNumber {
			return t, nil
		}
	}
	return domain.Ticket{}, repo.ErrNotFound
}

func (r *Repo) matches(t domain.Ticket, f domain.TicketFilter) bool {
	return (f.RequesterID == "" || t.Requester.ID == f.RequesterID) &&
		(f.AssigneeID == "" || (t.Assignee != nil && t.Assignee.ID == f.AssigneeID)) &&
		(f.Status == "" || string(t.Status) == f.Status) &&
		(f.Type == "" || t.Type == f.Type) &&
		(f.Severity == "" || t.Severity == f.Severity) &&
		(f.Search == "" || strings.Contains(strings.ToLower(t.Subject), strings.ToLower(f.Search)))
}

func (r *Repo) ListTickets(_ context.Context, f domain.TicketFilter) ([]domain.Ticket, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []domain.Ticket
	for _, t := range r.tickets {
		if r.matches(t, f) {
			all = append(all, t)
		}
	}
	slices.SortFunc(all, func(a, b domain.Ticket) int { return b.CreatedAt.Compare(a.CreatedAt) })
	start := min((f.Page-1)*f.PageSize, len(all))
	end := min(start+f.PageSize, len(all))
	return all[start:end], len(all), nil
}

func (r *Repo) update(id string, fn func(*domain.Ticket)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tickets[id]
	if !ok {
		return repo.ErrNotFound
	}
	fn(&t)
	t.UpdatedAt = r.Now()
	r.tickets[id] = t
	return nil
}

func (r *Repo) UpdateTicketStatus(_ context.Context, id string, status domain.Status, solvedAt *time.Time) error {
	return r.update(id, func(t *domain.Ticket) { t.Status, t.SolvedAt = status, solvedAt })
}

func (r *Repo) UpdateTicketAssignee(_ context.Context, id, assigneeID string) error {
	return r.update(id, func(t *domain.Ticket) {
		t.Assignee = &domain.UserRef{ID: assigneeID, Name: r.users[assigneeID].FullName}
	})
}

func (r *Repo) TouchTicket(_ context.Context, id string) error {
	return r.update(id, func(*domain.Ticket) {})
}

func (r *Repo) CountTickets(_ context.Context, f domain.TicketFilter) ([]domain.TicketCount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.TicketCount
	for _, t := range r.tickets {
		if r.matches(t, domain.TicketFilter{RequesterID: f.RequesterID, AssigneeID: f.AssigneeID}) {
			out = append(out, domain.TicketCount{Status: string(t.Status), Severity: t.Severity, Type: t.Type, Count: 1})
		}
	}
	return out, nil
}

func (r *Repo) AutoCloseSolved(_ context.Context, cutoff time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for id, t := range r.tickets {
		if t.Status == domain.StatusSolved && t.SolvedAt != nil && t.SolvedAt.Before(cutoff) {
			t.Status = domain.StatusClosed
			r.tickets[id] = t
			n++
		}
	}
	return n, nil
}

func (r *Repo) CreateMessage(_ context.Context, m repo.NewMessage) (domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u := r.users[m.AuthorID]
	msg := domain.Message{
		ID: r.nextID(), TicketID: m.TicketID, Author: domain.UserRef{ID: u.ID, Name: u.FullName},
		AuthorRole: u.Role, Body: m.Body, IsInternal: m.IsInternal, CreatedAt: r.Now(),
	}
	r.messages = append(r.messages, msg)
	return msg, nil
}

func (r *Repo) ListMessages(_ context.Context, ticketID string, includeInternal bool) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Message
	for _, m := range r.messages {
		if m.TicketID == ticketID && (includeInternal || !m.IsInternal) {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *Repo) CreateEvent(_ context.Context, ticketID, actorID, eventType string, data map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := domain.Event{ID: int64(len(r.events) + 1), TicketID: ticketID, Type: eventType, Data: data, CreatedAt: r.Now()}
	if actorID != "" {
		e.Actor = &domain.UserRef{ID: actorID, Name: r.users[actorID].FullName}
	}
	r.events = append(r.events, e)
	return nil
}

func (r *Repo) ListEvents(_ context.Context, ticketID string) ([]domain.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Event
	for _, e := range r.events {
		if e.TicketID == ticketID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *Repo) CreateLink(_ context.Context, ticketID, linkedID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.links {
		if (l.from == ticketID && l.to == linkedID) || (l.from == linkedID && l.to == ticketID) {
			return repo.ErrDuplicate
		}
	}
	r.links = append(r.links, link{from: ticketID, to: linkedID})
	return nil
}

func (r *Repo) DeleteLink(_ context.Context, ticketID, linkedID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, l := range r.links {
		if (l.from == ticketID && l.to == linkedID) || (l.from == linkedID && l.to == ticketID) {
			r.links = slices.Delete(r.links, i, i+1)
			return nil
		}
	}
	return repo.ErrNotFound
}

func (r *Repo) ListLinks(_ context.Context, ticketID string) ([]domain.LinkedTicket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.LinkedTicket{}
	for _, l := range r.links {
		other := ""
		switch ticketID {
		case l.from:
			other = l.to
		case l.to:
			other = l.from
		default:
			continue
		}
		t := r.tickets[other]
		out = append(out, domain.LinkedTicket{ID: t.ID, TicketNumber: t.TicketNumber, Subject: t.Subject, Status: t.Status, Relation: "related"})
	}
	return out, nil
}

func (r *Repo) CreateAttachment(_ context.Context, a repo.NewAttachment) (domain.Attachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	att := domain.Attachment{
		ID: r.nextID(), TicketID: a.TicketID, Uploader: domain.UserRef{ID: a.UploaderID, Name: r.users[a.UploaderID].FullName},
		FileName: a.FileName, ContentType: a.ContentType, SizeBytes: len(a.Data), CreatedAt: r.Now(), Data: a.Data,
	}
	r.attachments = append(r.attachments, att)
	return att, nil
}

func (r *Repo) ListAttachments(_ context.Context, ticketID string) ([]domain.Attachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Attachment{}
	for _, a := range r.attachments {
		if a.TicketID == ticketID {
			a.Data = nil
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *Repo) CountAttachments(ctx context.Context, ticketID string) (int, error) {
	list, err := r.ListAttachments(ctx, ticketID)
	return len(list), err
}

func (r *Repo) GetAttachment(_ context.Context, ticketID, attachmentID string) (domain.Attachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.attachments {
		if a.TicketID == ticketID && a.ID == attachmentID {
			return a, nil
		}
	}
	return domain.Attachment{}, repo.ErrNotFound
}
