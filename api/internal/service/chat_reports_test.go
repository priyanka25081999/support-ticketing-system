package service

import (
	"context"
	"testing"
	"time"

	"ticketing/api/internal/domain"
)

func TestChat_OneToOnePermissions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Chat please")
	f.svc.Assign(ctx, f.admin, tk.ID, f.agent.ID)

	// Reporter and assignee can send.
	if _, err := f.svc.SendChat(ctx, f.alice, tk.ID, "Hi, any update?"); err != nil {
		t.Fatalf("reporter send: %v", err)
	}
	if _, err := f.svc.SendChat(ctx, f.agent, tk.ID, "Looking now"); err != nil {
		t.Fatalf("assignee send: %v", err)
	}

	// Admin (not assignee) can read but not send; other customers can't see it.
	thread, err := f.svc.ChatThread(ctx, f.admin, tk.ID)
	if err != nil || thread.CanSend || len(thread.Messages) != 2 {
		t.Fatalf("admin thread: %+v (%v)", thread, err)
	}
	_, err = f.svc.SendChat(ctx, f.admin, tk.ID, "hello")
	assertCode(t, err, "FORBIDDEN")
	_, err = f.svc.ChatThread(ctx, f.bob, tk.ID)
	assertCode(t, err, "NOT_FOUND")

	// Empty messages are rejected.
	_, err = f.svc.SendChat(ctx, f.alice, tk.ID, "   ")
	assertCode(t, err, "VALIDATION_ERROR")
}

func TestChat_LiveDeliveryRespectsAccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	tk := f.create(t, f.alice, "Live chat")
	f.svc.Assign(ctx, f.admin, tk.ID, f.agent.ID)

	agentStream, stopAgent, err := f.svc.SubscribeChat(ctx, f.agent, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer stopAgent()

	f.svc.SendChat(ctx, f.alice, tk.ID, "ping")
	select {
	case msg := <-agentStream:
		if msg.Body != "ping" || msg.Sender.Name != "Alice" {
			t.Fatalf("unexpected message %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("assignee did not receive the message")
	}

	// After hand-off the old assignee's open stream stops receiving messages.
	other := f.repo.AddUser(domain.User{Email: "a2@x.com", FullName: "Agent Two", Role: domain.RoleAgent}, false)
	f.svc.Assign(ctx, f.agent, tk.ID, other.ID)
	f.svc.SendChat(ctx, f.alice, tk.ID, "secret")
	select {
	case msg := <-agentStream:
		t.Fatalf("old assignee should not receive %+v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSummary_RangeStatsAndTrend(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	clock := start
	f.repo.Now = func() time.Time { return clock }
	f.svc.now = func() time.Time { return clock }

	t1 := f.create(t, f.alice, "Day one")
	clock = start.Add(24 * time.Hour)
	f.create(t, f.alice, "Day two")
	clock = clock.Add(2 * time.Hour)
	f.svc.ChangeStatus(ctx, f.admin, t1.ID, domain.StatusSolved)

	from := start.Add(12 * time.Hour) // excludes the first ticket's creation
	rng := domain.ReportRange{From: &from, Location: time.UTC}
	sum, err := f.svc.Summary(ctx, f.admin, rng)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total != 1 || sum.Resolved != 1 || sum.AvgResolutionHours == nil || *sum.AvgResolutionHours != 26 {
		t.Fatalf("unexpected stats: total=%d resolved=%d avg=%v", sum.Total, sum.Resolved, sum.AvgResolutionHours)
	}
	if sum.Bucket != "hour" || len(sum.Trend) == 0 {
		t.Fatalf("expected hourly trend, got %s with %d points", sum.Bucket, len(sum.Trend))
	}
}

func TestParseReportRange(t *testing.T) {
	rng, err := ParseReportRange("2026-11-01T00:00:00Z", "2026-11-02T00:00:00Z", "Asia/Kolkata")
	if err != nil || rng.Location.String() != "Asia/Kolkata" || rng.From == nil || rng.To == nil {
		t.Fatalf("unexpected: %+v %v", rng, err)
	}
	_, err = ParseReportRange("2026-11-02T00:00:00Z", "2026-11-01T00:00:00Z", "")
	assertCode(t, err, "VALIDATION_ERROR")
	_, err = ParseReportRange("yesterday", "", "")
	assertCode(t, err, "VALIDATION_ERROR")
	_, err = ParseReportRange("", "", "Mars/Base")
	assertCode(t, err, "VALIDATION_ERROR")
}

func TestExport_StaffOnly(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.alice, "Exported")
	_, err := f.svc.ExportTickets(context.Background(), f.alice, domain.ReportRange{Location: time.UTC})
	assertCode(t, err, "FORBIDDEN")
	rows, err := f.svc.ExportTickets(context.Background(), f.admin, domain.ReportRange{Location: time.UTC})
	if err != nil || len(rows) != 1 {
		t.Fatalf("admin export: %d rows (%v)", len(rows), err)
	}
}
