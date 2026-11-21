# Support Desk: Demo Guide

Contents: the opening script, the feature list, step-by-step code flows starting from the login page, the tech-stack rationale, and the likely Q&A.

Architecture diagram for the demo: open [architecture.html](./architecture.html) in a browser. It has the system diagram, the request lifecycle and the data model.

---

## 1. Opening script (about 2 minutes)

> **Hello everyone, today we will be giving a demo of _Support Desk_, a full-stack customer support ticketing system, a "mini-Zendesk" that we built from scratch.**
>
> Support teams usually handle requests across email threads and spreadsheets, so tickets get lost, nobody knows who owns what, and customers get no visibility. Support Desk puts all of that in one place.
>
> There are **three roles**:
> - **Customers** register, raise tickets with attachments, track progress, reply, and reopen a ticket if the fix didn't work.
> - **Agents** work the queue assigned to them. They comment, add internal notes the customer can't see, move tickets through a defined lifecycle, and hand tickets off to colleagues.
> - **Admins** see everything. They manage the agent team, close tickets, and use the dashboard and CSV reports.
>
> Every new ticket is **auto-assigned**, gets a readable number like **TKT-000001**, and moves through a **status state machine**: Open → In progress → On hold → Solved → Closed. Solved tickets **auto-close after 7 days** unless the customer reopens them.
>
> For collaboration there is a **private live chat** between the customer and the assigned agent, a **notification bell** with unread tracking, and **live auto-refresh**, so everyone sees changes within seconds without reloading.
>
> Technically, it's a **React + TypeScript** single-page app talking to a **Go REST API** backed by **PostgreSQL**. Security is built in: JWT access tokens kept only in memory, rotating refresh tokens in HttpOnly cookies, bcrypt passwords, role-based access checks on every request, and rate-limited login.
>
> I'll start with the architecture, then walk through the product as a customer, an agent and an admin.

### Suggested click-path

| # | Who | What to show |
|---|-----|--------------|
| 1 | — | `architecture.html`: walk left to right: Browser → Vite proxy → Go API layers → Postgres |
| 2 | Customer | Register → lands on **Home** (greeting, quick links, active tickets) |
| 3 | Customer | **New Ticket**: subject, type, severity, notify emails, drag-and-drop a file, link an old solved ticket → submit → banner "created and assigned to Admin" |
| 4 | Admin (2nd browser / incognito) | Bell lights up → open the ticket → **reassign** to an agent |
| 5 | Agent | Ticket appears in **Ticket Queue** (blue unread dot) → move to **In progress** → add a **comment** and an **internal note** |
| 6 | Customer | Sees the comment (not the internal note) within ~10s via **Live** refresh; the **History** tab shows the audit trail |
| 7 | Customer ↔ Agent | Open **💬 Chat** on both sides → messages appear instantly (SSE) |
| 8 | Agent | Move to **On hold** → customer replies → the ticket **automatically returns to In progress** |
| 9 | Agent → Admin | **Solved** → customer can **Reopen** (within 7 days) → admin **Closes** |
| 10 | Admin | **Dashboard**: KPIs, created-vs-resolved trend, time ranges, **Export CSV** |
| 11 | Admin | **Agents** page: add an agent, remove an agent → their open tickets move to the admin |
| 12 | Any | Ticket list: multi-select filters, search, sort, table/cards, copy the URL (filters are in it) |

Tip: use separate browsers or an incognito window for each role. The session lives in a cookie, so two tabs in the same browser share one user.

---

## 2. Feature list

### Authentication and accounts
- Customer self-registration (name, email, password 8–72 chars)
- Login with email + password; generic error so attackers can't tell which emails exist
- **Session persistence**: reloading the page keeps you logged in (refresh cookie)
- **Silent token refresh**: the 15-minute access token renews automatically
- Logout revokes the session on the server
- Default **admin seeded** at startup from `.env`
- Admin creates **agent** accounts; admin **removes** agents (soft delete; their open tickets go back to the admin)
- Rate limit: 30 auth requests per minute per IP

### Role-based access (RBAC)
| Capability | Customer | Agent | Admin |
|---|:-:|:-:|:-:|
| Create ticket | ✅ | — | — |
| See tickets | own | assigned to them | all |
| Comment | ✅ | ✅ | ✅ |
| Internal notes (hidden from customer) | — | ✅ | ✅ |
| Change status | reopen only (within 7 days) | all except Close | all |
| Reassign | — | own tickets (hand-off) | any |
| Link tickets | owner | — | — |
| Chat | reporter ↔ assignee (admin read-only unless assignee) | ✅ | ✅ |
| Filter by assignee | — | — | ✅ |
| CSV export | — | ✅ (own) | ✅ (all) |
| Manage agents | — | — | ✅ |

### Tickets
- Create with subject, description, **type** (technical, billing, account, feature request, other), **severity** (low → critical), **notification emails** (`;`-separated, validated, de-duplicated, max 10)
- Auto-generated **ticket number** `TKT-000001` (Postgres sequence)
- **Auto-assignment** to the default admin
- **Attachments**: drag and drop, up to 10 files of 5 MB each per ticket. The server detects the real file type, cleans the file name, and always sends files as downloads.
- **Link previous tickets**: only your own solved/closed tickets; links work in both directions
- **Comments** and **internal notes**
- **Timeline**: Comments tab plus History tab (created, assigned, status changed, linked)
- **Status lifecycle** enforced on the server: `open → in_progress / on_hold / solved`, `solved → open / closed`, `closed` is final
- **Auto-reopen**: a customer reply on an *On hold* ticket moves it back to *In progress*
- **Auto-close**: solved for more than 7 days → closed (hourly background job)
- **Reassign** with workload shown ("· 3 open")

### Ticket list / queue
- Multi-select filters: status, type, severity, assignee (admin)
- Search by ticket number, subject or type (300 ms debounce)
- Created-date range (presets: last 1h … 1y, all time, or custom)
- Sort: newest, oldest, recently updated, highest severity
- Pagination (10/20/50 per page)
- **Table / Cards** view toggle (remembered)
- All filters live in the **URL**: shareable, bookmarkable, survive reloads
- **Unread dot** on tickets with new activity

