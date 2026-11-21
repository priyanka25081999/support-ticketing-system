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
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusOnHold     Status = "on_hold" // waiting on the customer
	StatusSolved     Status = "solved"
	StatusClosed     Status = "closed"
)

// Allowed enum values; they mirror the Postgres enums in db/migrations.
var (
	Statuses   = []string{"open", "in_progress", "on_hold", "solved", "closed"}
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
	// DeactivatedAt is set when an admin removes the user (soft delete).
	DeactivatedAt *time.Time `json:"-"`
}

// StaffMember is an agent/admin with their current workload.
type StaffMember struct {
	User
	OpenTickets int `json:"openTickets"` // open + in progress + on hold tickets assigned
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
	// Multi-select filters: a ticket matches if its value is any of the listed ones.
	Status   []string
	Type     []string
	Severity []string
	// AssigneeIDs filters by assignee (admin only; ignored for other roles).
	AssigneeIDs []string
	Search      string
	Sort        string // newest | oldest | updated | severity
	Page        int
	PageSize    int
	RequesterID string
	AssigneeID  string
	// Optional created_at range: CreatedFrom <= created_at < CreatedTo.
	CreatedFrom *time.Time
	CreatedTo   *time.Time
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

// Notification is one activity item (by someone else) on a ticket the user
// requested or is assigned to.
type Notification struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"` // event | message | attachment
	Type         string         `json:"type"` // created, assigned, status_changed, linked, comment, internal_note, attachment
	TicketID     string         `json:"ticketId"`
	TicketNumber string         `json:"ticketNumber"`
	Subject      string         `json:"subject"`
	ActorName    string         `json:"actorName"`
	Data         map[string]any `json:"data"`
	Preview      string         `json:"preview"`
	CreatedAt    time.Time      `json:"createdAt"`
	Unread       bool           `json:"unread"`
}

type NotificationFeed struct {
	UnreadCount     int            `json:"unreadCount"`
	UnreadTicketIDs []string       `json:"unreadTicketIds"`
	Items           []Notification `json:"items"`
}

// ReportRange is an optional time window plus the timezone used for trend buckets.
type ReportRange struct {
	From     *time.Time
	To       *time.Time
	Location *time.Location
}

// ResolutionStats are counts of status changes that happened inside a range.
type ResolutionStats struct {
	Resolved           int      `json:"resolved"`
	Closed             int      `json:"closed"`
	Reopened           int      `json:"reopened"`
	AvgResolutionHours *float64 `json:"avgResolutionHours"`
}

// TrendPoint is one bucket (hour/day/month) of created vs resolved tickets.
type TrendPoint struct {
	Bucket   time.Time `json:"bucket"`
	Created  int       `json:"created"`
	Resolved int       `json:"resolved"`
}

// Summary is the dashboard: tickets created in the range broken down by
// status/severity/type, resolution stats, a trend and recent tickets.
type Summary struct {
	From       *time.Time     `json:"from"`
	To         *time.Time     `json:"to"`
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"byStatus"`
	BySeverity map[string]int `json:"bySeverity"`
	ByType     map[string]int `json:"byType"`
	ResolutionStats
	Bucket string       `json:"bucket"`
	Trend  []TrendPoint `json:"trend"`
	Recent []Ticket     `json:"recent"`
}

// ChatMessage is a private 1:1 message between reporter and assignee.
type ChatMessage struct {
	ID         string    `json:"id"`
	TicketID   string    `json:"ticketId"`
	Sender     UserRef   `json:"sender"`
	SenderRole Role      `json:"senderRole"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ChatThread is the chat history plus what the viewer is allowed to do.
type ChatThread struct {
	Messages  []ChatMessage `json:"messages"`
	CanSend   bool          `json:"canSend"`
	Reason    string        `json:"reason,omitempty"`
	Requester UserRef       `json:"requester"`
	Assignee  *UserRef      `json:"assignee"`
}
