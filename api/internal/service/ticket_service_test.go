package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/auth"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/repo/memrepo"
)

func init() { auth.BcryptCost = 4 } // keep tests fast

// fixture holds a repo with a default admin, an agent and two customers.
type fixture struct {
	repo         *memrepo.Repo
	svc          *TicketService
	admin, agent domain.Actor
	alice, bob   domain.Actor
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	r := memrepo.New()
	actor := func(u domain.User) domain.Actor { return domain.Actor{ID: u.ID, Role: u.Role, Name: u.FullName} }
	return &fixture{
		repo:  r,
		svc:   NewTicketService(r),
		admin: actor(r.AddUser(domain.User{Email: "admin@x.com", FullName: "Admin", Role: domain.RoleAdmin}, true)),
		agent: actor(r.AddUser(domain.User{Email: "agent@x.com", FullName: "Agent", Role: domain.RoleAgent}, false)),
		alice: actor(r.AddUser(domain.User{Email: "alice@x.com", FullName: "Alice", Role: domain.RoleCustomer}, false)),
		bob:   actor(r.AddUser(domain.User{Email: "bob@x.com", FullName: "Bob", Role: domain.RoleCustomer}, false)),
	}
}

func (f *fixture) create(t *testing.T, who domain.Actor, subject string) domain.TicketDetail {
	t.Helper()
	tk, err := f.svc.Create(context.Background(), who, CreateTicketInput{
		Subject: subject, Description: "details", Type: "technical", NotifyEmails: "alice@x.com",
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	return tk
}

func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected error code %s, got %v", code, err)
	}
}

func TestCreate_AutoAssignsDefaultAdminAndDefaultsSeverity(t *testing.T) {
	f := newFixture(t)
	tk := f.create(t, f.alice, "Cannot login")

	if tk.Assignee == nil || tk.Assignee.ID != f.admin.ID {
		t.Fatalf("expected assignee %s, got %+v", f.admin.ID, tk.Assignee)
	}
	if tk.Status != domain.StatusOpen || tk.Severity != "medium" {
		t.Fatalf("unexpected status/severity: %s/%s", tk.Status, tk.Severity)
	}
}

func TestCreate_Validation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.svc.Create(ctx, f.alice, CreateTicketInput{Subject: "ab", Description: "d", Type: "technical"})
	assertCode(t, err, "VALIDATION_ERROR")

	_, err = f.svc.Create(ctx, f.alice, CreateTicketInput{Subject: "Valid", Description: "d", Type: "bogus", NotifyEmails: "a@x.com"})
	assertCode(t, err, "VALIDATION_ERROR")

	_, err = f.svc.Create(ctx, f.admin, CreateTicketInput{Subject: "Valid", Description: "d", Type: "technical", NotifyEmails: "a@x.com"})
	assertCode(t, err, "FORBIDDEN")

	_, err = f.svc.Create(ctx, f.alice, CreateTicketInput{Subject: "Valid", Description: "d", Type: "technical"})
	assertCode(t, err, "VALIDATION_ERROR") // notification email is mandatory
}

func TestAccess_IsScopedByRole(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Alice's ticket")

	// Another customer gets 404, not 403, so ticket existence is not leaked.
	_, err := f.svc.Get(ctx, f.bob, tk.ID)
	assertCode(t, err, "NOT_FOUND")

	// An agent only sees tickets assigned to them.
	_, err = f.svc.Get(ctx, f.agent, tk.ID)
	assertCode(t, err, "NOT_FOUND")
	if _, err := f.svc.Assign(ctx, f.admin, tk.ID, f.agent.ID); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if _, err := f.svc.Get(ctx, f.agent, tk.TicketNumber); err != nil {
		t.Fatalf("agent should see assigned ticket: %v", err)
	}

	page, _ := f.svc.List(ctx, f.bob, domain.TicketFilter{})
	if page.Total != 0 {
		t.Fatalf("bob should see no tickets, got %d", page.Total)
	}
	page, _ = f.svc.List(ctx, f.admin, domain.TicketFilter{})
	if page.Total != 1 || page.PageSize != defaultPageSize {
		t.Fatalf("admin list: total=%d pageSize=%d", page.Total, page.PageSize)
	}
}

func TestList_RejectsInvalidFilter(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.List(context.Background(), f.admin, domain.TicketFilter{Status: "bogus"})
	assertCode(t, err, "VALIDATION_ERROR")
}

func TestAddMessage_CustomerReplyReopensPendingTicket(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Need help")

	if _, err := f.svc.ChangeStatus(ctx, f.admin, tk.ID, domain.StatusPending); err != nil {
		t.Fatalf("set pending: %v", err)
	}
	if _, err := f.svc.AddMessage(ctx, f.alice, tk.ID, AddMessageInput{Body: "More info"}); err != nil {
		t.Fatalf("reply: %v", err)
	}
	got, _ := f.svc.Get(ctx, f.alice, tk.ID)
	if got.Status != domain.StatusOpen {
		t.Fatalf("expected open after customer reply, got %s", got.Status)
	}
}

