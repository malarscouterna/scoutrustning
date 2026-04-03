package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestArticleCRUD(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")
	leader := env.ClientAs("leader-yggdrasil")

	// Get seed location and category IDs
	resp, _ := manager.Get("/api/v1/locations")
	var locations []map[string]any
	json.NewDecoder(resp.Body).Decode(&locations)
	resp.Body.Close()
	locID := locations[0]["id"].(string)

	resp, _ = manager.Get("/api/v1/categories")
	var categories []map[string]any
	json.NewDecoder(resp.Body).Decode(&categories)
	resp.Body.Close()
	catID := categories[0]["id"].(string)

	var articleID string

	t.Run("manager can create article", func(t *testing.T) {
		body := map[string]any{
			"common_name":         "Sibley 10",
			"commercial_name":     "Sibley 600 Twin Ultimate",
			"category_id":         catID,
			"location_id":         locID,
			"individually_tracked": true,
			"place":               "Shelf 3",
		}
		b, _ := json.Marshal(body)
		resp, err := manager.Post("/api/v1/articles", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, respBody)
		}

		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		articleID = article["id"].(string)

		if article["common_name"] != "Sibley 10" {
			t.Errorf("expected common_name Sibley 10, got %v", article["common_name"])
		}
		if article["status"] != "ok" {
			t.Errorf("expected status ok, got %v", article["status"])
		}
	})

	t.Run("leader can list articles", func(t *testing.T) {
		resp, err := leader.Get("/api/v1/articles")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		if len(articles) != 1 {
			t.Fatalf("expected 1 article, got %d", len(articles))
		}
		if articles[0]["location_name"] == nil {
			t.Error("expected location_name to be populated")
		}
	})

	t.Run("leader cannot create article", func(t *testing.T) {
		body := map[string]any{
			"common_name": "Nope",
			"category_id": catID,
			"location_id": locID,
		}
		b, _ := json.Marshal(body)
		resp, err := leader.Post("/api/v1/articles", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("manager can delete article", func(t *testing.T) {
		resp, err := manager.Delete("/api/v1/articles/" + articleID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
	})
}

func TestArticleCSVImport(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")

	t.Run("import CSV creates articles and auto-creates categories", func(t *testing.T) {
		csvPath := findCSVPath()
		csvFile, err := os.Open(csvPath)
		if err != nil {
			t.Skipf("CSV file not found at %s, skipping import test", csvPath)
		}
		defer csvFile.Close()

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "inventory.csv")
		io.Copy(part, csvFile)
		writer.Close()

		req, _ := http.NewRequest("POST", env.Server.URL+"/api/v1/articles/import", &buf)
		req.Header.Set("X-Dev-Role-Override", "equipment-manager")
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		imported := int(result["imported"].(float64))
		if imported == 0 {
			t.Fatal("expected some articles to be imported")
		}
		t.Logf("imported %d articles, skipped %v", imported, result["skipped"])

		// Verify articles were created
		resp2, _ := manager.Get("/api/v1/articles")
		var articles []map[string]any
		json.NewDecoder(resp2.Body).Decode(&articles)
		resp2.Body.Close()

		if len(articles) != imported {
			t.Errorf("expected %d articles in list, got %d", imported, len(articles))
		}

		// Verify categories were auto-created
		resp3, _ := manager.Get("/api/v1/categories")
		var categories []map[string]any
		json.NewDecoder(resp3.Body).Decode(&categories)
		resp3.Body.Close()

		// Should have more than just the seed "Övrigt"
		if len(categories) < 5 {
			t.Errorf("expected multiple categories from import, got %d", len(categories))
		}
		t.Logf("categories after import: %d", len(categories))
	})

	t.Run("leader cannot import", func(t *testing.T) {
		leader := env.ClientAs("leader-yggdrasil")
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "test.csv")
		part.Write([]byte("a,b,c\n"))
		writer.Close()

		req, _ := http.NewRequest("POST", env.Server.URL+"/api/v1/articles/import", &buf)
		req.Header.Set("X-Dev-Role-Override", "leader-yggdrasil")
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := leader.Do("POST", "/api/v1/articles/import", &buf)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})
}

func findCSVPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "docs", "import-example.csv")
}