### Collaboration and live updates
- **Live chat**: private 1:1 reporter ↔ assignee, real-time over **Server-Sent Events**, auto-reconnect
- **Notification bell**: activity by others on your tickets (status, assignment, comments, internal notes for staff, attachments, chat), unread count in the browser tab title, "Mark all as read", and opening a ticket marks its items as read
- **Live auto-refresh**: Off / 5s / 10s (default) / 30s / 1m / 5m / 15m / 30m / 1h, plus a manual Refresh button and an "Updated HH:MM:SS" time

### Dashboard and reporting
- KPIs: **Created, Resolved, Closed, Reopened, Avg. resolution time**
- Breakdown bars by **severity** and **type** (click a bar to open the filtered list)
- **Created vs Resolved trend chart**, bucketed per hour / day / month depending on range, in the browser's time zone
- Time-range picker with presets, custom ranges, and back/forward shifting
- **Recently updated** tickets
- **Export CSV** (agents and admins), protected against formula injection
- Scoped by role: customers see their own numbers, agents their queue, admins everything

### Home page
- Greeting by time of day, role-specific tagline and quick links
- "Your active tickets" (open, in progress, on hold)
- Unread updates hint

### Non-functional
- Consistent JSON error format `{"error":{"code","message"}}` and friendly UI messages
- Health checks: `/healthz`, `/healthcheck`, `/api/v1/healthcheck` (includes a DB ping)
- Migrations applied automatically on startup
- Graceful shutdown; optional HTTPS with HTTP/2
- Unit tests: Go (services, state machine, auth, router, CSV, query builder) and Jest + React Testing Library (components, utils)

---

## 3. Architecture in one page

```
Browser (React SPA)  ──HTTP──►  Vite :5180  ──proxy /api──►  Go API :8080  ──pgx──►  PostgreSQL :5435
  pages, hooks,                 (dev server)                  middleware                 tables, enums,
  TanStack Query cache                                         → handlers                 FKs, indexes
  axios + SSE reader                                           → services (rules)
                                                               → repository (SQL)
```

**Backend layering** (all under [api/internal](../api/internal)):

| Layer | Package | Responsibility |
|---|---|---|
| Entry | [cmd/server/main.go](../api/cmd/server/main.go) | Load config, connect DB, migrate, seed admin, start auto-close job, start HTTP server, graceful shutdown |
| Routing + middleware | [httpapi/router.go](../api/internal/httpapi/router.go) | Routes, request ID, logging, panic recovery, timeout, CORS, rate limit, JWT auth, role checks |
| Handlers | [httpapi/auth_handler.go](../api/internal/httpapi/auth_handler.go), [httpapi/ticket_handler.go](../api/internal/httpapi/ticket_handler.go) | Parse HTTP input, call the service, write JSON |
| Services | [service/](../api/internal/service) | **All business rules**: access control, state machine, validation, auto-assign |
| Repository | [repo/postgres.go](../api/internal/repo/postgres.go) | SQL only. The `Repository` interface lets tests use [memrepo](../api/internal/repo/memrepo/memrepo.go) |
| Auth helpers | [auth/](../api/internal/auth) | bcrypt, JWT, refresh-token hashing, middleware |
| Errors | [apperr](../api/internal/apperr/apperr.go), [httpx](../api/internal/httpx/httpx.go) | Typed errors → HTTP status + stable error code |

**Frontend layering** (under [web/src](../web/src)):

| Layer | Files | Responsibility |
|---|---|---|
| App shell | [App.tsx](../web/src/App.tsx), [Layout.tsx](../web/src/components/Layout.tsx) | Providers, routes, top nav |
| Auth | [AuthContext.tsx](../web/src/auth/AuthContext.tsx), [ProtectedRoute.tsx](../web/src/auth/ProtectedRoute.tsx) | Session state, route guards |
| API | [client.ts](../web/src/api/client.ts), [endpoints.ts](../web/src/api/endpoints.ts), [types.ts](../web/src/api/types.ts) | Axios instance, token refresh, typed endpoint functions |
| Data hooks | [queries.ts](../web/src/hooks/queries.ts), [notifications.ts](../web/src/hooks/notifications.ts), [useChat.ts](../web/src/hooks/useChat.ts), [useLiveRefresh.tsx](../web/src/hooks/useLiveRefresh.tsx) | TanStack Query wrappers, polling, SSE |
| Pages | [pages/](../web/src/pages) | One component per screen |
| Components | [components/](../web/src/components) | Select, MultiSelect, FileInput, TimeRangePicker, TrendChart, ChatWindow, NotificationBell… |

**Golden rule to mention:** the UI hides buttons the user can't use, but **the server is the source of truth**. Every rule is enforced again in the service layer.

---

## 4. Feature-by-feature flows (UI → API → DB)

Notation: `UI` = React, `API` = HTTP call, `H` = handler, `S` = service, `R` = repository / SQL, `DB` = tables touched.

### 4.0 App boot and session restore (before the login page appears)

