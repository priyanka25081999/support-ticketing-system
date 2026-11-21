// Package repo is the data access layer. Services depend on the Repository
// interface so they can be unit-tested with an in-memory fake (see memrepo).
package repo

import (
	"context"
	"errors"
	"time"

	"ticketing/api/internal/domain"
)

var (
	ErrNotFound  = errors.New("record not found")
	ErrDuplicate = errors.New("duplicate record")
)

type NewTicket struct {
	Subject      string
	Description  string
	Type         string
	Severity     string
	RequesterID  string
	AssigneeID   string
	NotifyEmails []string
}

type NewAttachment struct {
	TicketID    string
	UploaderID  string
	FileName    string
	ContentType string
	Data        []byte
}

type NewMessage struct {
	TicketID   string
	AuthorID   string
	Body       string
	IsInternal bool
}

type RefreshToken struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	Revoked   bool
}

type Repository interface {
	// InTx runs fn inside a single database transaction.
	InTx(ctx context.Context, fn func(Repository) error) error

	// Users
	CreateUser(ctx context.Context, u domain.User) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByID(ctx context.Context, id string) (domain.User, error)
	ListStaff(ctx context.Context) ([]domain.User, error)
	GetDefaultAssignee(ctx context.Context) (domain.User, error)
	// EnsureDefaultAdmin creates the default admin if missing and marks it as default assignee.
	EnsureDefaultAdmin(ctx context.Context, email, passwordHash, name string) error

	// Refresh tokens
	CreateRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id string) error

	// Tickets (idOrNumber accepts a UUID or a ticket number such as TKT-000123)
	CreateTicket(ctx context.Context, t NewTicket) (string, error)
	GetTicket(ctx context.Context, idOrNumber string) (domain.Ticket, error)
	ListTickets(ctx context.Context, f domain.TicketFilter) ([]domain.Ticket, int, error)
	UpdateTicketStatus(ctx context.Context, id string, status domain.Status, solvedAt *time.Time) error
	UpdateTicketAssignee(ctx context.Context, id, assigneeID string) error
	TouchTicket(ctx context.Context, id string) error
	CountTickets(ctx context.Context, f domain.TicketFilter) ([]domain.TicketCount, error)
	// AutoCloseSolved closes tickets solved before the cutoff and returns how many were closed.
	AutoCloseSolved(ctx context.Context, cutoff time.Time) (int64, error)

	// Messages & events
	CreateMessage(ctx context.Context, m NewMessage) (domain.Message, error)
	ListMessages(ctx context.Context, ticketID string, includeInternal bool) ([]domain.Message, error)
	CreateEvent(ctx context.Context, ticketID, actorID, eventType string, data map[string]any) error
	ListEvents(ctx context.Context, ticketID string) ([]domain.Event, error)

	// Attachments
	CreateAttachment(ctx context.Context, a NewAttachment) (domain.Attachment, error)
	ListAttachments(ctx context.Context, ticketID string) ([]domain.Attachment, error)
	CountAttachments(ctx context.Context, ticketID string) (int, error)
	// GetAttachment returns metadata and file data.
	GetAttachment(ctx context.Context, ticketID, attachmentID string) (domain.Attachment, error)

	// Reports (scope + created range come from the filter; events use the range on their own time)
	ResolutionStats(ctx context.Context, f domain.TicketFilter) (domain.ResolutionStats, error)
	TicketTrend(ctx context.Context, f domain.TicketFilter, bucket string, loc *time.Location) ([]domain.TrendPoint, error)

	// Chat
	CreateChatMessage(ctx context.Context, ticketID, senderID, body string) (domain.ChatMessage, error)
	ListChatMessages(ctx context.Context, ticketID string, limit int) ([]domain.ChatMessage, error)

	// Notifications
	ListNotifications(ctx context.Context, userID string, includeInternal bool, limit int) (domain.NotificationFeed, error)
	MarkAllNotificationsRead(ctx context.Context, userID string) error
	MarkTicketRead(ctx context.Context, userID, ticketID string) error

	// Links
	CreateLink(ctx context.Context, ticketID, linkedID string) error
	DeleteLink(ctx context.Context, ticketID, linkedID string) error
	ListLinks(ctx context.Context, ticketID string) ([]domain.LinkedTicket, error)
}
