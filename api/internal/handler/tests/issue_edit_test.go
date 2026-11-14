package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

// putIssue sends PUT /issues/{id} and returns the status code and decoded body.
func putIssue(t *testing.T, client *testutil.TestClient, issueID string, body map[string]any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := client.Put("/api/v0/issues/"+issueID, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func getJSON(t *testing.T, client *testutil.TestClient, path string) map[string]any {
	t.Helper()
	resp, err := client.Get(path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func createTestIssue(t *testing.T, client *testutil.TestClient, articleID, severity, description string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    severity,
		"description": description,
	})
	resp, err := client.Post("/api/v0/issues", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create issue: expected 201, got %d", resp.StatusCode)
	}
	var issue map[string]any
	json.NewDecoder(resp.Body).Decode(&issue)
	return issue["id"].(string)
}

func TestIssueFlow_EditDetails(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	reporter := env.ClientAs("leader-yggdrasil")
	otherLeader := env.ClientAs("leader-flaskpost") // book-level, not the reporter
	otherGroup := env.ClientAs("other-kar-leader")

	articleID := createTestArticle(t, manager, "EditIssue")
	issueID := createTestIssue(t, reporter, articleID, "usable", "Liten reva")

	articleStatus := func() any {
		return getJSON(t, reporter, "/api/v0/articles/"+articleID)["status"]
	}

	t.Run("can_edit is true for reporter and manager, false for others", func(t *testing.T) {
		if v := getJSON(t, reporter, "/api/v0/issues/"+issueID)["can_edit"]; v != true {
			t.Errorf("reporter: expected can_edit true, got %v", v)
		}
		if v := getJSON(t, manager, "/api/v0/issues/"+issueID)["can_edit"]; v != true {
			t.Errorf("manager: expected can_edit true, got %v", v)
		}
		if v := getJSON(t, otherLeader, "/api/v0/issues/"+issueID)["can_edit"]; v != false {
			t.Errorf("other leader: expected can_edit false, got %v", v)
		}
	})

	t.Run("reporter edits description, title unchanged", func(t *testing.T) {
		code, issue := putIssue(t, reporter, issueID, map[string]any{"description": "  Stor reva i taket  "})
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d", code)
		}
		if issue["description"] != "Stor reva i taket" {
			t.Errorf("expected trimmed description, got %v", issue["description"])
		}
		if issue["title"] != "EditIssue - Användbar" {
			t.Errorf("expected title unchanged, got %v", issue["title"])
		}
	})

	t.Run("severity change regenerates title and updates article condition", func(t *testing.T) {
		if s := articleStatus(); s != "reported_usable" {
			t.Fatalf("precondition: expected reported_usable, got %v", s)
		}
		code, issue := putIssue(t, reporter, issueID, map[string]any{"severity": "unusable"})
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d", code)
		}
		if issue["severity"] != "unusable" {
			t.Errorf("expected severity unusable, got %v", issue["severity"])
		}
		if issue["title"] != "EditIssue - Ej användbar" {
			t.Errorf("expected regenerated title, got %v", issue["title"])
		}
		if s := articleStatus(); s != "reported_unusable" {
			t.Errorf("expected reported_unusable, got %v", s)
		}

		// And back again
		_, issue = putIssue(t, manager, issueID, map[string]any{"severity": "usable"})
		if issue["title"] != "EditIssue - Användbar" {
			t.Errorf("expected title back to Användbar, got %v", issue["title"])
		}
		if s := articleStatus(); s != "reported_usable" {
			t.Errorf("expected reported_usable after downgrade, got %v", s)
		}
	})

	t.Run("title in the request is ignored", func(t *testing.T) {
		code, issue := putIssue(t, manager, issueID, map[string]any{"title": "Fritext", "severity": "missing"})
		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d", code)
		}
		if issue["title"] != "EditIssue - Saknas" {
			t.Errorf("expected generated title, got %v", issue["title"])
		}
		if s := articleStatus(); s != "reported_missing" {
			t.Errorf("expected reported_missing, got %v", s)
		}
	})

	t.Run("edit is logged as details_changed without a separate title change", func(t *testing.T) {
		issue := getJSON(t, manager, "/api/v0/issues/"+issueID)
		events, _ := issue["events"].([]any)
		var found map[string]any
		for _, e := range events {
			ev := e.(map[string]any)
			if ev["event_type"] != "details_changed" {
				continue
			}
			meta, _ := ev["metadata"].(map[string]any)
			changes, _ := meta["changes"].(map[string]any)
			if _, ok := changes["title"]; ok {
				t.Errorf("title should not be logged as a change: %v", changes)
			}
			if sev, ok := changes["severity"].(map[string]any); ok && sev["old"] == "usable" && sev["new"] == "unusable" {
				found = changes
			}
		}
		if found == nil {
			t.Fatal("expected a details_changed event for usable -> unusable")
		}
	})

	t.Run("unchanged values do not log an event", func(t *testing.T) {
		before := len(getJSON(t, manager, "/api/v0/issues/"+issueID)["events"].([]any))
		putIssue(t, manager, issueID, map[string]any{"description": "Stor reva i taket", "severity": "missing"})
		after := len(getJSON(t, manager, "/api/v0/issues/"+issueID)["events"].([]any))
		if after != before {
			t.Errorf("expected no new event, had %d now %d", before, after)
		}
	})

	t.Run("validation", func(t *testing.T) {
		if code, _ := putIssue(t, reporter, issueID, map[string]any{"severity": "broken"}); code != http.StatusBadRequest {
			t.Errorf("invalid severity: expected 400, got %d", code)
		}
		if code, _ := putIssue(t, reporter, issueID, map[string]any{"description": "  "}); code != http.StatusBadRequest {
			t.Errorf("blank description: expected 400, got %d", code)
		}
	})

	t.Run("other leader cannot edit", func(t *testing.T) {
		for _, body := range []map[string]any{
			{"description": "Hacked"},
			{"severity": "usable"},
		} {
			if code, _ := putIssue(t, otherLeader, issueID, body); code != http.StatusForbidden {
				t.Errorf("%v: expected 403, got %d", body, code)
			}
		}
	})

	t.Run("other group cannot see or edit", func(t *testing.T) {
		if code, _ := putIssue(t, otherGroup, issueID, map[string]any{"description": "Hacked"}); code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", code)
		}
	})

	t.Run("resolved issue is locked", func(t *testing.T) {
		if code, _ := putIssue(t, manager, issueID, map[string]any{"status": "resolved"}); code != http.StatusOK {
			t.Fatalf("resolve: expected 200, got %d", code)
		}
		if code, _ := putIssue(t, reporter, issueID, map[string]any{"description": "Late edit"}); code != http.StatusConflict {
			t.Errorf("reporter: expected 409, got %d", code)
		}
		if code, _ := putIssue(t, manager, issueID, map[string]any{"severity": "unusable"}); code != http.StatusConflict {
			t.Errorf("manager: expected 409, got %d", code)
		}
		if v := getJSON(t, manager, "/api/v0/issues/"+issueID)["can_edit"]; v != false {
			t.Errorf("expected can_edit false once resolved, got %v", v)
		}
		if s := articleStatus(); s != "ok" {
			t.Errorf("expected article ok after resolve, got %v", s)
		}
	})
}

// A title generated in English stays English when the severity changes,
// even if the person editing uses Swedish.
func TestIssueFlow_EditKeepsTitleLanguage(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	articleID := createTestArticle(t, manager, "LangTitle")

	b, _ := json.Marshal(map[string]any{"article_id": articleID, "severity": "usable", "description": "English report"})
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/v0/issues", bytes.NewReader(b))
	req.Header.Set("X-Dev-Role-Override", "leader-yggdrasil")
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "paraglide_lang", Value: "en"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var issue map[string]any
	json.NewDecoder(resp.Body).Decode(&issue)
	resp.Body.Close()
	if issue["title"] != "LangTitle - Usable" {
		t.Fatalf("precondition: expected English title, got %v", issue["title"])
	}

	_, updated := putIssue(t, manager, issue["id"].(string), map[string]any{"severity": "unusable"})
	if updated["title"] != "LangTitle - Unusable" {
		t.Errorf("expected English regenerated title, got %v", updated["title"])
	}
}
