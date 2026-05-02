package notifications

import (
	"context"
	_ "embed"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/i18n"
)

//go:embed templates/booking.html
var bookingTemplate string

//go:embed templates/issue.html
var issueTemplate string

// eventStyle holds banner and CTA colors for a notification event.
type eventStyle struct {
	bannerBG string
	bannerFG string
	ctaBG    string
}

var eventStyles = map[string]eventStyle{
	EventBookingNeedsApproval:      {bannerBG: "#fef3c7", bannerFG: "#92400e", ctaBG: "#d97706"},
	EventBookingSubmittedNoApproval: {bannerBG: "#dcfce7", bannerFG: "#008236", ctaBG: "#008236"},
	EventBookingConfirmed:          {bannerBG: "#dcfce7", bannerFG: "#008236", ctaBG: "#008236"},
	EventBookingRejected:           {bannerBG: "#ffe2e2", bannerFG: "#c10007", ctaBG: "#374b5a"},
	EventBookingCancelled:          {bannerBG: "#e5e5e6", bannerFG: "#585a5c", ctaBG: "#374b5a"},
	EventBookingReminder:           {bannerBG: "#e6eeff", bannerFG: "#003660", ctaBG: "#003660"},
	EventBookingOverdue:            {bannerBG: "#ffe2e2", bannerFG: "#c10007", ctaBG: "#d97706"},
	EventIssueCreated:              {bannerBG: "#fef3c7", bannerFG: "#92400e", ctaBG: "#d97706"},
	EventIssueAssignedToMe:         {bannerBG: "#e6eeff", bannerFG: "#003660", ctaBG: "#003660"},
	EventIssueResolved:             {bannerBG: "#dcfce7", bannerFG: "#008236", ctaBG: "#008236"},
	EventIssueCommented:            {bannerBG: "#e6eeff", bannerFG: "#003660", ctaBG: "#003660"},
}

// statusBadge holds inline colors for booking status badges.
type badgeColors struct {
	bg string
	fg string
}

var bookingStatusBadge = map[string]badgeColors{
	"submitted": {bg: "#fef3c7", fg: "#92400e"},
	"confirmed": {bg: "#dcfce7", fg: "#008236"},
	"picked_up": {bg: "#d0e0ff", fg: "#003660"},
	"rejected":  {bg: "#ffe2e2", fg: "#c10007"},
	"cancelled": {bg: "#e5e5e6", fg: "#585a5c"},
	"approved":  {bg: "#d0e0ff", fg: "#003660"},
	"draft":     {bg: "#e5e5e6", fg: "#585a5c"},
	"returned":  {bg: "#e5e5e6", fg: "#585a5c"},
}

var issueSeverityBadge = map[string]badgeColors{
	"usable":   {bg: "#fef3c7", fg: "#92400e"},
	"unusable": {bg: "#ffe2e2", fg: "#c10007"},
	"missing":  {bg: "#ffe2e2", fg: "#c10007"},
}

var issueStatusBadge = map[string]badgeColors{
	"open":        {bg: "#e6eeff", fg: "#003660"},
	"in_progress": {bg: "#fef9c3", fg: "#854d0e"},
	"resolved":    {bg: "#dcfce7", fg: "#008236"},
	"archived":    {bg: "#e5e5e6", fg: "#585a5c"},
}

// BookingEmailData holds all values needed to render a booking email.
type BookingEmailData struct {
	Event         string
	Lang          string
	RecipientName string
	GroupName     string
	LogoURL       string // PNG URL for email header; empty if no logo configured
	BaseURL       string
	BookingID     pgtype.UUID
	StartDate     pgtype.Date
	EndDate       pgtype.Date
	Status        string
	TeamName      string // empty if personal or external
	Notes         string
	Items         []db.ListBookingItemsRow
	Events        []db.ListBookingEventsRow
}

