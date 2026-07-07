package notifications

import (
	_ "embed"
	"fmt"
	"html"
	"strings"

	"github.com/malarscouterna/scoutrustning/api/internal/i18n"
)

//go:embed templates/join.html
var joinTemplate string

// JoinAdminEmailData holds the values needed to render the internal
// notification sent to ADMIN_EMAIL when a group signup form is submitted.
type JoinAdminEmailData struct {
	ApplicantName      string
	ApplicantEmail     string
	OrgID              string
	GroupName          string
	RoleName           string
	TeamName           string
	ContactEmail       string
	GroupSize          string
	InterestedInDomain bool
	InitGroupCmd       string
}

// JoinApplicantEmailData holds the values needed to render the
// confirmation email sent to the applicant's given contact email.
type JoinApplicantEmailData struct {
	Lang               string
	Name               string
	OrgID              string
	GroupName          string
	RoleName           string
	ContactEmail       string
	GroupSize          string
	InterestedInDomain bool
}

func RenderJoinAdminEmail(d JoinAdminEmailData) (htmlOut, textOut string) {
	rows := []struct{ label, value string }{
		{"Applicant", d.ApplicantName + " <" + d.ApplicantEmail + ">"},
		{"Org ID", d.OrgID},
		{"Group name", d.GroupName},
		{"Role", d.RoleName},
		{"Manager team name", d.TeamName},
		{"Contact email", d.ContactEmail},
		{"Group size", d.GroupSize},
		{"Interested in custom domain", yesNo(d.InterestedInDomain)},
	}

	var body strings.Builder
	for _, row := range rows {
		body.WriteString("<strong>" + html.EscapeString(row.label) + ":</strong> " + html.EscapeString(row.value) + "<br>")
	}
	body.WriteString("<br>Run:<br><code style=\"background:#f1f5f9;padding:2px 6px;border-radius:4px;\">" + html.EscapeString(d.InitGroupCmd) + "</code>")

	title := "New group signup: " + d.GroupName
	replacer := strings.NewReplacer(
		"EMAIL_TITLE", html.EscapeString(title),
		"EMAIL_BODY_HTML", body.String(),
	)
	htmlOut = replacer.Replace(joinTemplate)

	var text strings.Builder
	text.WriteString(title + "\n\n")
	for _, row := range rows {
		text.WriteString(row.label + ": " + row.value + "\n")
	}
	text.WriteString("\nRun:\n" + d.InitGroupCmd + "\n")
	textOut = text.String()
	return
}

func RenderJoinApplicantEmail(d JoinApplicantEmailData) (htmlOut, textOut string) {
	title := i18n.T(d.Lang, "email_join_applicant_title")
	intro := i18n.T(d.Lang, "email_join_applicant_body", map[string]string{
		"name":          d.Name,
		"group_name":    d.GroupName,
		"contact_email": d.ContactEmail,
	})

	domainAnswer := i18n.T(d.Lang, "email_join_summary_no")
	if d.InterestedInDomain {
		domainAnswer = i18n.T(d.Lang, "email_join_summary_yes")
	}
	rows := []struct{ label, value string }{
		{i18n.T(d.Lang, "email_join_summary_group_name"), d.GroupName},
		{i18n.T(d.Lang, "email_join_summary_org_id"), d.OrgID},
		{i18n.T(d.Lang, "email_join_summary_role"), d.RoleName},
		{i18n.T(d.Lang, "email_join_summary_contact_email"), d.ContactEmail},
		{i18n.T(d.Lang, "email_join_summary_group_size"), d.GroupSize},
		{i18n.T(d.Lang, "email_join_summary_domain_interest"), domainAnswer},
	}

	var body strings.Builder
	body.WriteString(intro)
	body.WriteString("<br><br><strong>" + html.EscapeString(i18n.T(d.Lang, "email_join_summary_heading")) + "</strong><br>")
	for _, row := range rows {
		body.WriteString(html.EscapeString(row.label) + ": " + html.EscapeString(row.value) + "<br>")
	}

	replacer := strings.NewReplacer(
		"EMAIL_TITLE", html.EscapeString(title),
		"EMAIL_BODY_HTML", body.String(),
	)
	htmlOut = replacer.Replace(joinTemplate)

	var text strings.Builder
	text.WriteString(title + "\n\n" + stripTags(intro) + "\n\n" + i18n.T(d.Lang, "email_join_summary_heading") + "\n")
	for _, row := range rows {
		text.WriteString(row.label + ": " + row.value + "\n")
	}
	textOut = text.String()
	return
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func stripTags(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<strong>", "")
	s = strings.ReplaceAll(s, "</strong>", "")
	return s
}

// JoinAdminSubject returns the (untranslated - internal/technical) admin email subject.
func JoinAdminSubject(groupName string) string {
	return fmt.Sprintf("New group signup: %s", groupName)
}
