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
			"commercial_name":      "Sibley",
			"common_name":          names[i],
			"category_id":          catID,
			"location_id":          locID,
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

	// Helper to upload and return response
	uploadProduct := func(t *testing.T, persona string) (*http.Response, error) {
		t.Helper()
		body, contentType := buildMultipartUpload(t, testJPEG, "Sibley", locID)
		req, _ := http.NewRequest("POST", manager.BaseURL()+"/api/v0/images/product", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Dev-Role-Override", persona)
		return http.DefaultClient.Do(req)
	}

	t.Run("leader cannot upload product image", func(t *testing.T) {
		resp, err := uploadProduct(t, "leader-yggdrasil")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
	})

	var image1, image2 string

	t.Run("upload first image", func(t *testing.T) {
		resp, err := uploadProduct(t, "manager-equipment")
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
		image1 = result["image_id"].(string)
		ids := result["image_ids"].([]any)
		if len(ids) != 1 || ids[0].(string) != image1 {
			t.Errorf("expected [%s], got %v", image1, ids)
		}

		if _, err := os.Stat(filepath.Join(imageDir, image1+".webp")); err != nil {
			t.Errorf("source file not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(imageDir, image1+"_thumb.webp")); err != nil {
			t.Errorf("thumbnail not found: %v", err)
		}
	})

	t.Run("upload second image appends", func(t *testing.T) {
		resp, err := uploadProduct(t, "manager-equipment")
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
		image2 = result["image_id"].(string)
		ids := result["image_ids"].([]any)
		if len(ids) != 2 {
			t.Fatalf("expected 2 images, got %d", len(ids))
		}
		if ids[0].(string) != image1 || ids[1].(string) != image2 {
			t.Errorf("expected [%s, %s], got %v", image1, image2, ids)
		}
	})

	t.Run("image_ids set on both articles", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/articles")
		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		resp.Body.Close()

		for _, a := range articles {
			if a["commercial_name"] == "Sibley" {
				ids, ok := a["image_ids"].([]any)
				if !ok || len(ids) != 2 {
					t.Errorf("article %s: expected 2 image_ids, got %v", a["common_name"], a["image_ids"])
				}
			}
		}
	})

	t.Run("serve WebP source", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/images/" + image1 + ".webp")
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

	t.Run("serve JPEG download", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/images/" + image1 + ".webp?format=jpeg")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("expected image/jpeg, got %s", ct)
		}
	})

	t.Run("reorder images", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"commercial_name": "Sibley",
			"location_id":     locID,
			"image_ids":       []string{image2, image1},
		})
		resp, _ := manager.Put("/api/v0/images/product/reorder", bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}

		// Verify order on articles
		resp2, _ := manager.Get("/api/v0/articles")
		var articles []map[string]any
		json.NewDecoder(resp2.Body).Decode(&articles)
		resp2.Body.Close()
		for _, a := range articles {
			if a["commercial_name"] == "Sibley" {
				ids := a["image_ids"].([]any)
				if ids[0].(string) != image2 || ids[1].(string) != image1 {
					t.Errorf("expected reordered [%s, %s], got %v", image2, image1, ids)
				}
				break
			}
		}
	})

	t.Run("delete single image", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE",
			manager.BaseURL()+"/api/v0/images/product/"+image1+"?commercial_name=Sibley&location_id="+locID, nil)
		req.Header.Set("X-Dev-Role-Override", "manager-equipment")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		// image1 files gone
		if _, err := os.Stat(filepath.Join(imageDir, image1+".webp")); !os.IsNotExist(err) {
			t.Error("deleted image source should be gone")
		}
		// image2 files still exist
		if _, err := os.Stat(filepath.Join(imageDir, image2+".webp")); err != nil {
			t.Error("remaining image should still exist")
		}

		// Articles should have only image2
		resp2, _ := manager.Get("/api/v0/articles")
		var articles []map[string]any
		json.NewDecoder(resp2.Body).Decode(&articles)
		resp2.Body.Close()
		for _, a := range articles {
			if a["commercial_name"] == "Sibley" {
				ids := a["image_ids"].([]any)
				if len(ids) != 1 || ids[0].(string) != image2 {
					t.Errorf("expected [%s], got %v", image2, ids)
				}
				break
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
		if result["image_id"] == "" {
			t.Fatal("expected image_id")
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
