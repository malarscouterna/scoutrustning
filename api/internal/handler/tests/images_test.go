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
		r.Mount("/group-settings", (&handler.GroupSettingsHandler{Q: env.Queries}).Routes())
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

	uploadProduct := func(t *testing.T, persona string) (*http.Response, error) {
		t.Helper()
		body, contentType := buildMultipartUpload(t, testJPEG, "Sibley", locID)
		req, _ := http.NewRequest("POST", manager.BaseURL()+"/api/v0/images/product", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Dev-Role-Override", persona)
		return http.DefaultClient.Do(req)
	}

	// Helper to extract image ID from new response format
	extractImageID := func(t *testing.T, resp *http.Response) (string, []any) {
		t.Helper()
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		img := result["image"].(map[string]any)
		return img["id"].(string), result["image_ids"].([]any)
	}

	t.Run("leader can upload by default (image_upload_role=leader)", func(t *testing.T) {
		resp, err := uploadProduct(t, "leader-yggdrasil")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		// Clean up: delete the image so it doesn't interfere with later tests
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		img := result["image"].(map[string]any)
		imgID := img["id"].(string)
		req, _ := http.NewRequest("DELETE",
			manager.BaseURL()+"/api/v0/images/product/"+imgID+"?commercial_name=Sibley&location_id="+locID, nil)
		req.Header.Set("X-Dev-Role-Override", "manager-equipment")
		http.DefaultClient.Do(req)
	})

	t.Run("restrict upload to manager only", func(t *testing.T) {
		// Set image_upload_role to equipment_manager
		settingsBody, _ := json.Marshal(map[string]any{
			"image_upload_role":      "equipment_manager",
			"default_approval_level": "none",
		})
		resp, _ := manager.Put("/api/v0/group-settings", bytes.NewReader(settingsBody))
		resp.Body.Close()

		resp, err := uploadProduct(t, "leader-yggdrasil")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}

		// Reset to default
		settingsBody, _ = json.Marshal(map[string]any{
			"image_upload_role":      "leader",
			"default_approval_level": "none",
		})
		resp, _ = manager.Put("/api/v0/group-settings", bytes.NewReader(settingsBody))
		resp.Body.Close()
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
		var ids []any
		image1, ids = extractImageID(t, resp)
		if len(ids) != 1 || ids[0].(string) != image1 {
			t.Errorf("expected [%s], got %v", image1, ids)
		}

		// Check files exist (using file_id from product_images, which for fresh uploads equals the row id's file_id)
		// The files are named by file_id, which we can get from the image metadata
		metaResp, _ := manager.Get("/api/v0/images/product/" + image1)
		var meta map[string]any
		json.NewDecoder(metaResp.Body).Decode(&meta)
		metaResp.Body.Close()
		fileID := meta["file_id"].(string)

		if _, err := os.Stat(filepath.Join(imageDir, fileID+".webp")); err != nil {
			t.Errorf("source file not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(imageDir, fileID+"_thumb.webp")); err != nil {
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
		var ids []any
		image2, ids = extractImageID(t, resp)
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
		// Get file_id for serving
		metaResp, _ := manager.Get("/api/v0/images/product/" + image1)
		var meta map[string]any
		json.NewDecoder(metaResp.Body).Decode(&meta)
		metaResp.Body.Close()
		fileID := meta["file_id"].(string)

		resp, _ := leader.Get("/api/v0/images/" + fileID + ".webp")
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
		metaResp, _ := manager.Get("/api/v0/images/product/" + image1)
		var meta map[string]any
		json.NewDecoder(metaResp.Body).Decode(&meta)
		metaResp.Body.Close()
		fileID := meta["file_id"].(string)

		resp, _ := leader.Get("/api/v0/images/" + fileID + ".webp?format=jpeg")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("expected image/jpeg, got %s", ct)
		}
	})

	t.Run("list product images metadata", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/images/product?commercial_name=Sibley&location_id=" + locID)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var imgs []map[string]any
		json.NewDecoder(resp.Body).Decode(&imgs)
		if len(imgs) != 2 {
			t.Fatalf("expected 2 images, got %d", len(imgs))
		}
		if imgs[0]["id"].(string) != image1 {
			t.Errorf("expected first image %s, got %s", image1, imgs[0]["id"])
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

	t.Run("upload with metadata", func(t *testing.T) {
		body, contentType := buildMultipartUploadWithMeta(t, testJPEG, "Sibley", locID, map[string]string{
			"title":        "Sibley framifrån",
			"description":  "Tältet i solljus",
			"format":       "landscape",
			"shared":       "true",
			"attribution":  "Test Manager, Mälarscouterna",
		})
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
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		img := result["image"].(map[string]any)
		if img["title"] != "Sibley framifrån" {
			t.Errorf("expected title 'Sibley framifrån', got %v", img["title"])
		}
		if img["format"] != "landscape" {
			t.Errorf("expected format 'landscape', got %v", img["format"])
		}
		if img["shared"] != true {
			t.Errorf("expected shared=true, got %v", img["shared"])
		}
	})

	t.Run("browse shared images", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/images/shared")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var imgs []map[string]any
		json.NewDecoder(resp.Body).Decode(&imgs)
		if len(imgs) == 0 {
			t.Fatal("expected at least one shared image")
		}
		// Should include the shared image we just uploaded
		found := false
		for _, img := range imgs {
			if img["title"] == "Sibley framifrån" {
				found = true
				if img["attribution"] == "" {
					t.Error("expected attribution to be set")
				}
			}
		}
		if !found {
			t.Error("shared image not found in browse results")
		}
	})

	t.Run("browse shared with search", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/images/shared?search=framifrån")
		defer resp.Body.Close()
		var imgs []map[string]any
		json.NewDecoder(resp.Body).Decode(&imgs)
		found := false
		for _, img := range imgs {
			if img["title"] == "Sibley framifrån" {
				found = true
			}
		}
		if !found {
			t.Error("search should find the shared image by title")
		}
	})

	t.Run("leader cannot delete another users image", func(t *testing.T) {
		// image2 was uploaded by manager — leader should not be able to delete it
		req, _ := http.NewRequest("DELETE",
			leader.BaseURL()+"/api/v0/images/product/"+image2+"?commercial_name=Sibley&location_id="+locID, nil)
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

	t.Run("upload portrait format", func(t *testing.T) {
		body, contentType := buildMultipartUploadWithMeta(t, testJPEG, "Sibley", locID, map[string]string{
			"format": "portrait",
		})
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
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		img := result["image"].(map[string]any)
		if img["format"] != "portrait" {
			t.Errorf("expected format 'portrait', got %v", img["format"])
		}
	})

	t.Run("upload square format", func(t *testing.T) {
		body, contentType := buildMultipartUploadWithMeta(t, testJPEG, "Sibley", locID, map[string]string{
			"format": "square",
		})
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
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		img := result["image"].(map[string]any)
		if img["format"] != "square" {
			t.Errorf("expected format 'square', got %v", img["format"])
		}
	})

	t.Run("invalid format rejected", func(t *testing.T) {
		body, contentType := buildMultipartUploadWithMeta(t, testJPEG, "Sibley", locID, map[string]string{
			"format": "panorama",
		})
		req, _ := http.NewRequest("POST", manager.BaseURL()+"/api/v0/images/product", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Dev-Role-Override", "manager-equipment")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
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

func buildMultipartUploadWithMeta(t *testing.T, jpegData []byte, commercialName, locationID string, meta map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "test.jpg")
	fw.Write(jpegData)
	w.WriteField("commercial_name", commercialName)
	w.WriteField("location_id", locationID)
	for k, v := range meta {
		w.WriteField(k, v)
	}
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