1. [main.tsx](../web/src/main.tsx) renders `<App/>`.
2. [App.tsx](../web/src/App.tsx) wraps the app in `ErrorBoundary → QueryClientProvider → LiveRefreshProvider → BrowserRouter → AuthProvider`.
3. `AuthProvider` starts with `status = 'loading'` and calls `refreshSession()` ([client.ts](../web/src/api/client.ts)):
   - `POST /api/v1/auth/refresh` with the **HttpOnly cookie** (the browser attaches it automatically; JavaScript can't read it).
4. **H** `AuthHandler.Refresh` → **S** `AuthService.Refresh`:
   - SHA-256 hashes the cookie value → `GetRefreshToken(hash)`
   - rejects if missing, revoked or expired; rejects deactivated users
   - **in one transaction**: revokes the old token and issues a new access JWT plus a new refresh token (**rotation**)
5. Success: the cookie is replaced, `{accessToken, user}` is returned, the token is stored **in memory**, and status becomes `authenticated`.
   Failure: status becomes `anonymous` and `ProtectedRoute` redirects to `/login`.

DB: `refresh_tokens` (read, update `revoked`, insert), `users` (read).

### 4.1 Register (customer)

1. **UI** [AuthPages.tsx](../web/src/pages/AuthPages.tsx) `RegisterPage` checks the form in the browser (required fields, `minLength=8`, `maxLength=72`).
2. `authApi.register()` → `POST /api/v1/auth/register` (rate-limited 30/min/IP).
3. **H** `AuthHandler.Register` → `httpx.DecodeJSON` (1 MB limit, **unknown fields rejected**).
4. **S** `AuthService.Register` → `createUser(..., RoleCustomer)`:
   - `normalizeEmail` (trim + lowercase), `validateEmail` (`net/mail`), `validatePassword` (8–72; bcrypt ignores bytes past 72), `requireLength(name, 1..100)`
   - `auth.HashPassword` → **bcrypt cost 12**
5. **R** `CreateUser` → `INSERT INTO users … RETURNING …`. A duplicate email hits the `UNIQUE` constraint (`23505`), which becomes `ErrDuplicate` and then **409** "An account with this email already exists".
6. The UI then logs in automatically (4.2) and navigates to `/home`.

DB: `users`.

### 4.2 Login

1. **UI** `LoginPage` → `AuthContext.login(email, password)` → `authApi.login` → `POST /api/v1/auth/login`.
2. **H** `AuthHandler.Login` → **S** `AuthService.Login`:
   - `GetUserByEmail` (email is **CITEXT**, so case-insensitive)
   - `auth.CheckPassword` (bcrypt compare). Unknown email and wrong password return the **same 401** ("Invalid email or password") so attackers can't probe which emails exist.
   - Deactivated user → 401 "account has been deactivated"
   - `issueTokens`:
     - **Access token**: JWT **HS256**, claims `sub`(user id), `role`, `name`, `iat`, `exp` = 15 min ([auth.go](../api/internal/auth/auth.go) `TokenManager.Issue`)
     - **Refresh token**: 32 random bytes (base64url) → only the **SHA-256 hash** is stored, expiry 7 days
3. **H** sets cookie `refresh_token`: `HttpOnly`, `SameSite=Lax`, `Path=/api/v1/auth` (sent only to auth endpoints), `Secure` when `COOKIE_SECURE=true`.
4. Response `{accessToken, user}` → `setAccessToken()` (memory) → `navigate(from ?? '/home')`. You return to the page you originally requested.

DB: `users` (read), `refresh_tokens` (insert).

### 4.3 Authenticated requests, token refresh and logout

- Every request goes through the axios instance `api` ([client.ts](../web/src/api/client.ts)):
  - **request interceptor** adds `Authorization: Bearer <token>`
  - **response interceptor**: on **401** (not for `/auth/*`) it calls `refreshSession()` once and **replays** the original request. Parallel 401s share **one** in-flight refresh promise, so the refresh token isn't rotated twice. If the refresh fails, it calls `onSessionExpired`, which clears the session, empties the query cache and redirects to login.
- Server side, [middleware.go](../api/internal/auth/middleware.go):
  - `Authenticate` parses the Bearer JWT (`jwt.WithValidMethods(HS256)` blocks algorithm-confusion attacks) and stores `Actor{ID, Role, Name}` in the request context.
  - `RequireRole(...)` returns 403 if the role isn't allowed (e.g. `POST /tickets` is customer-only, `/agents` POST and DELETE are admin-only).
- **Logout**: `POST /auth/logout` uses **only the cookie** (so it works even with an expired access token) → `AuthService.Logout` revokes the token row → the cookie is cleared → the UI calls `clearSession()`.

### 4.4 Routing guards and navigation

- [ProtectedRoute.tsx](../web/src/auth/ProtectedRoute.tsx): shows a loader while the session is restored, redirects to `/login` (remembering `from`) when logged out, and shows "no permission" for the wrong role (`/tickets/new` is customer-only, `/agents` is admin-only).
- `PublicOnly` redirects logged-in users away from `/login` and `/register`.
- [Layout.tsx](../web/src/components/Layout.tsx) builds a **role-aware nav**: "My Tickets" vs "Ticket Queue", "New Ticket" for customers only, "Agents" for admins only. It also shows the **NotificationBell**, the user's name and role, and Logout.

### 4.5 Home page

[HomePage.tsx](../web/src/pages/HomePage.tsx)
- `greeting(hour)` for the time-of-day greeting, role-specific tagline and quick-link cards.
- `useTickets({status:'open,in_progress,on_hold', sort:'updated', pageSize:5})` → `GET /tickets?...`: active tickets (scoped by role on the server, see 4.7).
- `useNotifications()` provides the unread count for the hint chip.

### 4.6 Create ticket (customer)

1. **UI** [NewTicketPage.tsx](../web/src/pages/NewTicketPage.tsx):
   - Dropdown values come from `useEnums()` → `GET /meta/enums`, so the UI **always matches the DB enums**. Cached forever (`staleTime: Infinity`).
   - Notify emails are pre-filled with the user's email; `validateEmailList` checks them in the browser.
   - [FileInput](../web/src/components/FileInput.tsx): drag and drop, rejects files over 5 MB or empty, max 10.
   - [LinkTicketPicker](../web/src/components/LinkTicketPicker.tsx): shows the user's own **solved/closed** tickets.
2. `useMutation` runs:
   1. `POST /api/v1/tickets` `{subject, description, type, severity, notifyEmails, linkedTicketIds}`
   2. **then** uploads each file with `POST /tickets/{id}/attachments`. Uploads happen *after* the ticket exists, so **a failed upload never loses the ticket**. Failures are shown as a banner.
3. Server: route guarded by `RequireRole(customer)` → **H** `TicketHandler.Create` → **S** `TicketService.Create`:
   - validation: subject 3–200, description 1–5000, type/severity in the enum list, `ParseEmailList` (split on `;`, lowercase, de-duplicate, validate, 1..10), at most 10 links
   - **one DB transaction** (`repo.InTx`):
     - `GetDefaultAssignee()` (the admin with `is_default_assignee = true`)
     - `CreateTicket` → `ticket_number` comes from `DEFAULT 'TKT-' || lpad(nextval('ticket_number_seq'), 6, '0')`
     - `CreateEvent(created)` + `CreateEvent(assigned, {assigneeId, assigneeName})`
     - for each linked id, `link()` checks it's the same requester, not itself, solved/closed and not already linked, then `CreateLink` + `CreateEvent(linked)`
   - If any step fails, everything rolls back.
4. Attachment upload: **H** `UploadAttachment` limits the body (`MaxBytesReader`), reads the multipart `file` → **S** `AddAttachment`:
   - `loadAccessible`, not closed, 0 < size ≤ 5 MB, count < 10
   - `SanitizeFileName` (strips paths, quotes and control characters)
   - `http.DetectContentType(data)` (**ignores the client-sent MIME type**)
   - `INSERT INTO ticket_attachments (… data BYTEA)` then `TouchTicket` (updates `updated_at`)
5. UI invalidates `tickets` and `summary` and navigates to `/tickets/TKT-…` with a "created and assigned to Admin" banner.

DB: `tickets`, `ticket_events`, `ticket_links`, `ticket_attachments`.

### 4.7 Ticket list / queue

1. **UI** [TicketListPage.tsx](../web/src/pages/TicketListPage.tsx): every filter is read from and written to `useSearchParams()` (URL). Search is debounced 300 ms. The range comes from [timeRange.ts](../web/src/utils/timeRange.ts); relative presets are **resolved at fetch time** so live refresh always uses "now".
2. `useTickets(filters, range)` → `GET /tickets?status=open,on_hold&type=…&severity=…&assignee=…&search=…&sort=…&page=…&pageSize=…&createdFrom=…&createdTo=…`
   - `keepPreviousData` keeps the table on screen while filters change.
3. **H** `TicketHandler.List`: `listParam` accepts `a,b` or repeated params; `ParseReportRange` validates RFC 3339 dates.
4. **S** `TicketService.List`:
   - `validateFilter`: enum values must be valid, search ≤ 100 characters, page ≥ 1, pageSize 1..100 (default 20), from < to, and **client-supplied scope fields are wiped**
   - only admins may filter by assignee
   - `applyScope`: **customer → `requester_id = me`**, **agent → `assignee_id = me`**, admin → no restriction
5. **R** `ListTickets`:
   - `buildWhere` builds a **parameterised** `WHERE` (`$1, $2…`). Multi-select uses `= ANY($n::text[])`. Search uses `ILIKE` on subject / ticket_number / type with **LIKE wildcards escaped**.
   - `count(*)` for the total, then `ORDER BY` from a **whitelist map** (never user text) `LIMIT/OFFSET`.
   - `ticketSelect` joins `users` twice (requester and assignee names).
6. Unread dots: `useNotifications().unreadTicketIds` (4.12).

DB: `tickets`, `users`. Indexes: `tickets_requester_idx`, `tickets_assignee_idx`, `tickets_status_idx`, `tickets_created_idx`.

### 4.8 Ticket detail page

[TicketDetailPage.tsx](../web/src/pages/TicketDetailPage.tsx) makes **two parallel queries**:
- `useTicket(id)` → `GET /tickets/{id}` → **S** `Get` → `loadAccessible` + `detail()` (links + attachment metadata; the file bytes aren't loaded)
- `useTimeline(id)` → `GET /tickets/{id}/timeline` → **S** `Timeline` → `ListMessages(includeInternal = isStaff)` + `ListEvents` → merged and **sorted by time**

Key function: **`loadAccessible`** ([ticket_service.go](../api/internal/service/ticket_service.go)):
- `GetTicket(idOrNumber)` accepts a **UUID or `TKT-000123`** (regex check)
- `canView`: admin → all, agent → only if assignee, customer → only if requester
- If not allowed it returns **404, not 403**, so users can't discover which ticket numbers exist.

The page layout: header plus **ActionBar** (status select, Chat, Comment, Reopen), details, description, attachments, Comments/History tabs, and a sidebar (assignee and reassign, reporter, notify emails, dates, linked tickets). `useMarkTicketRead` marks this ticket's notifications as read.

After any action, `refreshAll()` invalidates `ticket`, `timeline`, `tickets`, `summary` and `notifications`, so everything updates immediately.

### 4.9 Comments and internal notes

1. **UI** `Composer` → `POST /tickets/{id}/messages {body, isInternal}`. The "Internal note" checkbox appears for staff only.
2. **S** `AddMessage`:
   - `loadAccessible`, body 1–5000
   - `isInternal && !staff` → 403
   - ticket closed → 409 "create a new ticket and link this one"
   - transaction: `CreateMessage`; then **if a customer replies to an `on_hold` ticket, `setStatus(in_progress)`** (and logs an event), otherwise `TouchTicket`
3. Customers never receive internal notes: `ListMessages(..., includeInternal=false)` adds `WHERE … AND ($2 OR NOT is_internal)`. The same rule applies in the notification feed.

DB: `ticket_messages`, `tickets`, `ticket_events`.

### 4.10 Status lifecycle (state machine)

```
            ┌────────────┐
            ▼            │
  open ⇄ in_progress ⇄ on_hold
    │         │           │
    └────► solved ◄───────┘
             │  ▲
   (admin)   │  │ reopen (customer ≤ 7 days, staff any time)
             ▼  │
           closed  (final; auto after 7 days in solved)
```

1. **UI**: `nextStatuses(role, current)` in [utils/index.ts](../web/src/utils/index.ts) decides which options to show (mirrors the server).
2. `PATCH /tickets/{id}/status {status}` → **S** `ChangeStatus`:
   - status must be a valid enum value
   - `loadAccessible`
   - **`CheckTransition(role, from, to, solvedAt, now)`** ([status_machine.go](../api/internal/service/status_machine.go)):
     - same status → 409; transition not in the map → 409 `INVALID_TRANSITION`
     - customer: **only solved → open**, and only within the **7-day reopen window**
     - agent: anything valid **except closing**
     - admin: any valid transition
   - transaction `setStatus`: sets `solved_at = now` when solved and clears it when reopened → `UpdateTicketStatus` + `CreateEvent(status_changed, {from, to})`
3. **Auto-close** ([main.go](../api/cmd/server/main.go) `runAutoClose`): at startup and then every hour → `AutoCloseSolved(cutoff = now − 7d)` runs **one SQL statement** (a CTE `UPDATE … RETURNING` followed by an `INSERT` into `ticket_events` with `{"auto":true}`), so the close and its audit event happen together.

### 4.11 Reassign / hand-off

1. **UI** `AssigneeSelect` uses `useAgents()` → `GET /agents` (staff only) → `ListStaff` returns active agents and admins with an **open-ticket count** (correlated subquery).
2. `PATCH /tickets/{id}/assignee {assigneeId}` (staff only) → **S** `Assign`:
   - closed → 409; the target must be an **active staff** user
   - no-op if the assignee hasn't changed
   - transaction: `UpdateTicketAssignee` + `CreateEvent(assigned)`
   - returns the result **without** the access check, because an agent who handed a ticket off can no longer view it. The UI then sends that agent back to `/tickets`.

### 4.12 Notifications bell

1. **UI** [NotificationBell.tsx](../web/src/components/NotificationBell.tsx) → `useNotifications()` **polls `GET /notifications` every 10s** (regardless of the Live setting). It also updates the **tab title** to `(3) Support Desk`.
2. **S** `Notifications` → **R** `ListNotifications` uses a single CTE (`notificationCTE` in [postgres.go](../api/internal/repo/postgres.go)):
   - `scope`: tickets where I am the requester **or** the assignee
   - `items`: a `UNION ALL` of **events, messages, attachments and chat messages** on those tickets, **excluding my own actions** (and the automatic assignment at creation). Internal notes are included for staff only.
   - `marked`: an item is **unread** if `created_at > GREATEST(users.notifications_read_at, ticket_reads.read_at)`
   - Query 1 returns unread counts per ticket (for the badge and the blue dots); query 2 returns the latest 30 items.
3. Read tracking uses **watermarks**:
   - **Mark all as read**: `POST /notifications/read {}` → `UPDATE users SET notifications_read_at = now()`
   - **Open a ticket**: `useMarkTicketRead` → `POST /notifications/read {ticketId}` → `UPSERT ticket_reads`
4. Clicking a chat notification opens the ticket **with the chat window already open** (`state.openChat`).

DB: `ticket_events`, `ticket_messages`, `ticket_attachments`, `ticket_chat_messages`, `ticket_reads`, `users`.

### 4.13 Live chat (real-time, SSE)

1. **UI** 💬 Chat → [ChatWindow.tsx](../web/src/components/ChatWindow.tsx) → [useChat.ts](../web/src/hooks/useChat.ts):
   - history: `GET /tickets/{id}/chat` → **S** `ChatThread` returns the last 200 messages plus `canSend` and `reason`
   - live: `openChatStream()` uses **`fetch` streaming**, not `EventSource`, because `EventSource` can't send an `Authorization` header. It handles 401 by refreshing and retrying once.
   - [sse.ts](../web/src/utils/sse.ts) `parseSSE` splits the stream into `event:`/`data:` frames
   - on `ready`: status shows **Live**. After a reconnect it **refetches history** so no messages are missed.
   - on `message`: appended to the TanStack cache (de-duplicated by id)
   - on error or close: **exponential back-off** 1s, 2s, 4s … up to 10s, and the status shows "Reconnecting…"
2. Server stream **H** `ChatStream`:
   - **S** `SubscribeChat` → `loadAccessible` → `chatHub.subscribe(ticketID, actor)` returns a buffered channel (16)
   - headers `text/event-stream`, `no-cache`, `X-Accel-Buffering: no`; sends `event: ready`
   - loop: on a new message → `event: message\ndata: {json}`; every **25s** a `: ping` keeps proxies from closing the connection; client disconnect → unsubscribe
   - the router **exempts `/chat/stream` from the 30s timeout**
3. Sending: `POST /tickets/{id}/chat {body}` → **S** `SendChat`:
   - `canSendChat`: only the **reporter or the current assignee** may send (admins otherwise read-only), and not on closed tickets
   - body 1–2000 → `CreateChatMessage` (DB) → **`chatHub.publish`** sends it to every open stream for that ticket, **re-checking `canView`** per subscriber (an agent who handed the ticket off stops receiving). A slow subscriber is dropped and will reconnect.
4. With TLS enabled (`TLS_CERT_FILE`), Go serves **HTTP/2**, so many SSE streams share one connection.

DB: `ticket_chat_messages`. Live fan-out happens **in memory** (`chatHub`).

### 4.14 Live auto-refresh (the "Live" pill)

1. [useLiveRefresh.tsx](../web/src/hooks/useLiveRefresh.tsx) `LiveRefreshProvider` holds `intervalMs` (Off, 5s … 1h, default 10s) and saves it in `localStorage`. It exposes `interval = live ? intervalMs : false`.
2. [RefreshControl](../web/src/components/common.tsx) (Dashboard, List, Detail) shows the **Live/Paused** pill, "Updated HH:MM:SS", the **Refresh** button and the interval dropdown.
3. [queries.ts](../web/src/hooks/queries.ts): `useTickets`, `useTicket`, `useTimeline` and `useSummary` all pass `refetchInterval: interval` to TanStack Query, which **polls** the normal REST endpoints. If the data hasn't changed, nothing re-renders.
4. It also refetches on window focus (`refetchOnWindowFocus`, with `staleTime` 5s) and immediately after your own actions (`invalidateQueries`).
5. The backend has **no special code** for this; polled requests are ordinary GETs.

### 4.15 Dashboard and reports

1. **UI** [DashboardPage.tsx](../web/src/pages/DashboardPage.tsx): the range is stored in the URL (default 30d) using [TimeRangePicker](../web/src/components/TimeRangePicker.tsx). `useSummary(range)` → `GET /reports/summary?from=…&to=…&tz=Asia/Kolkata`.
2. **S** `Summary` ([reports.go](../api/internal/service/reports.go)):
   - `ParseReportRange` validates RFC 3339 dates and the IANA time zone
   - `applyScope` (role scope, as in the list)
   - `CountTickets`: `GROUP BY status, severity, type` → totals and breakdowns
   - `ResolutionStats`: from **`ticket_events`** in the range, counts tickets *resolved* (`to = solved`), *closed*, and *reopened* (`solved → open`), plus the **average hours from creation to solved**
   - `trendBucket`: ≤ 48h → hour, ≤ 92 days → day, otherwise month
   - `TicketTrend`: one SQL with two CTEs (created per bucket, resolved per bucket) using `date_trunc(bucket, ts AT TIME ZONE tz)` and a `FULL JOIN`. `fillTrend` then **fills empty buckets** so the chart has no gaps (max 400).
   - `Recent`: the 5 most recently updated tickets
3. **CSV export** (agents and admins): `GET /reports/export?...` → **S** `ExportTickets` (scoped, max 10,000 rows) → [csv.go](../api/internal/httpapi/csv.go) `writeTicketsCSV`, where **`csvSafe` prefixes `= + - @` with `'`** to block Excel formula injection → the browser saves the file with `saveBlob`.
4. [TrendChart.tsx](../web/src/components/TrendChart.tsx) is a **hand-built SVG chart** (no chart library).

DB: `tickets`, `ticket_events` (indexes `ticket_events_status_idx`, `tickets_created_idx` added for these queries).

### 4.16 Agent management (admin)

[AgentsPage.tsx](../web/src/pages/AgentsPage.tsx)
- **List**: `GET /agents` → staff with open-ticket counts, plus search and role filtering in the browser.
- **Create**: `POST /agents` (admin) → **S** `AuthService.CreateAgent` → the same validation as register with `role = agent`.
- **Remove**: `DELETE /agents/{id}` (admin) → **S** `RemoveAgent` → transaction **R** `DeactivateAgent`:
  1. `UPDATE users SET deactivated_at = now()` (**soft delete**: their comments and events keep their name)
  2. revoke all their refresh tokens, so they're signed out within 15 minutes at most
  3. a CTE moves all their **non-closed tickets to the default admin** and writes one `assigned` event per ticket (`reason: agent removed`)
  4. returns how many tickets were reassigned → UI banner

### 4.17 Linked tickets

- Add: `POST /tickets/{id}/links {linkedTicketId}`. **Owner only**. The linked ticket must be the same requester's, **solved or closed**, not itself, and not already linked in **either** direction. Writes a link row, a `linked` event and touches the ticket.
- Remove: `DELETE /tickets/{id}/links/{linkedId}`. Owner only; deletes in either direction.
- `ListLinks` returns the *other* ticket for each link (`CASE WHEN … THEN linked_ticket_id ELSE ticket_id`), so links are **undirected**.

### 4.18 Attachment download

`ticketsApi.downloadAttachment` → `GET /tickets/{id}/attachments/{attId}` (with the auth header, as a blob) → **S** `GetAttachment` (`loadAccessible` + attachment must belong to that ticket) → headers `Content-Disposition: attachment` and `X-Content-Type-Options: nosniff` (**never displayed inline**, which prevents stored XSS through uploaded HTML or SVG) → `saveBlob`.

### 4.19 Errors, health and startup

- Every service error is an `apperr.Error{Status, Code, Message}` → `httpx.WriteError` → `{"error":{"code":"VALIDATION_ERROR","message":"…"}}`. Unknown errors are **logged** and returned as a generic 500, so internal details never leak.
- Codes: `BAD_REQUEST 400`, `UNAUTHORIZED 401`, `FORBIDDEN 403`, `NOT_FOUND 404`, `CONFLICT / INVALID_TRANSITION 409`, `VALIDATION_ERROR 422`, `INTERNAL 500`.
- UI: `getErrorMessage()` shows the server's message, or a friendly network/5xx message. `ErrorMessage` offers a **Retry** button.
- `recoverJSON` turns panics into a JSON 500 instead of dropping the connection.
- Health: `GET /healthz` pings the DB (2s timeout) → `{"status":"ok","db":"up"}` or **503** `degraded`.
- Startup: `config.Load` validates required env vars (JWT secret ≥ 32 chars, admin password ≥ 8) → DB connect **with 10 retries** (so the API can start while Postgres boots) → **migrations** → **seed admin** (password set only on first creation; the name follows `.env`).

---

## 5. Database summary

| Table | Purpose | Notable design |
|---|---|---|
| `users` | All roles | `CITEXT` unique email; `role` enum; partial unique index → only **one default assignee**; `deactivated_at` soft delete; `notifications_read_at` watermark |
| `refresh_tokens` | Sessions | Only the **SHA-256 hash** is stored; `revoked`, `expires_at`; cascade on user delete |
| `tickets` | Core entity | `ticket_number` from a sequence; enums for type, severity, status; `notify_emails TEXT[]`; `solved_at` for the reopen window and resolution time |
| `ticket_messages` | Comments and notes | `is_internal` flag |
| `ticket_events` | **Audit log** / history | `JSONB data` (`{from,to}`, `{assigneeName}`, `{auto:true}`); `actor_id` NULL = system |
| `ticket_links` | Related tickets | Composite PK; `CHECK (ticket_id <> linked_ticket_id)`; treated as undirected |
| `ticket_attachments` | Files | `BYTEA` (≤ 5 MB); `CHECK size_bytes > 0` |
| `ticket_chat_messages` | 1:1 chat | Indexed by `(ticket_id, created_at)` |
| `ticket_reads` | Per-ticket read time | PK `(user_id, ticket_id)` upserted |

Migrations: [db/migrations](../db/migrations) (001–006), applied automatically by `golang-migrate` at startup.

---

## 6. Security checklist (good to call out)

| Threat | Mitigation | Where |
|---|---|---|
| Password theft | bcrypt cost 12; never returned in JSON (`json:"-"`) | [auth.go](../api/internal/auth/auth.go), [domain.go](../api/internal/domain/domain.go) |
| Token theft via XSS | Access token **in memory only**; refresh token **HttpOnly** cookie, scoped to `/api/v1/auth` | [client.ts](../web/src/api/client.ts), [auth_handler.go](../api/internal/httpapi/auth_handler.go) |
| Stolen refresh token reuse | **Rotation** on each refresh; DB stores a hash only; revoked on logout or agent removal | [auth_service.go](../api/internal/service/auth_service.go) |
| CSRF | `SameSite=Lax` cookie; state-changing APIs need the Bearer header, which another site can't set | |
| Brute force | `httprate` 30 req/min/IP on `/auth/*`; generic login error | [router.go](../api/internal/httpapi/router.go) |
| Broken access control | `RequireRole` + `loadAccessible`/`canView` on **every** ticket operation; 404 instead of 403; server-side scope | [ticket_service.go](../api/internal/service/ticket_service.go) |
| SQL injection | Parameterised queries everywhere; `ORDER BY` whitelist; LIKE escaping | [postgres.go](../api/internal/repo/postgres.go) |
| Malicious uploads | Size/count limits, sniffed MIME type, sanitised filename, `attachment` + `nosniff` | [attachments.go](../api/internal/service/attachments.go) |
| CSV formula injection | `csvSafe` prefix | [csv.go](../api/internal/httpapi/csv.go) |
| Oversized / unexpected input | 1 MB JSON limit; unknown fields rejected; length limits | [httpx.go](../api/internal/httpx/httpx.go), [validate.go](../api/internal/service/validate.go) |
| Info leakage | Generic 500s, panics recovered, typed error codes | [httpx.go](../api/internal/httpx/httpx.go) |
| JWT algorithm confusion | `WithValidMethods(HS256)` | [auth.go](../api/internal/auth/auth.go) |

---

## 7. Why this tech stack?

| Layer | Choice | Why we picked it | Alternatives considered |
|---|---|---|---|
| **Frontend framework** | **React 18 + TypeScript** | Largest ecosystem and hiring pool; component model fits a screen-heavy app; TypeScript catches API contract mistakes at compile time (`types.ts` mirrors the Go structs) | Angular (heavier), Vue (smaller ecosystem) |
| **Build tool** | **Vite** | Near-instant dev server and HMR; built-in **proxy** gives a single origin (no CORS, cookies just work); tiny config | Create React App (deprecated), Webpack (slow, verbose) |
| **Server state** | **TanStack Query** | Caching, de-duplication, **polling (`refetchInterval`)**, refetch on focus, `invalidateQueries` after mutations. Live refresh needed almost no custom code | Redux (lots of boilerplate for server data), SWR |
| **HTTP client** | **Axios** | Interceptors make the **token attach + silent refresh + replay** easy and central | fetch (we use it only for SSE streaming) |
| **Routing** | **React Router 6** | Nested layouts, route guards, URL search params for shareable filters | |
| **UI components** | Hand-built (Select, MultiSelect, TrendChart…) | Small bundle (5 runtime deps), full control and accessibility, no heavy UI or chart library | MUI, Chart.js |
| **Backend language** | **Go 1.23** | Fast and memory-efficient; single static binary; goroutines and channels make **SSE fan-out and background jobs** simple; strong standard library (`net/http`, HTTP/2, `crypto`) | Node/Express (single-threaded, weaker typing), Java/Spring (heavier) |
| **Router** | **chi** | Lightweight, uses the standard `net/http` interfaces, composable middleware, route groups for auth and RBAC | Gin, Echo (own context types) |
| **DB driver** | **pgx v5** (hand-written SQL) | Fastest Postgres driver; native support for arrays, JSONB, UUIDs; full control over the SQL (CTEs, `FILTER`, `date_trunc`). No ORM surprises | GORM (hides SQL, slower), sqlc (planned in the README; not used yet) |
| **Database** | **PostgreSQL 16** | Relational data with **FKs, enums, transactions, CHECK constraints**; JSONB for flexible event data; `CITEXT`; partial indexes; strong reporting SQL | MySQL (weaker enum/JSON/CTE features), MongoDB (no relational integrity) |
| **Migrations** | **golang-migrate** | Versioned up/down SQL files, run automatically at startup | |
| **Auth** | **JWT (15 min) + rotating refresh cookie (7 days) + bcrypt** | Stateless request auth (no session lookup per request) while still allowing logout and revocation through the refresh table | Server sessions only (needs a lookup on every request), OAuth provider (out of scope) |
| **Real-time** | **Polling** for lists/dashboards, **SSE** for chat | Polling is simple and fine for data that changes every few seconds. SSE is one-way server push over plain HTTP that works through proxies and runs over HTTP/2; chat sends go through a normal POST | WebSockets (two-way, more infrastructure, harder through proxies) |
| **Local infra** | **Docker Compose** (Postgres) | One command for a consistent DB on any laptop | |
| **Testing** | Go `testing` + in-memory repo; **Jest + React Testing Library** | Fast unit tests for business rules without a DB; UI tests from the user's point of view | |

---

## 8. Likely questions and answers

**Architecture**
1. **Why separate handler / service / repository?** Each layer has one job. HTTP details stay in handlers, business rules in services, SQL in the repository. Services depend on the `Repository` *interface*, so tests use the in-memory `memrepo` and the database could be swapped.
2. **Where are business rules enforced?** Only in the service layer (`TicketService`, `AuthService`, `CheckTransition`). The UI mirrors some rules only to decide which buttons to show.
3. **How do transactions work?** `repo.InTx(ctx, fn)` uses `pgx.BeginFunc`. Inside it, a transaction-bound repository is passed to `fn`; any error rolls back. Used for create ticket, status change, reassign, token rotation and agent removal.
4. **Why does the browser call `/api` on port 5180 and not 8080?** The Vite proxy gives a single origin, so there are no CORS preflights and the SameSite refresh cookie works.
5. **How would you deploy this to production?** Build the SPA (`npm run build`) and serve the static files plus a reverse proxy for `/api` (nginx or a CDN), run the Go binary in a container, use managed Postgres, set `COOKIE_SECURE=true` and TLS (enables HTTP/2), and put secrets in a vault.

**Auth and security**
6. **Why not store the JWT in localStorage?** Any XSS could read it. Keeping it in memory, with the refresh token in an HttpOnly cookie, means scripts can't steal a long-lived credential.
7. **What happens when the access token expires?** The first 401 triggers one shared `/auth/refresh`, the token is rotated, and the original request is replayed. The user notices nothing.
8. **How is logout enforced if JWTs are stateless?** Logout revokes the refresh token in the DB. The access token remains valid for at most 15 minutes. Removing an agent revokes all their refresh tokens too.
9. **Why return 404 instead of 403 for other people's tickets?** So users can't tell which ticket numbers exist.
10. **How do you prevent SQL injection?** Every value is a `$n` parameter. The only dynamic SQL is `ORDER BY`, which comes from a fixed whitelist map.
11. **Is login brute-force protected?** Yes: 30 requests per minute per IP on `/auth/*`, bcrypt cost 12 (slow by design), and the same error for "no such user" and "wrong password".

**Tickets and workflow**
12. **How is the ticket number generated?** By a Postgres sequence in the column default: `'TKT-' || lpad(nextval('ticket_number_seq'), 6, '0')`. It's atomic and safe under concurrency.
13. **How does auto-assignment work?** In the create transaction, `GetDefaultAssignee()` returns the admin flagged `is_default_assignee`. A partial unique index guarantees only one exists. The admin then reassigns to agents.
14. **Can a closed ticket be reopened?** No, closed is final. The customer creates a new ticket and **links** the old one. Solved tickets can be reopened by the customer within 7 days, or by staff at any time.
15. **How does auto-close work?** A goroutine runs at startup and hourly. One SQL statement closes tickets solved more than 7 days ago and logs `status_changed {auto:true}` events.
16. **What is "On hold"?** The ticket is waiting on the customer. When the customer replies, it automatically moves back to In progress.
17. **How is the history/audit trail built?** Every assignment, status change, link and creation writes a row to `ticket_events` with JSONB details. The timeline merges these with messages, sorted by time.

**Live features**
18. **Is "Live" refresh real-time?** It's polling: TanStack Query refetches every N seconds (default 10). Your own actions update immediately through `invalidateQueries`. Chat is true push via SSE.
19. **Why SSE and not WebSockets for chat?** We only need server→client push; sending is a normal POST. SSE is plain HTTP, works through proxies, auto-multiplexes on HTTP/2, and is simpler to secure with the existing auth middleware.
20. **Why use `fetch` instead of `EventSource`?** `EventSource` can't send an `Authorization` header, and we don't put tokens in URLs.
21. **What if the chat connection drops?** It reconnects with exponential back-off (max 10s) and reloads history on reconnect, so nothing is missed. Messages are de-duplicated by id.
22. **Does chat scale to multiple API servers?** Not yet. `chatHub` is in-memory, so a message reaches only streams on the same instance. To scale, publish through **Postgres LISTEN/NOTIFY or Redis pub/sub** (noted in the code).
23. **How are unread notifications computed?** Two watermarks per user: the global "mark all read" time and a per-ticket "last opened" time. An item is unread if it's newer than both. It's computed in SQL; no notification table needs maintaining.

**Data and reporting**
24. **How is "Resolved" counted?** From `ticket_events` where `to = 'solved'` *within the selected range* (distinct tickets), so it reflects when work was done, not when the ticket was created.
25. **Why do trend charts respect my time zone?** The browser sends `tz` (e.g. `Asia/Kolkata`), and SQL buckets with `date_trunc(..., created_at AT TIME ZONE tz)`.
26. **Why store attachments in Postgres?** Simplicity and consistency: one backup, transactional, and access is checked through the same service. Limited to 5 MB × 10 per ticket. At scale we'd move to S3 or blob storage with signed URLs.
27. **Is CSV export safe to open in Excel?** Yes. Cells starting with `= + - @` are prefixed with `'` to prevent formula injection.

**Quality and limitations (be upfront)**
28. **What's tested?** Go: the state machine, ticket/auth services (using the in-memory repo), JWT, router (auth/RBAC), config, CSV, the query builder. Web: components (Select, MultiSelect, ChatWindow, TrendChart, nav), the ticket form, utils, time ranges, notifications.
29. **Are notification emails actually sent?** Not yet. The addresses are validated and stored on the ticket (`notify_emails`). Sending (SMTP/SES via a background worker) is the next step.
30. **Known next steps:** email delivery, password reset, shared pub/sub for multi-instance chat, object storage for files, SLA timers, full-text search, audit export.

---

## 9. Run it

```bash
docker compose up -d db                       # Postgres on :5435
cd api && go run ./cmd/server                 # API on :8080 (migrates + seeds admin)
cd web && npm install && npm run dev          # UI on :5180 (also on your LAN IP)

cd api && go test ./...                       # backend tests
cd web && npm test                            # frontend tests
```

Default admin: values of `ADMIN_EMAIL` / `ADMIN_PASSWORD` in `.env`.