func TestInternalNotes_HiddenFromCustomers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Need help")

	_, err := f.svc.AddMessage(ctx, f.alice, tk.ID, AddMessageInput{Body: "x", IsInternal: true})
	assertCode(t, err, "FORBIDDEN")

	if _, err := f.svc.AddMessage(ctx, f.admin, tk.ID, AddMessageInput{Body: "secret", IsInternal: true}); err != nil {
		t.Fatalf("internal note: %v", err)
	}
	countMessages := func(a domain.Actor) int {
		items, err := f.svc.Timeline(ctx, a, tk.ID)
		if err != nil {
			t.Fatalf("timeline: %v", err)
		}
		n := 0
		for _, it := range items {
			if it.Kind == "message" {
				n++
			}
		}
		return n
	}
	if countMessages(f.alice) != 0 || countMessages(f.admin) != 1 {
		t.Fatal("internal note visibility is wrong")
	}
}

func TestAddMessage_ClosedTicketRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Need help")
	f.svc.ChangeStatus(ctx, f.admin, tk.ID, domain.StatusSolved)
	f.svc.ChangeStatus(ctx, f.admin, tk.ID, domain.StatusClosed)

	_, err := f.svc.AddMessage(ctx, f.alice, tk.ID, AddMessageInput{Body: "hello"})
	assertCode(t, err, "CONFLICT")
}

func TestLinks_Rules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	old := f.create(t, f.alice, "Old issue")
	bobs := f.create(t, f.bob, "Bob issue")

	// Linking requires the old ticket to be solved/closed.
	_, err := f.svc.Create(ctx, f.alice, CreateTicketInput{Subject: "New", Description: "d", Type: "technical", NotifyEmails: "alice@x.com", LinkedTicketIDs: []string{old.ID}})
	assertCode(t, err, "VALIDATION_ERROR")

	f.svc.ChangeStatus(ctx, f.admin, old.ID, domain.StatusSolved)
	newTk, err := f.svc.Create(ctx, f.alice, CreateTicketInput{Subject: "New", Description: "d", Type: "technical", NotifyEmails: "alice@x.com", LinkedTicketIDs: []string{old.ID}})
	if err != nil {
		t.Fatalf("create with link: %v", err)
	}
	if len(newTk.LinkedTickets) != 1 || newTk.LinkedTickets[0].ID != old.ID {
		t.Fatalf("expected link to old ticket, got %+v", newTk.LinkedTickets)
	}

	// Duplicate links and other customers' tickets are rejected.
	_, err = f.svc.AddLink(ctx, f.alice, newTk.ID, old.ID)
	assertCode(t, err, "CONFLICT")
	f.svc.ChangeStatus(ctx, f.admin, bobs.ID, domain.StatusSolved)
	_, err = f.svc.AddLink(ctx, f.alice, newTk.ID, bobs.ID)
	assertCode(t, err, "VALIDATION_ERROR")

	if err := f.svc.RemoveLink(ctx, f.alice, newTk.ID, old.ID); err != nil {
		t.Fatalf("remove link: %v", err)
	}
	assertCode(t, f.svc.RemoveLink(ctx, f.alice, newTk.ID, old.ID), "NOT_FOUND")
}

func TestAssign_RequiresStaffAssignee(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Need help")

	_, err := f.svc.Assign(ctx, f.admin, tk.ID, f.bob.ID)
	assertCode(t, err, "VALIDATION_ERROR")
	_, err = f.svc.Assign(ctx, f.alice, tk.ID, f.agent.ID)
	assertCode(t, err, "FORBIDDEN")
}

func TestAssign_AgentCanHandOffOnlyOwnTickets(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Need help")
	other := f.repo.AddUser(domain.User{Email: "agent2@x.com", FullName: "Agent Two", Role: domain.RoleAgent}, false)

	// Not assigned to the agent yet: hidden, so it can't be reassigned.
	_, err := f.svc.Assign(ctx, f.agent, tk.ID, other.ID)
	assertCode(t, err, "NOT_FOUND")

	f.svc.Assign(ctx, f.admin, tk.ID, f.agent.ID)
	got, err := f.svc.Assign(ctx, f.agent, tk.ID, other.ID)
	if err != nil || got.Assignee.ID != other.ID {
		t.Fatalf("agent hand-off failed: %v", err)
	}
	// After handing off, the first agent no longer sees the ticket.
	_, err = f.svc.Get(ctx, f.agent, tk.ID)
	assertCode(t, err, "NOT_FOUND")
}

func TestSummary_ScopedToCustomer(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.create(t, f.alice, "One")
	f.create(t, f.alice, "Two")
	f.create(t, f.bob, "Three")

	sum, err := f.svc.Summary(ctx, f.alice, domain.ReportRange{Location: time.UTC})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.Total != 2 || sum.ByStatus["open"] != 2 || sum.ByStatus["closed"] != 0 || len(sum.Recent) != 2 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	adminSum, _ := f.svc.Summary(ctx, f.admin, domain.ReportRange{Location: time.UTC})
	if adminSum.Total != 3 {
		t.Fatalf("admin should see 3 tickets, got %d", adminSum.Total)
	}
}

