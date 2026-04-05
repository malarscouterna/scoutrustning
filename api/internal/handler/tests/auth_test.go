package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestAuth_DevPersonaOverride(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	env.V1(func(r chi.Router) {
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			claims, _ := auth.ClaimsFromContext(r.Context())
			handler.WriteJSON(w, http.StatusOK, claims)
		})
	})

	t.Run("leader persona returns correct claims", func(t *testing.T) {
		client := env.ClientAs("leader-yggdrasil")
		resp, err := client.Get("/api/v0/me")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var claims auth.Claims
		json.NewDecoder(resp.Body).Decode(&claims)

		if claims.MemberID != "3000005" {
			t.Errorf("expected member_id 3000005, got %s", claims.MemberID)
		}
		if claims.GroupID != "766" {
			t.Errorf("expected group_id 766, got %s", claims.GroupID)
		}
		if claims.Name != "Hanna Yggdrasil" {
			t.Errorf("expected name Hanna Yggdrasil, got %s", claims.Name)
		}
		if !claims.HasRole("leader") {
			t.Error("expected leader role")
		}
	})

	t.Run("unknown persona returns 400", func(t *testing.T) {
		client := env.ClientAs("nonexistent")
		resp, err := client.Get("/api/v0/me")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("no auth header returns 401", func(t *testing.T) {
		resp, err := http.Get(env.Server.URL + "/api/v0/me")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("health endpoint needs no auth", func(t *testing.T) {
		resp, err := http.Get(env.Server.URL + "/api/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestAuth_RoleEnforcement(t *testing.T) {
	env := testutil.SetupTestEnv(t)

	env.V1(func(r chi.Router) {
		r.With(auth.RequireRole("equipment_manager")).Get("/admin-only", func(w http.ResponseWriter, r *http.Request) {
			handler.WriteJSON(w, http.StatusOK, map[string]string{"ok": "true"})
		})
	})

	t.Run("manager can access admin endpoint", func(t *testing.T) {
		client := env.ClientAs("manager-equipment")
		resp, err := client.Get("/api/v0/admin-only")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("leader gets 403 on admin endpoint", func(t *testing.T) {
		client := env.ClientAs("leader-yggdrasil")
		resp, err := client.Get("/api/v0/admin-only")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})
}
