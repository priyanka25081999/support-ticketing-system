package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/repo"
)

const (
	MaxAttachmentBytes      = 5 << 20 // 5 MB per file
	maxAttachmentsPerTicket = 10
)

// AddAttachment stores a file on a ticket the actor can access (not on closed tickets).
func (s *TicketService) AddAttachment(ctx context.Context, actor domain.Actor, idOrNumber, fileName string, data []byte) (domain.Attachment, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.Attachment{}, err
	}
	if t.Status == domain.StatusClosed {
		return domain.Attachment{}, apperr.Conflict("Cannot add attachments to a closed ticket")
	}
	if len(data) == 0 {
		return domain.Attachment{}, apperr.Validation("File is empty")
	}
	if len(data) > MaxAttachmentBytes {
		return domain.Attachment{}, apperr.Validation(fmt.Sprintf("File is larger than %d MB", MaxAttachmentBytes>>20))
	}
	count, err := s.repo.CountAttachments(ctx, t.ID)
	if err != nil {
		return domain.Attachment{}, err
	}
	if count >= maxAttachmentsPerTicket {
		return domain.Attachment{}, apperr.Validation(fmt.Sprintf("A ticket can have at most %d attachments", maxAttachmentsPerTicket))
	}

	att, err := s.repo.CreateAttachment(ctx, repo.NewAttachment{
		TicketID:    t.ID,
		UploaderID:  actor.ID,
		FileName:    SanitizeFileName(fileName),
		ContentType: http.DetectContentType(data), // never trust the client-sent type
		Data:        data,
	})
	if err != nil {
		return domain.Attachment{}, err
	}
	att.Uploader.Name = actor.Name
	return att, s.repo.TouchTicket(ctx, t.ID)
}

// GetAttachment returns a file (with data) if the actor can access the ticket.
func (s *TicketService) GetAttachment(ctx context.Context, actor domain.Actor, idOrNumber, attachmentID string) (domain.Attachment, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.Attachment{}, err
	}
	att, err := s.repo.GetAttachment(ctx, t.ID, attachmentID)
	if errors.Is(err, repo.ErrNotFound) {
		return domain.Attachment{}, apperr.NotFound("Attachment not found")
	}
	return att, err
}

// SanitizeFileName strips paths and control/quote characters so the name is safe in headers.
func SanitizeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "attachment"
	}
	if runes := []rune(name); len(runes) > 200 {
		name = string(runes[len(runes)-200:])
	}
	return name
}
