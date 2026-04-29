# Email Templates - Step 10

Design and implementation document for the HTML email template system introduced in Step 10 of the notifications feature.

## Status

| Area | Status |
|---|---|
| MJML templates (`booking.mjml`, `issue.mjml`) | ✅ done |
| Compile script + `pnpm build` integration | ✅ done |
| Generated HTML (`templates/*.html`) | ✅ done |
| `template.go` - embed, renderers, color maps, plain text | ✅ done |
| `send.go` - all send functions use templates, `baseURL` param | ✅ done |
| `notifier.go` - `TextBody` field | ✅ done |
| `smtp.go` - plain-text alternative part | ✅ done |
| Handler structs - `BaseURL` field, all call sites updated | ✅ done |
| `main.go` - `APP_BASE_URL` env var, wired to handlers | ✅ done |
| `gen-env.sh` - `APP_BASE_URL` for dev and demo/prod | ✅ done |
| i18n keys (`email_banner_*`, `email_intro_*`, `email_cta_*`, etc.) | ⏳ pending |
| `docker-compose.yml` - `APP_BASE_URL` env block | ⏳ pending |
| `MeHandler` test-email - pass `BaseURL` | ⏳ pending |
| `TestNotifications_EventTriggered` - body assertions | ⏳ pending |
| Mailpit visual review | ⏳ pending |

## Approach

Templates are written in **MJML** (mjml.io) - a markup language that compiles to cross-client-safe HTML. MJML handles all email quirks (table-based layout, Outlook conditional comments, inline styles) so the source files remain readable.

The compiled HTML is embedded in the Go binary. Go substitutes dynamic values using simple string replacement before sending.

Plain text is constructed directly in Go (not a separate template) - it mirrors the email content without HTML.

## Why MJML

- Purpose-built for email cross-client compatibility, including Outlook desktop
- Readable XML-like syntax; no new language to learn
- Compiles to static HTML that Go can embed
- Active maintenance and wide industry adoption
- Integrates cleanly as a build script step alongside `pnpm build`

## File structure

```
web/
  src/
    lib/
      emails/
        booking.mjml     # booking events (all variants via placeholders)
        issue.mjml       # issue events (all variants via placeholders)
  scripts/
    compile-emails.ts    # compiles .mjml -> ../../api/internal/notifications/templates/

api/
  internal/
    notifications/
      templates/
        booking.html     # generated, committed
        issue.html       # generated, committed
      template.go        # embed + rendering (string replacement)
```

Generated HTML files are **committed** to the repo so `go build` never requires the Node toolchain.

## Template design

### Shared layout (both templates)

```
+----------------------------------+
|  [brand header: blue-700 bg]     |  group name in white
|                                  |
|  [event banner]                  |  colored strip: what happened / what is needed
|    EMAIL_BANNER_LABEL            |  e.g. "Godkännande krävs", "Bekräftad", "Försenad"
|                                  |
|  [content card: white]           |
|    Hej EMAIL_RECIPIENT_NAME,     |
|                                  |
|    EMAIL_INTRO                   |  one-line context sentence
|                                  |
|    [detail section]              |  template-specific (see below)
|                                  |
|    [CTA button]                  |  EMAIL_CTA_LABEL -> EMAIL_CTA_URL
|                                  |
|  [footer: neutral-50 bg]         |
|    EMAIL_GROUP_NAME              |
|    Ändra notiser -> /profile     |
+----------------------------------+
```

Background: `#ebf1f7` (neutral-50). Header: `#003660` (blue-700).

#### Event banner

The banner sits between the header and the content card, full-width, with a colored background that signals the nature of the event. This is the first thing the recipient reads and immediately communicates what changed or what is needed - before they read any detail.

| Event | Banner label (sv) | Banner color |
|---|---|---|
| `booking_needs_approval` | Godkännande krävs | orange-100 bg / orange-800 text |
| `booking_submitted_no_approval` | Ny bokning bekräftad | green-100 bg / green-800 text |
| `booking_confirmed` | Bokning bekräftad | green-100 bg / green-800 text |
| `booking_rejected` | Bokning nekad | red-100 bg / red-800 text |
| `booking_cancelled` | Bokning avbokad | neutral-100 bg / neutral-700 text |
| `booking_reminder` | Påminnelse: startar imorgon | blue-50 bg / blue-700 text |
| `booking_overdue` | Bokning försenad - åtgärd krävs | red-100 bg / red-800 text |
| `issue_created` | Nytt ärende rapporterat | orange-100 bg / orange-800 text |
| `issue_assigned_to_me` | Du har tilldelats ett ärende | blue-50 bg / blue-700 text |
| `issue_resolved` | Ärende löst | green-100 bg / green-800 text |
| `issue_commented` | Ny kommentar | blue-50 bg / blue-700 text |

The banner is driven by `EMAIL_BANNER_LABEL`, `EMAIL_BANNER_BG`, and `EMAIL_BANNER_FG` placeholders - one set of values per event type, set in Go.

