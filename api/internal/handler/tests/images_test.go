package tests

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/images"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestImageUpload(t *testing.T) {
	images.InitVips()
	t.Cleanup(images.ShutdownVips)

	env := testutil.SetupTestEnv(t)

	imageDir := t.TempDir()

	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/images", (&images.Handler{Q: env.Queries, ImageDir: imageDir}).Routes())
	})

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	// Get seed IDs
	resp, _ := manager.Get("/api/v0/locations")
	var locations []map[string]any
	json.NewDecoder(resp.Body).Decode(&locations)
	resp.Body.Close()
	locID := locations[0]["id"].(string)

	resp, _ = manager.Get("/api/v0/categories")
	var categories []map[string]any
	json.NewDecoder(resp.Body).Decode(&categories)
	resp.Body.Close()
	catID := categories[0]["id"].(string)

	// Create two articles with same commercial_name + location
	for i := range 2 {
		names := []string{"Sibley 1", "Sibley 2"}
		body := map[string]any{
			"commercial_name":    "Sibley",
			"common_name":        names[i],
			"category_id":        catID,
			"location_id":        locID,
			"individually_tracked": true,
		}
		b, _ := json.Marshal(body)
		resp, _ = manager.Post("/api/v0/articles", bytes.NewReader(b))
		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("create article: expected 201, got %d: %s", resp.StatusCode, respBody)
		}
		resp.Body.Close()
	}

	testJPEG := createTestJPEG(t)

	t.Run("leader cannot upload product image", func(t *testing.T) {
		body, contentType := buildMultipartUpload(t, testJPEG, "Sibley", locID)
		req, _ := http.NewRequest("POST", leader.BaseURL()+"/api/v0/images/product", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Dev-Role-Override", "leader-yggdrasil")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
	})

	var imageID string

	t.Run("manager uploads product image", func(t *testing.T) {
		body, contentType := buildMultipartUpload(t, testJPEG, "Sibley", locID)
		req, _ := http.NewRequest("POST", manager.BaseURL()+"/api/v0/images/product", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Dev-Role-Override", "manager-equipment")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		imageID = result["image_id"]
		if imageID == "" {
			t.Fatal("expected image_id in response")
		}

		// Verify files on disk
		if _, err := os.Stat(filepath.Join(imageDir, imageID+".webp")); err != nil {
			t.Errorf("source file not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(imageDir, imageID+"_thumb.webp")); err != nil {
			t.Errorf("thumbnail file not found: %v", err)
		}
	})

	t.Run("image_path set on both articles", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/articles")
		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		resp.Body.Close()

		for _, a := range articles {
			if a["commercial_name"] == "Sibley" {
				if a["image_path"] != imageID {
					t.Errorf("article %s: expected image_path %q, got %v", a["common_name"], imageID, a["image_path"])
				}
			}
		}
	})

	t.Run("serve WebP source", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/images/" + imageID + ".webp")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/webp" {
			t.Errorf("expected image/webp, got %s", ct)
		}
		if cc := resp.Header.Get("Cache-Control"); cc == "" {
			t.Error("expected Cache-Control header")
		}
	})

	t.Run("serve WebP thumbnail", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/images/" + imageID + "_thumb.webp")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("serve JPEG download", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/images/" + imageID + ".webp?format=jpeg")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("expected image/jpeg, got %s", ct)
		}
		if cd := resp.Header.Get("Content-Disposition"); cd == "" {
			t.Error("expected Content-Disposition header for download")
		}
	})

	t.Run("serve 404 for nonexistent image", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/images/00000000-0000-0000-0000-000000000000.webp")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("replace image deletes old files", func(t *testing.T) {
		oldID := imageID

		body, contentType := buildMultipartUpload(t, testJPEG, "Sibley", locID)
		req, _ := http.NewRequest("POST", manager.BaseURL()+"/api/v0/images/product", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Dev-Role-Override", "manager-equipment")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		imageID = result["image_id"]

		// Old files should be gone
		if _, err := os.Stat(filepath.Join(imageDir, oldID+".webp")); !os.IsNotExist(err) {
			t.Error("old source file should have been deleted")
		}
		if _, err := os.Stat(filepath.Join(imageDir, oldID+"_thumb.webp")); !os.IsNotExist(err) {
			t.Error("old thumbnail should have been deleted")
		}

		// New files should exist
		if _, err := os.Stat(filepath.Join(imageDir, imageID+".webp")); err != nil {
			t.Errorf("new source file not found: %v", err)
		}
	})

	t.Run("delete product image", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", manager.BaseURL()+"/api/v0/images/product?commercial_name=Sibley&location_id="+locID, nil)
		req.Header.Set("X-Dev-Role-Override", "manager-equipment")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		// Files should be gone
		if _, err := os.Stat(filepath.Join(imageDir, imageID+".webp")); !os.IsNotExist(err) {
			t.Error("source file should have been deleted")
		}

		// image_path should be cleared on articles
		resp2, _ := manager.Get("/api/v0/articles")
		var articles []map[string]any
		json.NewDecoder(resp2.Body).Decode(&articles)
		resp2.Body.Close()
		for _, a := range articles {
			if a["commercial_name"] == "Sibley" && a["image_path"] != nil {
				t.Errorf("article %s: expected nil image_path, got %v", a["common_name"], a["image_path"])
			}
		}
	})

	t.Run("leader can upload issue image", func(t *testing.T) {
		body, contentType := buildIssueMultipartUpload(t, testJPEG)
		req, _ := http.NewRequest("POST", leader.BaseURL()+"/api/v0/images/issue", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Dev-Role-Override", "leader-yggdrasil")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		issueImageID := result["image_id"]
		if issueImageID == "" {
			t.Fatal("expected image_id")
		}

		// Verify files
		if _, err := os.Stat(filepath.Join(imageDir, issueImageID+".webp")); err != nil {
			t.Errorf("issue source file not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(imageDir, issueImageID+"_thumb.webp")); err != nil {
			t.Errorf("issue thumbnail not found: %v", err)
		}
	})
}

func createTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := range 600 {
		for x := range 800 {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func buildMultipartUpload(t *testing.T, jpegData []byte, commercialName, locationID string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "test.jpg")
	fw.Write(jpegData)
	w.WriteField("commercial_name", commercialName)
	w.WriteField("location_id", locationID)
	w.Close()
	return &buf, w.FormDataContentType()
}

func buildIssueMultipartUpload(t *testing.T, jpegData []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "issue.jpg")
	fw.Write(jpegData)
	w.Close()
	return &buf, w.FormDataContentType()
}
