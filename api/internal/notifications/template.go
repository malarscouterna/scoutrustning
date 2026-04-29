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
	BaseURL       string
	BookingID     pgtype.UUID
	StartDate     pgtype.Date
	EndDate       pgtype.Date
	Status        string
	TeamName      string // empty if personal or external
	Notes         string
	Items         []db.ListBookingItemsRow
}

// IssueEmailData holds all values needed to render an issue email.
type IssueEmailData struct {
	Event        string
	Lang         string
	RecipientName string
	GroupName    string
	BaseURL      string
	IssueID      pgtype.UUID
	Title        string
	Severity     string
	Status       string
	Description  string
	ReporterName string
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
	itemsHTML := buildItemsHTML(d.Items)
	notesHTML := html.EscapeString(d.Notes)

	replacer := strings.NewReplacer(
		"EMAIL_RECIPIENT_NAME", html.EscapeString(d.RecipientName),
		"EMAIL_GROUP_NAME", html.EscapeString(d.GroupName),
		"EMAIL_BANNER_LABEL", html.EscapeString(i18n.T(d.Lang, "email_banner_"+d.Event)),
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
		"EMAIL_NOTES_HEADING", html.EscapeString(i18n.T(d.Lang, "email_notes_heading")),
		"EMAIL_NOTES", notesHTML,
		"EMAIL_CTA_LABEL", html.EscapeString(i18n.T(d.Lang, "email_cta_"+d.Event)),
		"EMAIL_CTA_URL", bookingURL,
		"EMAIL_CTA_BG", style.ctaBG,
		"EMAIL_UNSUBSCRIBE_URL", d.BaseURL+"/profile",
		"EMAIL_FOOTER_UNSUBSCRIBE", html.EscapeString(i18n.T(d.Lang, "email_footer_unsubscribe")),
	)

	htmlOut = replacer.Replace(bookingTemplate)
	textOut = buildBookingText(d, start, end, teamLabel, bookingURL)
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

	return BookingEmailData{
		Event:         event,
		Lang:          lang,
		RecipientName: recipientName,
		GroupName:     group.Name,
		BaseURL:       baseURL,
		BookingID:     b.ID,
		StartDate:     b.StartDate,
		EndDate:       b.EndDate,
		Status:        b.Status,
		TeamName:      teamName,
		Notes:         b.Notes,
		Items:         items,
	}
}

// fetchIssueEmailData loads group name and reporter name for an issue.
func fetchIssueEmailData(ctx context.Context, q *db.Queries, issue db.IssueReport, event, lang, recipientName, baseURL string) IssueEmailData {
	group, _ := q.GetGroup(ctx, issue.GroupID)

	var reporterName string
	if u, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID}); err == nil {
		reporterName = u.Name
	}

	return IssueEmailData{
		Event:         event,
		Lang:          lang,
		RecipientName: recipientName,
		GroupName:     group.Name,
		BaseURL:       baseURL,
		IssueID:       issue.ID,
		Title:         issue.Title,
		Severity:      issue.Severity,
		Status:        issue.Status,
		Description:   issue.Description,
		ReporterName:  reporterName,
	}
}

// --- Helpers ---

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

// buildItemsHTML renders the booking item list as HTML lines.
// Mirrors the grouping logic in IssueCard's articleNames derived value.
func buildItemsHTML(items []db.ListBookingItemsRow) string {
	if len(items) == 0 {
		return ""
	}
	type qtKey struct{ name, loc string }
	qtCounts := map[qtKey]int{}
	var lines []string

	for _, it := range items {
		if it.IndividuallyTracked {
			name := html.EscapeString(it.CommercialName)
			if it.CommonName != "" {
				name += " &ndash; " + html.EscapeString(it.CommonName)
			}
			lines = append(lines, name)
		} else {
			k := qtKey{it.CommercialName, it.LocationName}
			qtCounts[k]++
		}
	}
	for k, count := range qtCounts {
		if count > 1 {
			lines = append(lines, fmt.Sprintf("%s (%d st)", html.EscapeString(k.name), count))
		} else {
			lines = append(lines, html.EscapeString(k.name))
		}
	}

	return strings.Join(lines, "<br>")
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "..."
}

// buildBookingText constructs the plain-text version of a booking email.
func buildBookingText(d BookingEmailData, start, end, teamLabel, bookingURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", i18n.T(d.Lang, "email_banner_"+d.Event))
	fmt.Fprintf(&b, "Hej %s,\n\n", d.RecipientName)
	fmt.Fprintf(&b, "%s\n\n", i18n.T(d.Lang, "email_intro_"+d.Event))
	fmt.Fprintf(&b, "%s - %s\n", start, end)
	fmt.Fprintf(&b, "%s  |  %s\n\n", teamLabel, i18n.T(d.Lang, "booking_status_"+d.Status))
	if len(d.Items) > 0 {
		fmt.Fprintf(&b, "%s\n", i18n.T(d.Lang, "email_items_heading"))
		for _, it := range d.Items {
			if it.IndividuallyTracked {
				if it.CommonName != "" {
					fmt.Fprintf(&b, "- %s - %s\n", it.CommercialName, it.CommonName)
				} else {
					fmt.Fprintf(&b, "- %s\n", it.CommercialName)
				}
			} else {
				fmt.Fprintf(&b, "- %s\n", it.CommercialName)
			}
		}
		b.WriteString("\n")
	}
	if d.Notes != "" {
		fmt.Fprintf(&b, "%s\n%s\n\n", i18n.T(d.Lang, "email_notes_heading"), d.Notes)
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
	fmt.Fprintf(&b, "%s: %s\n\n", i18n.T(d.Lang, "email_cta_"+d.Event), issueURL)
	fmt.Fprintf(&b, "---\n%s\n%s: %s\n", d.GroupName, i18n.T(d.Lang, "email_footer_unsubscribe"), d.BaseURL+"/profile")
	return b.String()
}

// timeNow is used for tests to override the current time.
var timeNow = time.Now