#### CTA button color

The button color also follows the event tone rather than always using blue-700:

- Action required (needs_approval, overdue): orange-600 (`#d97706`)
- Positive outcome (confirmed, resolved): green-700 (`#008236`)
- Neutral/info (reminder, comment, assignment): blue-700 (`#003660`)
- Negative (rejected, cancelled): neutral-700 (`#374b5a`)

Driven by `EMAIL_CTA_BG` placeholder.

### booking.mjml detail section

Mirrors the `BookingCard` visual identity: date range prominent, team badge, status badge, notes. All items in the booking are listed — this is the primary content the recipient needs to act on.

```
+--------------------------------+
| 15 jan - 20 jan 2025           |  dates, large
| Tältpatrullen  [Bekräftad]     |  team badge (blue-50/blue-700) + status badge
|                                |
| Utrustning                     |  section heading
| - Sibley 500 - Tält 1          |  individually tracked: commercial + common name
| - Sibley 500 - Tält 2          |
| - Stormkök (3 st)              |  quantity-tracked: commercial name + count
| - Kniv (2 st)                  |
|                                |
| Anteckningar: ...              |  if non-empty
+--------------------------------+
```

Items are fetched via `ListBookingItems` (already exists), which returns `commercial_name`, `common_name`, `location_name`, `individually_tracked`. Grouping logic mirrors `IssueCard`'s `articleNames` derived value: individually-tracked items show `commercial_name - common_name`; quantity-tracked items group by `commercial_name` and show a count. Items are ordered by category then commercial name (the query already does this).

Status badge colors (inline hex - matches `styles.ts`):

| Status | bg | text |
|---|---|---|
| submitted | #fef3c7 | #92400e |
| confirmed | #dcfce7 | #008236 |
| picked_up | #d0e0ff | #003660 |
| rejected | #ffe2e2 | #c10007 |
| cancelled | #e5e5e6 | #585a5c |

### issue.mjml detail section

Mirrors the `IssueCard`: title prominent, severity badge, description (truncated to ~300 chars), reporter.

```
+--------------------------------+
| [Allvarlig]  [Öppen]           |  severity + status badges
| Trasigt tältdrag...            |  description excerpt
| Rapporterat av Anna Svensson   |  reporter name
+--------------------------------+
```

Severity badge colors:

| Severity | bg | text |
|---|---|---|
| usable | #fef3c7 | #92400e |
| unusable | #ffe2e2 | #c10007 |
| missing | #ffe2e2 | #c10007 |

## Placeholder system

Placeholders are uppercase strings that cannot appear in normal content. Go uses `strings.NewReplacer` to substitute them at send time.

### Shared (both templates)

| Placeholder | Go source |
|---|---|
| `EMAIL_RECIPIENT_NAME` | `recipient.name` (HTML-escaped) |
| `EMAIL_BANNER_LABEL` | `i18n.T(lang, "email_banner_<event>")` |
| `EMAIL_BANNER_BG` | hex from banner color map |
| `EMAIL_BANNER_FG` | hex text color from banner color map |
| `EMAIL_INTRO` | `i18n.T(lang, "email_intro_<event>", vars)` |
| `EMAIL_CTA_URL` | `baseURL + "/bookings/" + id` or `"/issues/" + id` |
| `EMAIL_CTA_LABEL` | `i18n.T(lang, "email_cta_<event>")` |
| `EMAIL_CTA_BG` | hex from CTA color map (event tone) |
| `EMAIL_GROUP_NAME` | `group.Name` (HTML-escaped) |
| `EMAIL_UNSUBSCRIBE_URL` | `baseURL + "/profile"` |

### Booking-specific

| Placeholder | Go source |
|---|---|
| `EMAIL_START_DATE` | `booking.StartDate` formatted as "15 jan 2025" |
| `EMAIL_END_DATE` | `booking.EndDate` formatted as "15 jan 2025" |
| `EMAIL_TEAM_LABEL` | team name, or `i18n.T(lang, "email_personal_booking")` |
| `EMAIL_TEAM_BG` | `#e6eeff` (team) or `#e5e5e6` (personal) |
| `EMAIL_TEAM_FG` | `#003660` (team) or `#585a5c` (personal) |
| `EMAIL_STATUS` | `i18n.T(lang, "booking_status_<status>")` |
| `EMAIL_STATUS_BG` | hex from `bookingStatusBadge` map |
| `EMAIL_STATUS_FG` | hex text from `bookingStatusBadge` map |
| `EMAIL_ITEMS_HEADING` | `i18n.T(lang, "email_items_heading")` |
| `EMAIL_ITEMS_HTML` | `<br>`-separated item lines built by `buildItemsHTML` |
| `EMAIL_NOTES_HEADING` | `i18n.T(lang, "email_notes_heading")` |
| `EMAIL_NOTES` | HTML-escaped `booking.Notes` (empty string if no notes) |

