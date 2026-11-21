package service

import (
	"context"

	"ticketing/api/internal/domain"
)

const notificationLimit = 30

// Notifications returns recent activity by others on tickets the actor
// requested or is assigned to. Internal notes are only shown to staff.
func (s *TicketService) Notifications(ctx context.Context, actor domain.Actor) (domain.NotificationFeed, error) {
	return s.repo.ListNotifications(ctx, actor.ID, actor.Role.IsStaff(), notificationLimit)
}

// MarkNotificationsRead marks one ticket's activity as read, or everything when ticketID is empty.
func (s *TicketService) MarkNotificationsRead(ctx context.Context, actor domain.Actor, ticketID string) error {
	if ticketID == "" {
		return s.repo.MarkAllNotificationsRead(ctx, actor.ID)
	}
	t, err := s.loadAccessible(ctx, actor, ticketID)
	if err != nil {
		return err
	}
	return s.repo.MarkTicketRead(ctx, actor.ID, t.ID)
}
