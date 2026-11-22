package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

// Vera Visa (the view-only persona) is also manager of the seeded district:
// with the district active she resolves against the prefixed tenant key, gets
// her manager access from the district team, and only sees the district's data.
func TestDistrictOrganisation_TenantResolution(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Get("/me", (&handler.MeHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Get)
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
	})

	_, err := env.Pool.Exec(t.Context(), `
		INSERT INTO locations (group_id, name, sort_order) VALUES
			('district:111', 'Distriktsförrådet', 1),
			('766', 'Kårens förråd', 2)
	`)
	if err != nil {
		t.Fatalf("seed locations: %v", err)
	}

	client := env.ClientAs("view-only").WithActiveGroup("district:111")

	resp, err := client.Get("/api/v0/me")
	if err != nil {
		t.Fatal(err)
	}
	var me map[string]any
	if err := json.Unmarshal(expectStatus(t, resp, http.StatusOK), &me); err != nil {
		t.Fatal(err)
	}
	if me["group_id"] != "district:111" {
		t.Errorf("group_id = %v, want district:111", me["group_id"])
	}
	if me["group_type"] != "district" {
		t.Errorf("group_type = %v, want district", me["group_type"])
	}
	if me["max_access"] != "manager" {
		t.Errorf("max_access = %v, want manager", me["max_access"])
	}

	groups, _ := me["groups"].([]any)
	foundDistrict := false
	for _, g := range groups {
		group, _ := g.(map[string]any)
		if group["id"] == "district:111" &&
			group["type"] == "district" &&
			group["scoutnet_id"] == "111" &&
			group["name"] == "Mälardalens distrikt" {
			foundDistrict = true
		}
	}
	if !foundDistrict {
		t.Errorf("expected district:111 in groups, got %v", groups)
	}

	resp, err = client.Get("/api/v0/locations")
	if err != nil {
		t.Fatal(err)
	}
	var locations []map[string]any
	if err := json.Unmarshal(expectStatus(t, resp, http.StatusOK), &locations); err != nil {
		t.Fatal(err)
	}
	if len(locations) != 1 || locations[0]["name"] != "Distriktsförrådet" {
		t.Errorf("expected only the district location, got %v", locations)
	}
}

// Signup accepts a district membership and generates an init command with the
// organisation type so provisioning creates the right tenant.
func TestJoinAcceptsDistrict(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	notifier := &notifications.CapturingNotifier{}
	join := &handler.JoinHandler{Notifier: notifier, AdminTo: "admin@test.example"}
	env.V1(func(r chi.Router) {
		r.Post("/join", join.Submit)
	})

	body, _ := json.Marshal(map[string]any{
		"org_id":                      "111",
		"org_type":                    "district",
		"group_name":                  "Mälardalens distrikt",
		"role_name":                   "Distriktsstyrelsen",
		"role_key":                    "distriktsstyrelsen",
		"team_name":                   "Distriktsstyrelsen",
		"contact_email":               "distrikt@example.com",
		"group_size":                  "1200",
		"interested_in_custom_domain": false,
	})
	resp, err := env.ClientAs("view-only").Post("/api/v0/join", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	admin := waitForMessage(t, notifier, func(m notifications.Message) bool {
		return m.To == "admin@test.example"
	})
	if !strings.Contains(admin.TextBody, "--org-type=district") {
		t.Errorf("expected district init command, got:\n%s", admin.TextBody)
	}
	if !strings.Contains(admin.TextBody, "--org-id=111") {
		t.Errorf("expected org id in init command, got:\n%s", admin.TextBody)
	}
	if !strings.Contains(admin.Subject, "organisation signup") {
		t.Errorf("admin subject = %q, want organisation signup", admin.Subject)
	}

	applicant := waitForMessage(t, notifier, func(m notifications.Message) bool {
		return m.To == "distrikt@example.com"
	})
	if !strings.Contains(applicant.TextBody, "Organisationstyp: Distrikt") {
		t.Errorf("expected localized organisation type in applicant email, got:\n%s", applicant.TextBody)
	}
}
