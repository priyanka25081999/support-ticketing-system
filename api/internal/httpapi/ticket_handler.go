package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

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
	rng, err := service.ParseReportRange(q.Get("createdFrom"), q.Get("createdTo"), "")
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	filter := domain.TicketFilter{
		Status:      listParam(q, "status"),
		Type:        listParam(q, "type"),
		Severity:    listParam(q, "severity"),
		AssigneeIDs: listParam(q, "assignee"),
		Search:      q.Get("search"),
		Sort:        q.Get("sort"),
		Page:        page,
		PageSize:    pageSize,
		CreatedFrom: rng.From,
		CreatedTo:   rng.To,
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

// Summary accepts optional ?from=&to= (RFC 3339) and ?tz= (IANA zone for trend buckets).
func (h *TicketHandler) Summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rng, err := service.ParseReportRange(q.Get("from"), q.Get("to"), q.Get("tz"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	sum, err := h.svc.Summary(r.Context(), mustActor(r), rng)
	respond(w, http.StatusOK, sum, err)
}

// Export streams tickets created in the range as CSV (agents: assigned tickets; admins: all).
func (h *TicketHandler) Export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rng, err := service.ParseReportRange(q.Get("from"), q.Get("to"), q.Get("tz"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	tickets, err := h.svc.ExportTickets(r.Context(), mustActor(r), rng)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tickets-%s.csv"`, time.Now().In(rng.Location).Format("20060102-1504")))
	if err := writeTicketsCSV(w, tickets, rng.Location); err != nil {
		slog.Error("csv export failed", "err", err)
	}
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

func (h *TicketHandler) Notifications(w http.ResponseWriter, r *http.Request) {
	feed, err := h.svc.Notifications(r.Context(), mustActor(r))
	respond(w, http.StatusOK, feed, err)
}

// MarkNotificationsRead accepts {"ticketId": "..."} or {} to mark everything read.
func (h *TicketHandler) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TicketID string `json:"ticketId"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.svc.MarkNotificationsRead(r.Context(), mustActor(r), in.TicketID); err != nil {
		httpx.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Chat ----------

func (h *TicketHandler) ChatThread(w http.ResponseWriter, r *http.Request) {
	thread, err := h.svc.ChatThread(r.Context(), mustActor(r), chi.URLParam(r, "id"))
	respond(w, http.StatusOK, thread, err)
}

func (h *TicketHandler) SendChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Body string `json:"body"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.WriteError(w, err)
		return
	}
	msg, err := h.svc.SendChat(r.Context(), mustActor(r), chi.URLParam(r, "id"), in.Body)
	respond(w, http.StatusCreated, msg, err)
}

const chatPingInterval = 25 * time.Second

// ChatStream is a Server-Sent Events stream of new chat messages. Over HTTPS
// it runs on HTTP/2, so many streams share one connection.
func (h *TicketHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.WriteError(w, fmt.Errorf("streaming not supported"))
		return
	}
	msgs, unsubscribe, err := h.svc.SubscribeChat(r.Context(), mustActor(r), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	ping := time.NewTicker(chatPingInterval)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg, open := <-msgs:
			if !open {
				return // dropped as a slow consumer; the client reconnects
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n") // keeps proxies from closing an idle stream
			flusher.Flush()
		}
	}
}

// listParam reads a multi-value filter given as ?status=open,on_hold or repeated ?status=open&status=on_hold.
func listParam(q url.Values, key string) []string {
	var out []string
	for _, raw := range q[key] {
		for _, v := range strings.Split(raw, ",") {
			if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
				out = append(out, v)
			}
		}
	}
	return out
}

// RemoveAgent deactivates an agent and reassigns their open tickets to the admin.
func (h *TicketHandler) RemoveAgent(w http.ResponseWriter, r *http.Request) {
	moved, err := h.svc.RemoveAgent(r.Context(), mustActor(r), chi.URLParam(r, "id"))
	respond(w, http.StatusOK, map[string]int{"reassignedTickets": moved}, err)
}
