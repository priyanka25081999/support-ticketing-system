// Package domain contains the core types shared by all backend layers.
package domain

import (
	"slices"
	"time"
)

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAgent    Role = "agent"
	RoleAdmin    Role = "admin"
)

// IsStaff reports whether the role works tickets (agent or admin).
func (r Role) IsStaff() bool { return r == RoleAgent || r == RoleAdmin }

type Status string

const (
	StatusOpen    Status = "open"
	StatusPending Status = "pending"
	StatusSolved  Status = "solved"
	StatusClosed  Status = "closed"
)

// Allowed enum values; they mirror the Postgres enums in db/migrations.
var (
	Statuses   = []string{"open", "pending", "solved", "closed"}
	Types      = []string{"technical", "billing", "account", "feature_request", "other"}
	Severities = []string{"low", "medium", "high", "critical"}
)

// IsOneOf reports whether value is in allowed.
func IsOneOf(value string, allowed []string) bool { return slices.Contains(allowed, value) }

// Event types stored in ticket_events.
const (
	EventCreated       = "created"
	EventAssigned      = "assigned"
	EventStatusChanged = "status_changed"
	EventLinked        = "linked"
)

// Actor is the authenticated user making a request.
type Actor struct {
	ID   string
	Role Role
	Name string
}

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	FullName     string    `json:"fullName"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

// UserRef is a lightweight reference to a user shown in API responses.
type UserRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Ticket struct {
	ID           string     `json:"id"`
	TicketNumber string     `json:"ticketNumber"`
	Subject      string     `json:"subject"`
	Description  string     `json:"description"`
	Type         string     `json:"type"`
	Severity     string     `json:"severity"`
	Status       Status     `json:"status"`
	Requester    UserRef    `json:"requester"`
	Assignee     *UserRef   `json:"assignee"`
	NotifyEmails []string   `json:"notifyEmails"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	SolvedAt     *time.Time `json:"solvedAt"`
}

type LinkedTicket struct {
	ID           string    `json:"id"`
	TicketNumber string    `json:"ticketNumber"`
	Subject      string    `json:"subject"`
	Status       Status    `json:"status"`
	Relation     string    `json:"relation"`
	CreatedAt    time.Time `json:"createdAt"`
}

// TicketDetail is a ticket plus its linked tickets and attachments.
type TicketDetail struct {
	Ticket
	LinkedTickets []LinkedTicket `json:"linkedTickets"`
	Attachments   []Attachment   `json:"attachments"`
}

// Attachment is file metadata; Data is only loaded for downloads.
type Attachment struct {
	ID          string    `json:"id"`
	TicketID    string    `json:"ticketId"`
	Uploader    UserRef   `json:"uploader"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	SizeBytes   int       `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	Data        []byte    `json:"-"`
}

type Message struct {
	ID         string    `json:"id"`
	TicketID   string    `json:"ticketId"`
	Author     UserRef   `json:"author"`
	AuthorRole Role      `json:"authorRole"`
	Body       string    `json:"body"`
	IsInternal bool      `json:"isInternal"`
	CreatedAt  time.Time `json:"createdAt"`
}

type Event struct {
	ID        int64          `json:"id"`
	TicketID  string         `json:"ticketId"`
	Actor     *UserRef       `json:"actor"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"createdAt"`
}

// TimelineItem is either a message or an event, ordered by CreatedAt.
type TimelineItem struct {
	Kind      string    `json:"kind"` // "message" or "event"
	CreatedAt time.Time `json:"createdAt"`
	Message   *Message  `json:"message,omitempty"`
	Event     *Event    `json:"event,omitempty"`
}

// TicketFilter holds list filters. RequesterID/AssigneeID scope results by role.
type TicketFilter struct {
	Status      string
	Type        string
	Severity    string
	Search      string
	Sort        string // newest | oldest | updated | severity
	Page        int
	PageSize    int
	RequesterID string
	AssigneeID  string
}

type Page[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// TicketCount is one grouped row used to build the dashboard summary.
type TicketCount struct {
	Status   string
	Severity string
	Type     string
	Count    int
}

type Summary struct {
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"byStatus"`
	BySeverity map[string]int `json:"bySeverity"`
	ByType     map[string]int `json:"byType"`
	Recent     []Ticket       `json:"recent"`
}