// IssueEmailData holds all values needed to render an issue email.
type IssueEmailData struct {
	Event         string
	Lang          string
	RecipientName string
	GroupName     string
	LogoURL       string // PNG URL for email header; empty if no logo configured
	BaseURL       string
	IssueID      pgtype.UUID
	Title        string
	Severity     string
	Status       string
	Description  string
	ReporterName string
	Events       []db.ListIssueEventsRow
}

func renderBookingEmail(d BookingEmailData) (htmlOut, textOut string) {
	style := eventStyles[d.Event]
	statusBadge := bookingStatusBadge[d.Status]
	if statusBadge.bg == "" {
		statusBadge = badgeColors{bg: "#e5e5e6", fg: "#585a5c"}
	}

	start := formatDate(d.Lang, d.StartDate)
	end := formatDate(d.Lang, d.EndDate)
	bookingURL := d.BaseURL + "/bookings/" + uuidString(d.BookingID)

	teamLabel, teamBG, teamFG := teamBadge(d.Lang, d.TeamName)
	itemsHTML := buildItemsHTML(d.Lang, d.Items)

	bannerLabel := i18n.T(d.Lang, "email_banner_"+d.Event)
	if d.TeamName != "" {
		bannerLabel = d.TeamName + ": " + bannerLabel
	}

	logoHdr := logoHeader(d.LogoURL, d.GroupName)
	replacer := strings.NewReplacer(
		"EMAIL_RECIPIENT_NAME", html.EscapeString(d.RecipientName),
		"EMAIL_LOGO_HEADER", logoHdr,
		"EMAIL_GROUP_NAME", html.EscapeString(d.GroupName),
		"EMAIL_BANNER_LABEL", html.EscapeString(bannerLabel),
		"EMAIL_BANNER_BG", style.bannerBG,
		"EMAIL_BANNER_FG", style.bannerFG,
		"EMAIL_INTRO", html.EscapeString(i18n.T(d.Lang, "email_intro_"+d.Event)),
		"EMAIL_START_DATE", html.EscapeString(start),
		"EMAIL_END_DATE", html.EscapeString(end),
		"EMAIL_TEAM_LABEL", html.EscapeString(teamLabel),
		"EMAIL_TEAM_BG", teamBG,
		"EMAIL_TEAM_FG", teamFG,
		"EMAIL_STATUS", html.EscapeString(i18n.T(d.Lang, "booking_status_"+d.Status)),
		"EMAIL_STATUS_BG", statusBadge.bg,
		"EMAIL_STATUS_FG", statusBadge.fg,
		"EMAIL_ITEMS_HEADING", html.EscapeString(i18n.T(d.Lang, "email_items_heading")),
		"EMAIL_ITEMS_HTML", itemsHTML,
		"EMAIL_NOTES_BLOCK", buildNotesBlock(d.Lang, d.Notes),
		"EMAIL_BOOKING_EVENTS_BLOCK", buildBookingEventsHTML(d.Lang, d.Events),
		"EMAIL_CTA_LABEL", html.EscapeString(i18n.T(d.Lang, "email_cta_"+d.Event)),
		"EMAIL_CTA_URL", bookingURL,
		"EMAIL_CTA_BG", style.ctaBG,
		"EMAIL_UNSUBSCRIBE_URL", d.BaseURL+"/profile",
		"EMAIL_FOOTER_UNSUBSCRIBE", html.EscapeString(i18n.T(d.Lang, "email_footer_unsubscribe")),
	)

	htmlOut = replacer.Replace(bookingTemplate)
	textOut = buildBookingText(d, bannerLabel, start, end, teamLabel, bookingURL)
	return
}

