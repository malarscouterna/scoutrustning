package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func TestTeams_CRUD(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	t.Run("manager can create team", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"name": "Nytt lag", "type": "troop", "access_level": "trusted"})
		resp, err := manager.Post("/api/v0/teams", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var team map[string]any
		json.NewDecoder(resp.Body).Decode(&team)
		if team["name"] != "Nytt lag" {
			t.Errorf("expected Nytt lag, got %v", team["name"])
		}
		if team["access_level"] != "trusted" {
			t.Errorf("expected trusted, got %v", team["access_level"])
		}
	})

	t.Run("leader cannot create team", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"name": "Nope"})
		resp, err := leader.Post("/api/v0/teams", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("all users can list teams", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/teams")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var teams []map[string]any
		json.NewDecoder(resp.Body).Decode(&teams)
		// Seed creates 7 teams for group 766, plus the one we just created
		if len(teams) < 7 {
			t.Fatalf("expected at least 7 teams, got %d", len(teams))
		}
	})
}

func TestImport_ApprovalLevelFromCSV(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("manager-equipment")

	csvContent := strings.Join([]string{
		"titelgrupp,title,description,location,plats,rum,lage,tags,requires_approval",
		"Sibley,Sibley 1,,Hajkförrådet,,,,Sova,",
		"Boombox,Boombox 1,,Karsvik,Ladan,,,Elektronik,low",
		"Chainsaw,Chainsaw 1,,Verkstan,,,,Verktyg,high",
		"Tent,Tent 1,,Hajkförrådet,,,,Sova,false",
	}, "\n")

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.csv")
	part.Write([]byte(csvContent))
	writer.Close()

	req, _ := http.NewRequest("POST", env.Server.URL+"/api/v0/articles/import", &buf)
	req.Header.Set("X-Dev-Role-Override", "manager-equipment")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// List articles and check approval_level
	resp, _ = manager.Get("/api/v0/articles")
	var articles []map[string]any
	json.NewDecoder(resp.Body).Decode(&articles)
	resp.Body.Close()

	expected := map[string]string{
		"Sibley 1":    "none",
		"Boombox 1":   "low",
		"Chainsaw 1":  "high",
		"Tent 1":      "none",
	}
	for _, a := range articles {
		name := a["common_name"].(string)
		level := a["approval_level"].(string)
		if want, ok := expected[name]; ok {
			if level != want {
				t.Errorf("%s: expected approval_level %q, got %q", name, want, level)
			}
		}
	}
}