func TestAutoClose_ClosesOldSolvedTickets(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Old")
	solved := time.Now().Add(-8 * 24 * time.Hour)
	stored, _ := f.repo.GetTicket(ctx, tk.ID)
	stored.Status, stored.SolvedAt = domain.StatusSolved, &solved
	f.repo.SetTicket(stored)

	n, err := f.svc.AutoClose(ctx)
	if err != nil || n != 1 {
		t.Fatalf("expected 1 closed, got %d (%v)", n, err)
	}
}

func TestParseEmailList(t *testing.T) {
	emails, err := ParseEmailList(" A@x.com ; b@y.com;;a@x.com ")
	if err != nil || len(emails) != 2 || emails[0] != "a@x.com" || emails[1] != "b@y.com" {
		t.Fatalf("unexpected result %v (%v)", emails, err)
	}
	_, err = ParseEmailList("a@x.com; not-an-email")
	assertCode(t, err, "VALIDATION_ERROR")
	_, err = ParseEmailList(" ; ")
	assertCode(t, err, "VALIDATION_ERROR")
}

func TestCreate_StoresNotifyEmails(t *testing.T) {
	f := newFixture(t)
	tk, err := f.svc.Create(context.Background(), f.alice, CreateTicketInput{
		Subject: "Emails", Description: "d", Type: "billing", NotifyEmails: "alice@x.com; boss@x.com",
	})
	if err != nil || len(tk.NotifyEmails) != 2 {
		t.Fatalf("expected 2 notify emails, got %v (%v)", tk.NotifyEmails, err)
	}
}

func TestAttachments(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "With file")

	att, err := f.svc.AddAttachment(ctx, f.alice, tk.ID, "../../etc/report.pdf", []byte("%PDF-1.4 hello"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if att.FileName != "report.pdf" || att.ContentType != "application/pdf" {
		t.Fatalf("unexpected attachment %+v", att)
	}

	got, err := f.svc.GetAttachment(ctx, f.admin, tk.ID, att.ID)
	if err != nil || string(got.Data) != "%PDF-1.4 hello" {
		t.Fatalf("download: %v", err)
	}
	_, err = f.svc.GetAttachment(ctx, f.bob, tk.ID, att.ID)
	assertCode(t, err, "NOT_FOUND") // other customers can't download

	_, err = f.svc.AddAttachment(ctx, f.alice, tk.ID, "big.bin", make([]byte, MaxAttachmentBytes+1))
	assertCode(t, err, "VALIDATION_ERROR")

	detail, _ := f.svc.Get(ctx, f.alice, tk.ID)
	if len(detail.Attachments) != 1 || detail.Attachments[0].Data != nil {
		t.Fatalf("detail should list metadata only: %+v", detail.Attachments)
	}
}

func TestNotifications_ScopeUnreadAndMarkRead(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	base := time.Now()
	tick := 0
	f.repo.Now = func() time.Time { tick++; return base.Add(time.Duration(tick) * time.Second) }

	tk := f.create(t, f.alice, "Need help")
	f.create(t, f.bob, "Bob's ticket")

	// Admin (default assignee) is notified about both new tickets, without the auto-assign duplicate.
	adminFeed, _ := f.svc.Notifications(ctx, f.admin)
	if adminFeed.UnreadCount != 2 || len(adminFeed.UnreadTicketIDs) != 2 || adminFeed.Items[0].Type != "created" {
		t.Fatalf("admin feed: %+v", adminFeed)
	}

	// Alice sees the admin's reply but not internal notes or her own actions.
	f.svc.AddMessage(ctx, f.admin, tk.ID, AddMessageInput{Body: "Looking into it"})
	f.svc.AddMessage(ctx, f.admin, tk.ID, AddMessageInput{Body: "secret", IsInternal: true})
	aliceFeed, _ := f.svc.Notifications(ctx, f.alice)
	if aliceFeed.UnreadCount != 1 || aliceFeed.Items[0].Type != "comment" || aliceFeed.Items[0].Preview != "Looking into it" {
		t.Fatalf("alice feed: %+v", aliceFeed)
	}

	// Opening the ticket clears it; Bob can't mark Alice's ticket.
	if err := f.svc.MarkNotificationsRead(ctx, f.alice, tk.TicketNumber); err != nil {
		t.Fatal(err)
	}
	aliceFeed, _ = f.svc.Notifications(ctx, f.alice)
	if aliceFeed.UnreadCount != 0 || len(aliceFeed.Items) != 1 || aliceFeed.Items[0].Unread {
		t.Fatalf("expected all read: %+v", aliceFeed)
	}
	assertCode(t, f.svc.MarkNotificationsRead(ctx, f.bob, tk.ID), "NOT_FOUND")

	// Mark all as read.
	f.svc.MarkNotificationsRead(ctx, f.admin, "")
	adminFeed, _ = f.svc.Notifications(ctx, f.admin)
	if adminFeed.UnreadCount != 0 {
		t.Fatalf("admin should have no unread: %+v", adminFeed)
	}
}
