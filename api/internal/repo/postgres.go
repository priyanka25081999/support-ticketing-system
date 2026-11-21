package repo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ticketing/api/internal/domain"
)

// dbtx is satisfied by both *pgxpool.Pool and pgx.Tx, so the same queries
// run inside or outside a transaction.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var _ Repository = (*Postgres)(nil)

// Postgres implements Repository using pgx. All queries are parameterised.
type Postgres struct {
	db   dbtx
	pool *pgxpool.Pool // nil when this instance is bound to a transaction
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{db: pool, pool: pool}
}

func (p *Postgres) InTx(ctx context.Context, fn func(Repository) error) error {
	if p.pool == nil { // already inside a transaction
		return fn(p)
	}
	return pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		return fn(&Postgres{db: tx})
	})
}

// mapErr converts driver errors into repository errors.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return ErrDuplicate
		case "22P02": // invalid_text_representation, e.g. malformed UUID
			return ErrNotFound
		}
	}
	return err
}

// ---------- Users ----------

const userColumns = `id::text, email::text, password_hash, full_name, role::text, created_at, deactivated_at`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.CreatedAt, &u.DeactivatedAt)
	return u, mapErr(err)
}

func (p *Postgres) CreateUser(ctx context.Context, u domain.User) (domain.User, error) {
	return scanUser(p.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, full_name, role)
		 VALUES ($1, $2, $3, $4::user_role) RETURNING `+userColumns,
		u.Email, u.PasswordHash, u.FullName, string(u.Role)))
}

func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	return scanUser(p.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

func (p *Postgres) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	return scanUser(p.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1::uuid`, id))
}

func (p *Postgres) GetDefaultAssignee(ctx context.Context) (domain.User, error) {
	return scanUser(p.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE is_default_assignee`))
}

func (p *Postgres) ListStaff(ctx context.Context) ([]domain.StaffMember, error) {
	rows, err := p.db.Query(ctx, `
		SELECT `+userColumns+`,
		       (SELECT count(*)::int FROM tickets t WHERE t.assignee_id = users.id AND t.status IN ('open', 'in_progress', 'on_hold'))
		FROM users
		WHERE role IN ('agent', 'admin') AND deactivated_at IS NULL
		ORDER BY role DESC, full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	staff := []domain.StaffMember{}
	for rows.Next() {
		var m domain.StaffMember
		if err := rows.Scan(&m.ID, &m.Email, &m.PasswordHash, &m.FullName, &m.Role, &m.CreatedAt, &m.DeactivatedAt, &m.OpenTickets); err != nil {
			return nil, err
		}
		staff = append(staff, m)
	}
	return staff, rows.Err()
}

func (p *Postgres) DeactivateAgent(ctx context.Context, agentID, reassignTo, reassignToName, actorID string) (int, error) {
	tag, err := p.db.Exec(ctx,
		`UPDATE users SET deactivated_at = now()
		 WHERE id = $1::uuid AND role = 'agent' AND deactivated_at IS NULL`, agentID)
	if err != nil {
		return 0, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return 0, ErrNotFound
	}
	if _, err := p.db.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1::uuid`, agentID); err != nil {
		return 0, err
	}
	// Reassign unclosed tickets and log one "assigned" event per ticket.
	tag, err = p.db.Exec(ctx, `
		WITH moved AS (
		  UPDATE tickets SET assignee_id = $2::uuid, updated_at = now()
		  WHERE assignee_id = $1::uuid AND status <> 'closed'
		  RETURNING id
		)
		INSERT INTO ticket_events (ticket_id, actor_id, type, data)
		SELECT id, $3::uuid, 'assigned',
		       jsonb_build_object('assigneeId', $2::text, 'assigneeName', $4::text, 'reason', 'agent removed')
		FROM moved`, agentID, reassignTo, actorID, reassignToName)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (p *Postgres) EnsureDefaultAdmin(ctx context.Context, email, passwordHash, name string) error {
	// The display name always follows ADMIN_NAME; the password is only set on first creation.
	if _, err := p.db.Exec(ctx,
		`INSERT INTO users (email, password_hash, full_name, role)
		 VALUES ($1, $2, $3, 'admin')
		 ON CONFLICT (email) DO UPDATE SET full_name = EXCLUDED.full_name`,
		email, passwordHash, name); err != nil {
		return err
	}
	// Make this admin the default assignee only if none is set yet.
	_, err := p.db.Exec(ctx,
		`UPDATE users SET is_default_assignee = true
		 WHERE email = $1 AND role = 'admin'
		   AND NOT EXISTS (SELECT 1 FROM users WHERE is_default_assignee)`, email)
	return err
}

// ---------- Refresh tokens ----------

func (p *Postgres) CreateRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := p.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1::uuid, $2, $3)`,
		userID, tokenHash, expiresAt)
	return mapErr(err)
}