func renderIssueEmail(d IssueEmailData) (htmlOut, textOut string) {
	style := eventStyles[d.Event]
	sevBadge := issueSeverityBadge[d.Severity]
	if sevBadge.bg == "" {
		sevBadge = badgeColors{bg: "#e5e5e6", fg: "#585a5c"}
	}
	stBadge := issueStatusBadge[d.Status]
	if stBadge.bg == "" {
		stBadge = badgeColors{bg: "#e5e5e6", fg: "#585a5c"}
	}

	issueURL := d.BaseURL + "/issues/" + uuidString(d.IssueID)
	desc := truncate(d.Description, 300)

	replacer := strings.NewReplacer(
		"EMAIL_RECIPIENT_NAME", html.EscapeString(d.RecipientName),
		"EMAIL_LOGO_HEADER", logoHeader(d.LogoURL, d.GroupName),
		"EMAIL_GROUP_NAME", html.EscapeString(d.GroupName),
		"EMAIL_BANNER_LABEL", html.EscapeString(i18n.T(d.Lang, "email_banner_"+d.Event)),
		"EMAIL_BANNER_BG", style.bannerBG,
		"EMAIL_BANNER_FG", style.bannerFG,
		"EMAIL_INTRO", html.EscapeString(i18n.T(d.Lang, "email_intro_"+d.Event)),
		"EMAIL_ISSUE_TITLE", html.EscapeString(d.Title),
		"EMAIL_SEVERITY", html.EscapeString(i18n.T(d.Lang, "issue_severity_"+d.Severity)),
		"EMAIL_SEVERITY_BG", sevBadge.bg,
		"EMAIL_SEVERITY_FG", sevBadge.fg,
		"EMAIL_ISSUE_STATUS", html.EscapeString(i18n.T(d.Lang, "issue_status_"+d.Status)),
		"EMAIL_ISSUE_STATUS_BG", stBadge.bg,
		"EMAIL_ISSUE_STATUS_FG", stBadge.fg,
		"EMAIL_DESCRIPTION", html.EscapeString(desc),
		"EMAIL_REPORTER_LINE", html.EscapeString(i18n.T(d.Lang, "email_reporter_line", map[string]string{"name": d.ReporterName})),
		"EMAIL_EVENTS_HTML", buildEventsHTML(d.Lang, d.Events),
		"EMAIL_EVENTS_HEADING", html.EscapeString(i18n.T(d.Lang, "email_events_heading")),
		"EMAIL_CTA_LABEL", html.EscapeString(i18n.T(d.Lang, "email_cta_"+d.Event)),
		"EMAIL_CTA_URL", issueURL,
		"EMAIL_CTA_BG", style.ctaBG,
		"EMAIL_UNSUBSCRIBE_URL", d.BaseURL+"/profile",
		"EMAIL_FOOTER_UNSUBSCRIBE", html.EscapeString(i18n.T(d.Lang, "email_footer_unsubscribe")),
	)

	htmlOut = replacer.Replace(issueTemplate)
	textOut = buildIssueText(d, desc, issueURL)
	return
}

// fetchBookingEmailData loads group name, team name, and items for a booking.
func fetchBookingEmailData(ctx context.Context, q *db.Queries, b db.Booking, event, lang, recipientName, baseURL string) BookingEmailData {
	group, _ := q.GetGroup(ctx, b.GroupID)

	var teamName string
	if b.UsedByTeamID.Valid {
		if t, err := q.GetTeam(ctx, db.GetTeamParams{ID: b.UsedByTeamID, GroupID: b.GroupID}); err == nil {
			teamName = t.Name
		}
	}

	items, _ := q.ListBookingItems(ctx, db.ListBookingItemsParams{BookingID: b.ID, GroupID: b.GroupID})
	events, _ := q.ListBookingEvents(ctx, db.ListBookingEventsParams{BookingID: b.ID, GroupID: b.GroupID})

	return BookingEmailData{
		Event:         event,
		Lang:          lang,
		RecipientName: recipientName,
		GroupName:     group.Name,
		LogoURL:       groupLogoURL(ctx, q, b.GroupID, baseURL),
		BaseURL:       baseURL,
		BookingID:     b.ID,
		StartDate:     b.StartDate,
		EndDate:       b.EndDate,
		Status:        b.Status,
		TeamName:      teamName,
		Notes:         b.Notes,
		Items:         items,
		Events:        events,
	}
}