`EMAIL_ITEMS_HTML` is built by `buildItemsHTML` from `ListBookingItems` results. Individually-tracked items: `commercial_name &ndash; common_name`. Quantity-tracked: grouped by `commercial_name` with count `(N st)`.

### Issue-specific

| Placeholder | Go source |
|---|---|
| `EMAIL_ISSUE_TITLE` | HTML-escaped `issue.Title` |
| `EMAIL_SEVERITY` | `i18n.T(lang, "issue_severity_<severity>")` |
| `EMAIL_SEVERITY_BG` | hex from `issueSeverityBadge` map |
| `EMAIL_SEVERITY_FG` | hex text from `issueSeverityBadge` map |
| `EMAIL_ISSUE_STATUS` | `i18n.T(lang, "issue_status_<status>")` |
| `EMAIL_ISSUE_STATUS_BG` | hex from `issueStatusBadge` map |
| `EMAIL_ISSUE_STATUS_FG` | hex text from `issueStatusBadge` map |
| `EMAIL_DESCRIPTION` | HTML-escaped description, truncated to 300 runes + "..." |
| `EMAIL_REPORTER_LINE` | `i18n.T(lang, "email_reporter_line", {"name": reporterName})` |

## Build pipeline

`web/scripts/compile-emails.ts` uses the `mjml` npm package (Node API) to compile each `.mjml` file to HTML and write the result to `api/internal/notifications/templates/`.

`package.json` scripts:

```json
"compile-emails": "tsx scripts/compile-emails.ts",
"build": "pnpm compile-emails && vite build"
```

Run manually after editing a template:
```bash
cd web && pnpm compile-emails
```

The generated `.html` files are committed. No Go build dependency on Node.

## Go-side rendering

`api/internal/notifications/template.go` (implemented):

```go
//go:embed templates/booking.html
var bookingTemplate string

//go:embed templates/issue.html
var issueTemplate string

func renderBookingEmail(d BookingEmailData) (htmlOut, textOut string)
func renderIssueEmail(d IssueEmailData) (htmlOut, textOut string)
func fetchBookingEmailData(ctx, q, b, event, lang, recipientName, baseURL) BookingEmailData
func fetchIssueEmailData(ctx, q, issue, event, lang, recipientName, baseURL) IssueEmailData
```

Each renderer builds a `strings.NewReplacer` from all placeholder->value pairs and applies it to the embedded template string. Plain text is built by `buildBookingText` / `buildIssueText` as simple formatted strings.

`fetchBookingEmailData` calls `GetGroup`, `GetTeam`, and `ListBookingItems`. `fetchIssueEmailData` calls `GetGroup` and `GetUser` (for reporter name). These fetches happen inside the fire-and-forget goroutine in the handler, so they don't block the HTTP response.

Color maps in `template.go`: `eventStyles` (banner + CTA colors per event), `bookingStatusBadge`, `issueSeverityBadge`, `issueStatusBadge`.

## Plain text

Plain text is built in Go as a simple formatted string per event, not a template. Example for `booking_confirmed`:

```
Hej {name},

{heading}

{intro}

Datum: {start} - {end}
Enhet: {team}
Status: {status}
Anteckningar: {notes}

Se bokning: {url}

---
{group}
Ändra notiser: {unsubscribeUrl}
```

This is readable and correct across all email clients. It mirrors the HTML content.

## Message struct changes

`Message` in `notifier.go` gains a `TextBody string` field. `smtp.go` calls `m.AddAlternativeString(mail.TypeTextPlain, msg.TextBody)` alongside the existing HTML body.

## New env var

`APP_BASE_URL` - the public URL of the frontend (e.g. `https://utrustning.malarscouterna.se`).

- Default in dev: `http://localhost:5173`
- Set in `gen-env.sh` for demo/prod with a `CHANGEME` placeholder
- Read in `main.go`, passed to `BookingHandler.BaseURL` and `IssueHandler.BaseURL`

## i18n keys to add

Per-event banner label, intro, and CTA keys in both `sv.json` and `en.json`:

```
email_banner_booking_needs_approval    # "Godkännande krävs"
email_banner_booking_confirmed         # "Bokning bekräftad"
... (one banner label per event)

email_intro_booking_needs_approval     # one-line sentence shown below the greeting
email_intro_booking_confirmed
... (one intro per event)

email_cta_booking_needs_approval       # CTA button label
email_cta_booking_confirmed
... (one CTA per event)

email_items_heading                    # "Utrustning"
email_notes_heading                    # "Anteckningar"
email_footer_unsubscribe               # "Ändra notiser"
email_personal_booking                 # label when no team (replacing "Personlig" from the card)
```

## Testing

- Extend `TestNotifications_EventTriggered` to assert captured `Body` contains the booking URL and dates
- Extend to assert `TextBody` is non-empty
- Visual review via Mailpit at `http://localhost:8025` (already in dev Compose stack, SMTP at `localhost:1025`)