func (p *Postgres) GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error) {
	var t RefreshToken
	err := p.db.QueryRow(ctx,
		`SELECT id::text, user_id::text, expires_at, revoked FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&t.ID, &t.UserID, &t.ExpiresAt, &t.Revoked)
	return t, mapErr(err)
}

func (p *Postgres) RevokeRefreshToken(ctx context.Context, id string) error {
	_, err := p.db.Exec(ctx, `UPDATE refresh_tokens SET revoked = true WHERE id = $1::uuid`, id)
	return mapErr(err)
}

// ---------- Tickets ----------

const ticketSelect = `
SELECT t.id::text, t.ticket_number, t.subject, t.description, t.type::text, t.severity::text,
       t.status::text, t.requester_id::text, r.full_name, t.assignee_id::text, a.full_name,
       t.notify_emails, t.created_at, t.updated_at, t.solved_at
FROM tickets t
JOIN users r ON r.id = t.requester_id
LEFT JOIN users a ON a.id = t.assignee_id`

func scanTicket(row pgx.Row) (domain.Ticket, error) {
	var t domain.Ticket
	var assigneeID, assigneeName *string
	err := row.Scan(&t.ID, &t.TicketNumber, &t.Subject, &t.Description, &t.Type, &t.Severity,
		&t.Status, &t.Requester.ID, &t.Requester.Name, &assigneeID, &assigneeName,
		&t.NotifyEmails, &t.CreatedAt, &t.UpdatedAt, &t.SolvedAt)
	if err != nil {
		return t, mapErr(err)
	}
	if assigneeID != nil {
		t.Assignee = &domain.UserRef{ID: *assigneeID, Name: deref(assigneeName)}
	}
	return t, nil
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (p *Postgres) GetTicket(ctx context.Context, idOrNumber string) (domain.Ticket, error) {
	if uuidPattern.MatchString(idOrNumber) {
		return scanTicket(p.db.QueryRow(ctx, ticketSelect+` WHERE t.id = $1::uuid`, idOrNumber))
	}
	return scanTicket(p.db.QueryRow(ctx, ticketSelect+` WHERE t.ticket_number = $1`, strings.ToUpper(idOrNumber)))
}

func (p *Postgres) CreateTicket(ctx context.Context, t NewTicket) (string, error) {
	var id string
	err := p.db.QueryRow(ctx,
		`INSERT INTO tickets (subject, description, type, severity, requester_id, assignee_id, notify_emails)
		 VALUES ($1, $2, $3::ticket_type, $4::ticket_severity, $5::uuid, $6::uuid, $7)
		 RETURNING id::text`,
		t.Subject, t.Description, t.Type, t.Severity, t.RequesterID, t.AssigneeID, nonNil(t.NotifyEmails)).Scan(&id)
	return id, mapErr(err)
}

// sortOrders whitelists ORDER BY clauses (never interpolate user input).
var sortOrders = map[string]string{
	"newest":   "t.created_at DESC",
	"oldest":   "t.created_at ASC",
	"updated":  "t.updated_at DESC",
	"severity": "t.severity DESC, t.created_at DESC", // enum order: low < ... < critical
}

// buildWhere turns a filter into a WHERE clause and its positional args.
func buildWhere(f domain.TicketFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.RequesterID != "" {
		add("t.requester_id = ?::uuid", f.RequesterID)
	}
	if f.AssigneeID != "" {
		add("t.assignee_id = ?::uuid", f.AssigneeID)
	}
	if len(f.Status) > 0 {
		add("t.status::text = ANY(?::text[])", f.Status)
	}
	if len(f.Type) > 0 {
		add("t.type::text = ANY(?::text[])", f.Type)
	}
	if len(f.Severity) > 0 {
		add("t.severity::text = ANY(?::text[])", f.Severity)
	}
	if f.Search != "" {
		// Matches ticket number, subject or type ("feature request" matches feature_request).
		add("(t.subject ILIKE ? OR t.ticket_number ILIKE ? OR replace(t.type::text, '_', ' ') ILIKE ?)", "%"+escapeLike(f.Search)+"%")
	}
	if f.CreatedFrom != nil {
		add("t.created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil {
		add("t.created_at < ?", *f.CreatedTo)
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (p *Postgres) ListTickets(ctx context.Context, f domain.TicketFilter) ([]domain.Ticket, int, error) {
	where, args := buildWhere(f)

	var total int
	if err := p.db.QueryRow(ctx, `SELECT count(*) FROM tickets t`+where, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	order, ok := sortOrders[f.Sort]
	if !ok {
		order = sortOrders["newest"]
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	query := fmt.Sprintf("%s%s ORDER BY %s LIMIT $%d OFFSET $%d",
		ticketSelect, where, order, len(args)-1, len(args))

	rows, err := p.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	tickets := []domain.Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, 0, err
		}
		tickets = append(tickets, t)
	}
	return tickets, total, rows.Err()
}

func (p *Postgres) UpdateTicketStatus(ctx context.Context, id string, status domain.Status, solvedAt *time.Time) error {
	_, err := p.db.Exec(ctx,
		`UPDATE tickets SET status = $2::ticket_status, solved_at = $3, updated_at = now() WHERE id = $1::uuid`,
		id, string(status), solvedAt)
	return mapErr(err)
}

func (p *Postgres) UpdateTicketAssignee(ctx context.Context, id, assigneeID string) error {
	_, err := p.db.Exec(ctx,
		`UPDATE tickets SET assignee_id = $2::uuid, updated_at = now() WHERE id = $1::uuid`, id, assigneeID)
	return mapErr(err)
}

func (p *Postgres) TouchTicket(ctx context.Context, id string) error {
	_, err := p.db.Exec(ctx, `UPDATE tickets SET updated_at = now() WHERE id = $1::uuid`, id)
	return mapErr(err)
}

func (p *Postgres) CountTickets(ctx context.Context, f domain.TicketFilter) ([]domain.TicketCount, error) {
	// Dashboard counts use the role scope and the created range only.
	where, args := buildWhere(domain.TicketFilter{
		RequesterID: f.RequesterID, AssigneeID: f.AssigneeID, CreatedFrom: f.CreatedFrom, CreatedTo: f.CreatedTo,
	})
	rows, err := p.db.Query(ctx,
		`SELECT t.status::text, t.severity::text, t.type::text, count(*)::int
		 FROM tickets t`+where+` GROUP BY 1, 2, 3`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	counts := []domain.TicketCount{}
	for rows.Next() {
		var c domain.TicketCount
		if err := rows.Scan(&c.Status, &c.Severity, &c.Type, &c.Count); err != nil {
			return nil, err
		}
		counts = append(counts, c)
	}
	return counts, rows.Err()
}

func (p *Postgres) AutoCloseSolved(ctx context.Context, cutoff time.Time) (int64, error) {
	// Close and log the event in one statement so both happen atomically.
	tag, err := p.db.Exec(ctx, `
		WITH closed AS (
			UPDATE tickets SET status = 'closed', updated_at = now()
			WHERE status = 'solved' AND solved_at < $1
			RETURNING id
		)
		INSERT INTO ticket_events (ticket_id, type, data)
		SELECT id, 'status_changed', '{"from":"solved","to":"closed","auto":true}'::jsonb FROM closed`,
		cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---------- Messages & events ----------

func (p *Postgres) CreateMessage(ctx context.Context, m NewMessage) (domain.Message, error) {
	msg := domain.Message{TicketID: m.TicketID, Author: domain.UserRef{ID: m.AuthorID}, Body: m.Body, IsInternal: m.IsInternal}
	err := p.db.QueryRow(ctx,
		`INSERT INTO ticket_messages (ticket_id, author_id, body, is_internal)
		 VALUES ($1::uuid, $2::uuid, $3, $4) RETURNING id::text, created_at`,
		m.TicketID, m.AuthorID, m.Body, m.IsInternal).Scan(&msg.ID, &msg.CreatedAt)
	return msg, mapErr(err)
}

func (p *Postgres) ListMessages(ctx context.Context, ticketID string, includeInternal bool) ([]domain.Message, error) {
	rows, err := p.db.Query(ctx,
		`SELECT m.id::text, m.ticket_id::text, m.author_id::text, u.full_name, u.role::text,
		        m.body, m.is_internal, m.created_at
		 FROM ticket_messages m JOIN users u ON u.id = m.author_id
		 WHERE m.ticket_id = $1::uuid AND ($2 OR NOT m.is_internal)
		 ORDER BY m.created_at`, ticketID, includeInternal)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	msgs := []domain.Message{}
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.TicketID, &m.Author.ID, &m.Author.Name, &m.AuthorRole,
			&m.Body, &m.IsInternal, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (p *Postgres) CreateEvent(ctx context.Context, ticketID, actorID, eventType string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	var actor *string // NULL for system events
	if actorID != "" {
		actor = &actorID
	}
	_, err := p.db.Exec(ctx,
		`INSERT INTO ticket_events (ticket_id, actor_id, type, data)
		 VALUES ($1::uuid, $2::uuid, $3::event_type, $4)`, ticketID, actor, eventType, data)
	return mapErr(err)
}

func (p *Postgres) ListEvents(ctx context.Context, ticketID string) ([]domain.Event, error) {
	rows, err := p.db.Query(ctx,
		`SELECT e.id, e.ticket_id::text, e.actor_id::text, u.full_name, e.type::text, e.data, e.created_at
		 FROM ticket_events e LEFT JOIN users u ON u.id = e.actor_id
		 WHERE e.ticket_id = $1::uuid ORDER BY e.created_at, e.id`, ticketID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	events := []domain.Event{}
	for rows.Next() {
		var e domain.Event
		var actorID, actorName *string
		if err := rows.Scan(&e.ID, &e.TicketID, &actorID, &actorName, &e.Type, &e.Data, &e.CreatedAt); err != nil {
			return nil, err
		}
		if actorID != nil {
			e.Actor = &domain.UserRef{ID: *actorID, Name: deref(actorName)}
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ---------- Attachments ----------

const attachmentColumns = `a.id::text, a.ticket_id::text, a.uploader_id::text, u.full_name,
       a.file_name, a.content_type, a.size_bytes, a.created_at`

func (p *Postgres) CreateAttachment(ctx context.Context, a NewAttachment) (domain.Attachment, error) {
	att := domain.Attachment{TicketID: a.TicketID, Uploader: domain.UserRef{ID: a.UploaderID},
		FileName: a.FileName, ContentType: a.ContentType, SizeBytes: len(a.Data)}
	err := p.db.QueryRow(ctx,
		`INSERT INTO ticket_attachments (ticket_id, uploader_id, file_name, content_type, size_bytes, data)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6) RETURNING id::text, created_at`,
		a.TicketID, a.UploaderID, a.FileName, a.ContentType, len(a.Data), a.Data).Scan(&att.ID, &att.CreatedAt)
	return att, mapErr(err)
}

func (p *Postgres) ListAttachments(ctx context.Context, ticketID string) ([]domain.Attachment, error) {
	rows, err := p.db.Query(ctx, `SELECT `+attachmentColumns+`
		 FROM ticket_attachments a JOIN users u ON u.id = a.uploader_id
		 WHERE a.ticket_id = $1::uuid ORDER BY a.created_at`, ticketID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	list := []domain.Attachment{}
	for rows.Next() {
		var a domain.Attachment
		if err := rows.Scan(&a.ID, &a.TicketID, &a.Uploader.ID, &a.Uploader.Name,
			&a.FileName, &a.ContentType, &a.SizeBytes, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (p *Postgres) CountAttachments(ctx context.Context, ticketID string) (int, error) {
	var n int
	err := p.db.QueryRow(ctx, `SELECT count(*) FROM ticket_attachments WHERE ticket_id = $1::uuid`, ticketID).Scan(&n)
	return n, mapErr(err)
}

func (p *Postgres) GetAttachment(ctx context.Context, ticketID, attachmentID string) (domain.Attachment, error) {
	var a domain.Attachment
	err := p.db.QueryRow(ctx, `SELECT `+attachmentColumns+`, a.data
		 FROM ticket_attachments a JOIN users u ON u.id = a.uploader_id
		 WHERE a.ticket_id = $1::uuid AND a.id = $2::uuid`, ticketID, attachmentID).
		Scan(&a.ID, &a.TicketID, &a.Uploader.ID, &a.Uploader.Name,
			&a.FileName, &a.ContentType, &a.SizeBytes, &a.CreatedAt, &a.Data)
	return a, mapErr(err)
}

// ---------- Reports ----------

// eventScope builds a WHERE clause for status events: role scope on the ticket,
// time range on the event itself.
func eventScope(f domain.TicketFilter) (string, []any) {
	where, args := buildWhere(domain.TicketFilter{RequesterID: f.RequesterID, AssigneeID: f.AssigneeID})
	conds := []string{"e.type = 'status_changed'"}
	if where != "" {
		conds = append(conds, strings.TrimPrefix(where, " WHERE "))
	}
	if f.CreatedFrom != nil {
		args = append(args, *f.CreatedFrom)
		conds = append(conds, fmt.Sprintf("e.created_at >= $%d", len(args)))
	}
	if f.CreatedTo != nil {
		args = append(args, *f.CreatedTo)
		conds = append(conds, fmt.Sprintf("e.created_at < $%d", len(args)))
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func (p *Postgres) ResolutionStats(ctx context.Context, f domain.TicketFilter) (domain.ResolutionStats, error) {
	var s domain.ResolutionStats
	where, args := eventScope(f)
	err := p.db.QueryRow(ctx, `
		SELECT count(DISTINCT e.ticket_id) FILTER (WHERE e.data->>'to' = 'solved'),
		       count(DISTINCT e.ticket_id) FILTER (WHERE e.data->>'to' = 'closed'),
		       count(*) FILTER (WHERE e.data->>'from' = 'solved' AND e.data->>'to' = 'open')
		FROM ticket_events e JOIN tickets t ON t.id = e.ticket_id`+where, args...).
		Scan(&s.Resolved, &s.Closed, &s.Reopened)
	if err != nil {
		return s, mapErr(err)
	}

	// Average time from creation to resolution for tickets solved inside the range.
	where, args = buildWhere(domain.TicketFilter{RequesterID: f.RequesterID, AssigneeID: f.AssigneeID})
	conds := []string{"t.solved_at IS NOT NULL"}
	if where != "" {
		conds = append(conds, strings.TrimPrefix(where, " WHERE "))
	}
	if f.CreatedFrom != nil {
		args = append(args, *f.CreatedFrom)
		conds = append(conds, fmt.Sprintf("t.solved_at >= $%d", len(args)))
	}
	if f.CreatedTo != nil {
		args = append(args, *f.CreatedTo)
		conds = append(conds, fmt.Sprintf("t.solved_at < $%d", len(args)))
	}
	err = p.db.QueryRow(ctx,
		`SELECT avg(extract(epoch FROM t.solved_at - t.created_at)) / 3600 FROM tickets t WHERE `+strings.Join(conds, " AND "),
		args...).Scan(&s.AvgResolutionHours)
	return s, mapErr(err)
}

func (p *Postgres) TicketTrend(ctx context.Context, f domain.TicketFilter, bucket string, loc *time.Location) ([]domain.TrendPoint, error) {
	createdWhere, createdArgs := buildWhere(domain.TicketFilter{
		RequesterID: f.RequesterID, AssigneeID: f.AssigneeID, CreatedFrom: f.CreatedFrom, CreatedTo: f.CreatedTo,
	})
	eventWhere, eventArgs := eventScope(f)
	// Renumber the event placeholders so both halves share one argument list.
	offset := len(createdArgs)
	eventWhere = regexp.MustCompile(`\$(\d+)`).ReplaceAllStringFunc(eventWhere, func(m string) string {
		var n int
		fmt.Sscanf(m, "$%d", &n)
		return fmt.Sprintf("$%d", n+offset)
	})
	args := append(append(createdArgs, eventArgs...), bucket, loc.String())
	b, tz := len(args)-1, len(args)

	rows, err := p.db.Query(ctx, fmt.Sprintf(`
		WITH c AS (
		  SELECT date_trunc($%[1]d::text, t.created_at AT TIME ZONE $%[2]d::text) AS bucket, count(*)::int AS n
		  FROM tickets t%[3]s GROUP BY 1
		), r AS (
		  SELECT date_trunc($%[1]d::text, e.created_at AT TIME ZONE $%[2]d::text) AS bucket, count(DISTINCT e.ticket_id)::int AS n
		  FROM ticket_events e JOIN tickets t ON t.id = e.ticket_id%[4]s AND e.data->>'to' = 'solved' GROUP BY 1
		)
		SELECT COALESCE(c.bucket, r.bucket), COALESCE(c.n, 0), COALESCE(r.n, 0)
		FROM c FULL JOIN r ON c.bucket = r.bucket ORDER BY 1`, b, tz, createdWhere, eventWhere), args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	points := []domain.TrendPoint{}
	for rows.Next() {
		var wall time.Time // local wall-clock time in loc (timestamp without time zone)
		var pt domain.TrendPoint
		if err := rows.Scan(&wall, &pt.Created, &pt.Resolved); err != nil {
			return nil, err
		}
		pt.Bucket = time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), 0, 0, 0, loc)
		points = append(points, pt)
	}
	return points, rows.Err()
}

// ---------- Chat ----------

func (p *Postgres) CreateChatMessage(ctx context.Context, ticketID, senderID, body string) (domain.ChatMessage, error) {
	m := domain.ChatMessage{TicketID: ticketID, Sender: domain.UserRef{ID: senderID}, Body: body}
	err := p.db.QueryRow(ctx,
		`INSERT INTO ticket_chat_messages (ticket_id, sender_id, body) VALUES ($1::uuid, $2::uuid, $3)
		 RETURNING id::text, created_at`, ticketID, senderID, body).Scan(&m.ID, &m.CreatedAt)
	return m, mapErr(err)
}

func (p *Postgres) ListChatMessages(ctx context.Context, ticketID string, limit int) ([]domain.ChatMessage, error) {
	// Latest N messages, returned oldest first.
	rows, err := p.db.Query(ctx, `
		SELECT * FROM (
		  SELECT c.id::text, c.ticket_id::text, c.sender_id::text, u.full_name, u.role::text, c.body, c.created_at
		  FROM ticket_chat_messages c JOIN users u ON u.id = c.sender_id
		  WHERE c.ticket_id = $1::uuid ORDER BY c.created_at DESC LIMIT $2
		) latest ORDER BY created_at`, ticketID, limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	msgs := []domain.ChatMessage{}
	for rows.Next() {
		var m domain.ChatMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.Sender.ID, &m.Sender.Name, &m.SenderRole, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ---------- Notifications ----------

// notificationCTE builds "marked": activity by others on tickets the user
// requested or is assigned to, each flagged unread if newer than both the
// user's mark-all watermark and their per-ticket read time.
// $1 = user id, $2 = include internal notes.
const notificationCTE = `
WITH scope AS (
  SELECT id, ticket_number, subject, requester_id FROM tickets
  WHERE requester_id = $1::uuid OR assignee_id = $1::uuid
),
items AS (
  SELECT e.id::text AS id, 'event' AS kind, e.type::text AS type, e.ticket_id, e.actor_id,
         e.data, ''::text AS preview, e.created_at
  FROM ticket_events e JOIN scope s ON s.id = e.ticket_id
  WHERE e.actor_id IS DISTINCT FROM $1::uuid
    -- skip the automatic assignment logged when the requester creates the ticket
    AND NOT (e.type = 'assigned' AND e.actor_id = s.requester_id)
  UNION ALL
  SELECT m.id::text, 'message', CASE WHEN m.is_internal THEN 'internal_note' ELSE 'comment' END,
         m.ticket_id, m.author_id, '{}'::jsonb, left(m.body, 140), m.created_at
  FROM ticket_messages m JOIN scope s ON s.id = m.ticket_id
  WHERE m.author_id <> $1::uuid AND ($2::boolean OR NOT m.is_internal)
  UNION ALL
  SELECT a.id::text, 'attachment', 'attachment', a.ticket_id, a.uploader_id,
         jsonb_build_object('fileName', a.file_name), '', a.created_at
  FROM ticket_attachments a JOIN scope s ON s.id = a.ticket_id
  WHERE a.uploader_id <> $1::uuid
  UNION ALL
  SELECT c.id::text, 'chat', 'chat', c.ticket_id, c.sender_id, '{}'::jsonb, left(c.body, 140), c.created_at
  FROM ticket_chat_messages c JOIN scope s ON s.id = c.ticket_id
  WHERE c.sender_id <> $1::uuid
),
marked AS (
  SELECT i.*, i.created_at > GREATEST(u.notifications_read_at, COALESCE(tr.read_at, '-infinity'::timestamptz)) AS unread
  FROM items i
  CROSS JOIN (SELECT notifications_read_at FROM users WHERE id = $1::uuid) u
  LEFT JOIN ticket_reads tr ON tr.user_id = $1::uuid AND tr.ticket_id = i.ticket_id
)`

func (p *Postgres) ListNotifications(ctx context.Context, userID string, includeInternal bool, limit int) (domain.NotificationFeed, error) {
	feed := domain.NotificationFeed{UnreadTicketIDs: []string{}, Items: []domain.Notification{}}

	rows, err := p.db.Query(ctx, notificationCTE+`
		SELECT ticket_id::text, count(*)::int FROM marked WHERE unread GROUP BY ticket_id`,
		userID, includeInternal)
	if err != nil {
		return feed, mapErr(err)
	}
	for rows.Next() {
		var ticketID string
		var n int
		if err := rows.Scan(&ticketID, &n); err != nil {
			rows.Close()
			return feed, err
		}
		feed.UnreadTicketIDs = append(feed.UnreadTicketIDs, ticketID)
		feed.UnreadCount += n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return feed, err
	}

	rows, err = p.db.Query(ctx, notificationCTE+`
		SELECT m.id, m.kind, m.type, s.id::text, s.ticket_number, s.subject,
		       COALESCE(u.full_name, 'System'), m.data, m.preview, m.created_at, m.unread
		FROM marked m
		JOIN scope s ON s.id = m.ticket_id
		LEFT JOIN users u ON u.id = m.actor_id
		ORDER BY m.created_at DESC, m.id
		LIMIT $3`, userID, includeInternal, limit)
	if err != nil {
		return feed, mapErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(&n.ID, &n.Kind, &n.Type, &n.TicketID, &n.TicketNumber, &n.Subject,
			&n.ActorName, &n.Data, &n.Preview, &n.CreatedAt, &n.Unread); err != nil {
			return feed, err
		}
		feed.Items = append(feed.Items, n)
	}
	return feed, rows.Err()
}

func (p *Postgres) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	_, err := p.db.Exec(ctx, `UPDATE users SET notifications_read_at = now() WHERE id = $1::uuid`, userID)
	return mapErr(err)
}

func (p *Postgres) MarkTicketRead(ctx context.Context, userID, ticketID string) error {
	_, err := p.db.Exec(ctx,
		`INSERT INTO ticket_reads (user_id, ticket_id) VALUES ($1::uuid, $2::uuid)
		 ON CONFLICT (user_id, ticket_id) DO UPDATE SET read_at = now()`, userID, ticketID)
	return mapErr(err)
}

// ---------- Links ----------

func (p *Postgres) CreateLink(ctx context.Context, ticketID, linkedID string) error {
	// Links are treated as undirected: skip if a link exists in either direction.
	tag, err := p.db.Exec(ctx,
		`INSERT INTO ticket_links (ticket_id, linked_ticket_id)
		 SELECT $1::uuid, $2::uuid
		 WHERE NOT EXISTS (
		   SELECT 1 FROM ticket_links
		   WHERE (ticket_id = $1::uuid AND linked_ticket_id = $2::uuid)
		      OR (ticket_id = $2::uuid AND linked_ticket_id = $1::uuid))`, ticketID, linkedID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDuplicate
	}
	return nil
}

func (p *Postgres) DeleteLink(ctx context.Context, ticketID, linkedID string) error {
	tag, err := p.db.Exec(ctx,
		`DELETE FROM ticket_links
		 WHERE (ticket_id = $1::uuid AND linked_ticket_id = $2::uuid)
		    OR (ticket_id = $2::uuid AND linked_ticket_id = $1::uuid)`, ticketID, linkedID)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) ListLinks(ctx context.Context, ticketID string) ([]domain.LinkedTicket, error) {
	rows, err := p.db.Query(ctx,
		`SELECT o.id::text, o.ticket_number, o.subject, o.status::text, l.relation, l.created_at
		 FROM ticket_links l
		 JOIN tickets o ON o.id = CASE WHEN l.ticket_id = $1::uuid THEN l.linked_ticket_id ELSE l.ticket_id END
		 WHERE l.ticket_id = $1::uuid OR l.linked_ticket_id = $1::uuid
		 ORDER BY l.created_at`, ticketID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	links := []domain.LinkedTicket{}
	for rows.Next() {
		var l domain.LinkedTicket
		if err := rows.Scan(&l.ID, &l.TicketNumber, &l.Subject, &l.Status, &l.Relation, &l.CreatedAt); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// nonNil avoids writing NULL into NOT NULL array columns.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