// fetchIssueEmailData loads group name, reporter name, and event history for an issue.
func fetchIssueEmailData(ctx context.Context, q *db.Queries, issue db.IssueReport, event, lang, recipientName, baseURL string) IssueEmailData {
	group, _ := q.GetGroup(ctx, issue.GroupID)

	var reporterName string
	if u, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID}); err == nil {
		reporterName = u.Name
	}

	allEvents, _ := q.ListIssueEvents(ctx, db.ListIssueEventsParams{IssueID: issue.ID, GroupID: issue.GroupID})
	// The first event is the creation comment, which duplicates issue.Description shown in the card above.
	// Skip it so the history section only shows subsequent activity.
	events := allEvents
	if len(allEvents) > 0 && allEvents[0].EventType == "comment" && allEvents[0].Description == issue.Description {
		events = allEvents[1:]
	}

	return IssueEmailData{
		Event:         event,
		Lang:          lang,
		RecipientName: recipientName,
		GroupName:     group.Name,
		LogoURL:       groupLogoURL(ctx, q, issue.GroupID, baseURL),
		BaseURL:       baseURL,
		IssueID:       issue.ID,
		Title:         issue.Title,
		Severity:      issue.Severity,
		Status:        issue.Status,
		Description:   issue.Description,
		ReporterName:  reporterName,
		Events:        events,
	}
}

// --- Helpers ---

// groupLogoURL returns the absolute PNG logo URL for a group, or empty string if none set.
func groupLogoURL(ctx context.Context, q *db.Queries, groupID, baseURL string) string {
	fileID, err := q.GetGroupLogoFileID(ctx, groupID)
	if err != nil || !fileID.Valid {
		return ""
	}
	return baseURL + "/api/v0/public/groups/" + groupID + "/logo.png"
}

// logoHeader returns an <img> tag for the logo (email PNG variant) when logoURL is set,
// or the HTML-escaped group name otherwise. Substituted into EMAIL_LOGO_HEADER.
func logoHeader(logoURL, groupName string) string {
	if logoURL == "" {
		return html.EscapeString(groupName)
	}
	return `<img src="` + html.EscapeString(logoURL) + `" style="height:50px;max-width:280px;display:block;" alt="` + html.EscapeString(groupName) + `">`
}

