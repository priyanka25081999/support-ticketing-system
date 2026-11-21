package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
	"ticketing/api/internal/httpx"
	"ticketing/api/internal/service"
)

// TicketHandler serves ticket, meta, agent-list and report endpoints.
type TicketHandler struct {
	svc *service.TicketService
}

func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.CreateTicketInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.svc.Create(r.Context(), mustActor(r), in)
	respond(w, http.StatusCreated, ticket, err)
}

func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	filter := domain.TicketFilter{
		Status:   q.Get("status"),
		Type:     q.Get("type"),
		Severity: q.Get("severity"),
		Search:   q.Get("search"),
		Sort:     q.Get("sort"),
		Page:     page,
		PageSize: pageSize,
	}
	result, err := h.svc.List(r.Context(), mustActor(r), filter)
	respond(w, http.StatusOK, result, err)
}

func (h *TicketHandler) Get(w http.ResponseWriter, r *http.Request) {
	ticket, err := h.svc.Get(r.Context(), mustActor(r), chi.URLParam(r, "id"))
	respond(w, http.StatusOK, ticket, err)
}

func (h *TicketHandler) Timeline(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Timeline(r.Context(), mustActor(r), chi.URLParam(r, "id"))
	respond(w, http.StatusOK, items, err)
}

func (h *TicketHandler) AddMessage(w http.ResponseWriter, r *http.Request) {
	var in service.AddMessageInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	msg, err := h.svc.AddMessage(r.Context(), mustActor(r), chi.URLParam(r, "id"), in)
	respond(w, http.StatusCreated, msg, err)
}

func (h *TicketHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status domain.Status `json:"status"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.svc.ChangeStatus(r.Context(), mustActor(r), chi.URLParam(r, "id"), in.Status)
	respond(w, http.StatusOK, ticket, err)
}

func (h *TicketHandler) Assign(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AssigneeID string `json:"assigneeId"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.svc.Assign(r.Context(), mustActor(r), chi.URLParam(r, "id"), in.AssigneeID)
	respond(w, http.StatusOK, ticket, err)
}

func (h *TicketHandler) AddLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LinkedTicketID string `json:"linkedTicketId"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	ticket, err := h.svc.AddLink(r.Context(), mustActor(r), chi.URLParam(r, "id"), in.LinkedTicketID)
	respond(w, http.StatusCreated, ticket, err)
}

func (h *TicketHandler) RemoveLink(w http.ResponseWriter, r *http.Request) {
	err := h.svc.RemoveLink(r.Context(), mustActor(r), chi.URLParam(r, "id"), chi.URLParam(r, "linkedId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TicketHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.svc.ListAgents(r.Context())
	respond(w, http.StatusOK, agents, err)
}

func (h *TicketHandler) Summary(w http.ResponseWriter, r *http.Request) {
	sum, err := h.svc.Summary(r.Context(), mustActor(r))
	respond(w, http.StatusOK, sum, err)
}

// Enums feeds the UI dropdowns so values always match the database enums.
func (h *TicketHandler) Enums(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string][]string{
		"types":      domain.Types,
		"severities": domain.Severities,
		"statuses":   domain.Statuses,
	})
}

// respond writes either the error or the success payload.
func respond(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, status, v)
}

// UploadAttachment accepts multipart/form-data with a single "file" field.
func (h *TicketHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxAttachmentBytes+1<<20) // file + form overhead
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, apperr.Validation("Please attach a file (max 5 MB)"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, service.MaxAttachmentBytes+1))
	if err != nil {
		httpx.WriteError(w, apperr.BadRequest("Could not read the uploaded file"))
		return
	}
	att, err := h.svc.AddAttachment(r.Context(), mustActor(r), chi.URLParam(r, "id"), header.Filename, data)
	respond(w, http.StatusCreated, att, err)
}

// DownloadAttachment sends a file as a download (never rendered inline, to avoid XSS).
func (h *TicketHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	att, err := h.svc.GetAttachment(r.Context(), mustActor(r), chi.URLParam(r, "id"), chi.URLParam(r, "attachmentId"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(att.Data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		att.FileName, url.PathEscape(att.FileName)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(att.Data)
}
