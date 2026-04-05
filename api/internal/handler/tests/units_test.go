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

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestUnits_CRUD(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/units", (&handler.UnitHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	t.Run("manager can create unit", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"name": "Yggdrasil"})
		resp, err := manager.Post("/api/v0/units", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var unit map[string]any
		json.NewDecoder(resp.Body).Decode(&unit)
		if unit["name"] != "Yggdrasil" {
			t.Errorf("expected Yggdrasil, got %v", unit["name"])
		}
	})

	t.Run("leader cannot create unit", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"name": "Nope"})
		resp, err := leader.Post("/api/v0/units", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("all users can list units", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/units")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var units []map[string]any
		json.NewDecoder(resp.Body).Decode(&units)
		if len(units) != 1 {
			t.Fatalf("expected 1 unit, got %d", len(units))
		}
	})
}

func TestImport_RequiresApprovalByLocation(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("manager-equipment")

	csvContent := strings.Join([]string{
		"titelgrupp,title,description,location,plats,rum,lage,tags,secondtag,kit,custodian,status,inventory_date,inventory_status,available,repair,instock,reserved,purchasedate,value,retailer,retailname",
		"Sibley,Sibley 1,,Hajkförrådet,,,,Sova,,,,,,,,,,,,,,",
		"Boombox,Boombox 1,,Karsvik,Ladan,,,Elektronik,,,,,,,,,,,,,,",
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

	// List articles and check requires_approval
	resp, _ = manager.Get("/api/v0/articles")
	var articles []map[string]any
	json.NewDecoder(resp.Body).Decode(&articles)
	resp.Body.Close()

	for _, a := range articles {
		name := a["common_name"].(string)
		approval := a["requires_approval"].(bool)
		if name == "Sibley 1" && approval {
			t.Error("Sibley 1 (Hajkförrådet) should NOT require approval")
		}
		if name == "Boombox 1" && !approval {
			t.Error("Boombox 1 (Ladan) SHOULD require approval")
		}
	}
}