func formatDate(lang string, d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	t := d.Time
	if lang == "en" {
		return fmt.Sprintf("%d %s %d", t.Day(), t.Month().String()[:3], t.Year())
	}
	months := []string{"jan", "feb", "mar", "apr", "maj", "jun", "jul", "aug", "sep", "okt", "nov", "dec"}
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func teamBadge(lang, teamName string) (label, bg, fg string) {
	if teamName != "" {
		return teamName, "#e6eeff", "#003660"
	}
	return i18n.T(lang, "email_personal_booking"), "#e5e5e6", "#585a5c"
}

// articleStatusBadge returns inline-style colors for an article status badge, matching BookingItemsList.svelte.
var articleStatusBadge = map[string]badgeColors{
	"ok":                {"#dcfce7", "#008236"},
	"reported_usable":   {"#fef3c7", "#c2410c"},
	"reported_unusable": {"#ffe2e2", "#c10007"},
	"incoming":          {"#e6eeff", "#003660"},
	"under_repair":      {"#f4f4f5", "#52525b"},
	"lost":              {"#ffe2e2", "#c10007"},
	"archived":          {"#f4f4f5", "#52525b"},
}

// itemGroup holds the items sharing a commercial name within a location.
type itemGroup struct {
	commercialName      string
	individuallyTracked bool
	items               []db.ListBookingItemsRow
}

// buildItemsHTML renders the booking item list as card-style HTML, mirroring BookingItemsList.svelte.
// Layout: location label → per-commercial-name card (neutral header + rows).
// For individually tracked: one row per item showing common_name + place.
// For quantity tracked: colored status badge rows with counts.
func buildItemsHTML(lang string, items []db.ListBookingItemsRow) string {
	if len(items) == 0 {
		return ""
	}

	// Build location → groups structure, preserving DB order.
	type locKey = string
	var locOrder []string
	locGroups := map[locKey][]*itemGroup{}
	groupIndex := map[string]*itemGroup{} // key = commercialName+"||"+locationName

	for i := range items {
		it := &items[i]
		gKey := it.CommercialName + "||" + it.LocationName
		g, ok := groupIndex[gKey]
		if !ok {
			g = &itemGroup{commercialName: it.CommercialName, individuallyTracked: it.IndividuallyTracked}
			groupIndex[gKey] = g
			if _, seen := locGroups[it.LocationName]; !seen {
				locOrder = append(locOrder, it.LocationName)
			}
			locGroups[it.LocationName] = append(locGroups[it.LocationName], g)
		}
		g.items = append(g.items, *it)
	}

	var b strings.Builder
	for li, loc := range locOrder {
		if li > 0 {
			b.WriteString(`<div style="height:12px"></div>`)
		}
		// Location header
		fmt.Fprintf(&b,
			`<div style="font-size:11px;font-weight:700;color:#608199;text-transform:uppercase;letter-spacing:0.06em;margin-bottom:4px">%s</div>`,
			html.EscapeString(loc),
		)
		for _, g := range locGroups[loc] {
			count := len(g.items)
			// Card
			b.WriteString(`<table width="100%" cellpadding="0" cellspacing="0" style="border:1px solid #d7e4f0;margin-bottom:6px;border-collapse:collapse">`)
			// Card header
			fmt.Fprintf(&b,
				`<tr><td style="background:#f0f4f8;padding:6px 10px;font-weight:600;color:#131d24;font-size:14px">%s`+
					`<span style="float:right;color:#608199;font-size:13px;font-weight:400">%d st</span></td></tr>`,
				html.EscapeString(g.commercialName), count,
			)
			if g.individuallyTracked {
				for j, it := range g.items {
					borderStyle := ""
					if j > 0 {
						borderStyle = "border-top:1px solid #eef2f7;"
					}
					commonName := html.EscapeString(it.CommonName)
					place := ""
					if it.Place != "" {
						place = fmt.Sprintf(`<span style="color:#608199;margin-left:10px;font-size:13px">%s</span>`, html.EscapeString(it.Place))
					}
					fmt.Fprintf(&b,
						`<tr><td style="padding:5px 10px;font-size:14px;%s">%s%s</td></tr>`,
						borderStyle, commonName, place,
					)
				}
			} else {
				// Group by article_status
				statusCount := map[string]int{}
				var statusOrder []string
				for _, it := range g.items {
					if _, seen := statusCount[it.ArticleStatus]; !seen {
						statusOrder = append(statusOrder, it.ArticleStatus)
					}
					statusCount[it.ArticleStatus]++
				}
				b.WriteString(`<tr><td style="padding:7px 10px">`)
				for si, st := range statusOrder {
					if si > 0 {
						b.WriteString(" ")
					}
					colors, ok := articleStatusBadge[st]
					if !ok {
						colors = badgeColors{bg: "#f4f4f5", fg: "#52525b"}
					}
					label := i18n.T(lang, "article_status_"+st)
					fmt.Fprintf(&b,
						`<span style="background:%s;color:%s;font-size:12px;padding:2px 7px;border-radius:4px">&times;%d %s</span>`,
						colors.bg, colors.fg, statusCount[st], html.EscapeString(label),
					)
				}
				b.WriteString(`</td></tr>`)
			}
			b.WriteString(`</table>`)
		}
	}
	return b.String()
}

// buildEventsHTML renders the issue event history as HTML.
func buildEventsHTML(lang string, events []db.ListIssueEventsRow) string {
	if len(events) == 0 {
		return ""
	}
	var b strings.Builder
	for _, ev := range events {
		var ts string
		if ev.CreatedAt.Valid {
			ts = ev.CreatedAt.Time.Format("2 jan 2006 15:04")
		}
		header := fmt.Sprintf(
			`<span style="color:#608199;font-size:12px">%s &mdash; %s</span>`,
			html.EscapeString(ts),
			html.EscapeString(ev.ActorName),
		)

		var label string
		switch ev.EventType {
		case "comment":
			label = ""
		default:
			label = i18n.T(lang, "issue_event_type_"+ev.EventType)
			if label == "issue_event_type_"+ev.EventType {
				label = ev.EventType // fallback to raw value
			}
			label = `<span style="font-size:13px;font-style:italic;color:#374b5a">` + html.EscapeString(label) + `</span><br>`
		}

		b.WriteString(header)
		b.WriteString("<br>")
		if label != "" {
			b.WriteString(label)
		}
		if ev.Description != "" {
			b.WriteString(html.EscapeString(ev.Description))
			b.WriteString("<br>")
		}
		b.WriteString("<br>")
	}
	return strings.TrimSuffix(b.String(), "<br>")
}

// buildNotesBlock renders the notes section as a self-contained HTML block, or empty string.
func buildNotesBlock(lang, notes string) string {
	if notes == "" {
		return ""
	}
	heading := html.EscapeString(i18n.T(lang, "email_notes_heading"))
	return fmt.Sprintf(
		`<div style="padding-top:16px;border-top:1px solid #d7e4f0;margin-top:8px"><span style="font-weight:600;color:#374b5a">%s</span><br><span style="color:#608199">%s</span></div>`,
		heading, html.EscapeString(notes),
	)
}

// hasNotes reports whether events contains at least one user-written note.
func hasNotes(events []db.ListBookingEventsRow) bool {
	for _, ev := range events {
		if ev.EventType == "note" {
			return true
		}
	}
	return false
}

// buildBookingEventsHTML renders the full booking event thread block (heading + events) as HTML,
// or empty string when there are no user notes.
func buildBookingEventsHTML(lang string, events []db.ListBookingEventsRow) string {
	if !hasNotes(events) {
		return ""
	}
	heading := html.EscapeString(i18n.T(lang, "email_booking_events_heading"))
	var b strings.Builder
	fmt.Fprintf(&b,
		`<div style="padding-top:16px;border-top:1px solid #d7e4f0;margin-top:8px"><span style="font-weight:600;color:#374b5a">%s</span></div>`,
		heading,
	)
	for _, ev := range events {
		var ts string
		if ev.CreatedAt.Valid {
			ts = ev.CreatedAt.Time.Format("2 jan 2006 15:04")
		}
		header := fmt.Sprintf(
			`<span style="color:#608199;font-size:12px">%s &mdash; %s</span>`,
			html.EscapeString(ts),
			html.EscapeString(ev.ActorName),
		)
		b.WriteString(header)
		b.WriteString("<br>")
		if ev.EventType == "note" {
			b.WriteString(html.EscapeString(ev.Message))
			b.WriteString("<br>")
		} else {
			label := i18n.T(lang, "page_booking_event_"+ev.EventType)
			if label == "page_booking_event_"+ev.EventType {
				label = ev.EventType
			}
			fmt.Fprintf(&b,
				`<span style="font-size:13px;font-style:italic;color:#374b5a">%s</span><br>`,
				html.EscapeString(label),
			)
			if ev.Message != "" {
				b.WriteString(html.EscapeString(ev.Message))
				b.WriteString("<br>")
			}
		}
		b.WriteString("<br>")
	}
	return strings.TrimSuffix(b.String(), "<br>")
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "..."
}

// buildBookingText constructs the plain-text version of a booking email.
func buildBookingText(d BookingEmailData, bannerLabel, start, end, teamLabel, bookingURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", bannerLabel)
	fmt.Fprintf(&b, "Hej %s,\n\n", d.RecipientName)
	fmt.Fprintf(&b, "%s\n\n", i18n.T(d.Lang, "email_intro_"+d.Event))
	fmt.Fprintf(&b, "%s - %s\n", start, end)
	fmt.Fprintf(&b, "%s  |  %s\n\n", teamLabel, i18n.T(d.Lang, "booking_status_"+d.Status))
	if len(d.Items) > 0 {
		fmt.Fprintf(&b, "%s\n", i18n.T(d.Lang, "email_items_heading"))
		curLoc := ""
		for _, it := range d.Items {
			if it.LocationName != curLoc {
				curLoc = it.LocationName
				fmt.Fprintf(&b, "  [%s]\n", it.LocationName)
			}
			if it.IndividuallyTracked {
				name := it.CommercialName
				if it.CommonName != "" {
					name += " – " + it.CommonName
				}
				if it.Place != "" {
					name += " (" + it.Place + ")"
				}
				fmt.Fprintf(&b, "  - %s\n", name)
			} else {
				fmt.Fprintf(&b, "  - %s\n", it.CommercialName)
			}
		}
		b.WriteString("\n")
	}
	if d.Notes != "" {
		fmt.Fprintf(&b, "%s\n%s\n\n", i18n.T(d.Lang, "email_notes_heading"), d.Notes)
	}
	if hasNotes(d.Events) {
		fmt.Fprintf(&b, "%s\n", i18n.T(d.Lang, "email_booking_events_heading"))
		for _, ev := range d.Events {
			var ts string
			if ev.CreatedAt.Valid {
				ts = ev.CreatedAt.Time.Format("2 jan 2006 15:04")
			}
			if ev.EventType == "note" {
				fmt.Fprintf(&b, "  %s — %s\n  %s\n\n", ts, ev.ActorName, ev.Message)
			} else {
				label := i18n.T(d.Lang, "page_booking_event_"+ev.EventType)
				fmt.Fprintf(&b, "  %s — %s: %s\n", ts, ev.ActorName, label)
				if ev.Message != "" {
					fmt.Fprintf(&b, "  %s\n", ev.Message)
				}
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%s: %s\n\n", i18n.T(d.Lang, "email_cta_"+d.Event), bookingURL)
	fmt.Fprintf(&b, "---\n%s\n%s: %s\n", d.GroupName, i18n.T(d.Lang, "email_footer_unsubscribe"), d.BaseURL+"/profile")
	return b.String()
}

// buildIssueText constructs the plain-text version of an issue email.
func buildIssueText(d IssueEmailData, desc, issueURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", i18n.T(d.Lang, "email_banner_"+d.Event))
	fmt.Fprintf(&b, "Hej %s,\n\n", d.RecipientName)
	fmt.Fprintf(&b, "%s\n\n", i18n.T(d.Lang, "email_intro_"+d.Event))
	fmt.Fprintf(&b, "%s\n", d.Title)
	fmt.Fprintf(&b, "%s  |  %s\n\n", i18n.T(d.Lang, "issue_severity_"+d.Severity), i18n.T(d.Lang, "issue_status_"+d.Status))
	if desc != "" {
		fmt.Fprintf(&b, "%s\n\n", desc)
	}
	fmt.Fprintf(&b, "%s\n\n", i18n.T(d.Lang, "email_reporter_line", map[string]string{"name": d.ReporterName}))
	if len(d.Events) > 0 {
		fmt.Fprintf(&b, "%s\n", i18n.T(d.Lang, "email_events_heading"))
		for _, ev := range d.Events {
			var ts string
			if ev.CreatedAt.Valid {
				ts = ev.CreatedAt.Time.Format("2 jan 2006 15:04")
			}
			if ev.EventType == "comment" {
				fmt.Fprintf(&b, "  %s — %s\n  %s\n\n", ts, ev.ActorName, ev.Description)
			} else {
				label := i18n.T(d.Lang, "issue_event_type_"+ev.EventType)
				fmt.Fprintf(&b, "  %s — %s: %s\n", ts, ev.ActorName, label)
				if ev.Description != "" {
					fmt.Fprintf(&b, "  %s\n", ev.Description)
				}
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%s: %s\n\n", i18n.T(d.Lang, "email_cta_"+d.Event), issueURL)
	fmt.Fprintf(&b, "---\n%s\n%s: %s\n", d.GroupName, i18n.T(d.Lang, "email_footer_unsubscribe"), d.BaseURL+"/profile")
	return b.String()
}

// timeNow is used for tests to override the current time.
var timeNow = time.Now
