package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/images"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Subcommand dispatch
	if len(os.Args) > 1 && os.Args[1] == "init-group" {
		runInitGroup(os.Args[2:])
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dbURL := getenv("DATABASE_URL", "postgres://utrustning:utrustning@localhost:5432/utrustning?sslmode=disable")
	devMode := getenv("DEV_MODE", "false") == "true"
	imageDir := getenv("IMAGE_DIR", "/data/images")

	images.InitVips()
	defer images.ShutdownVips()

	// Run migrations with database/sql (goose requirement)
	if err := runMigrations(dbURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// pgxpool for application queries
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("failed to create connection pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := db.New(pool)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v0", func(r chi.Router) {
		r.Use(auth.Middleware(auth.MiddlewareConfig{
			JWKSURL:      getenv("JWKS_URL", ""),
			DevMode:      devMode,
			PersonasPath: getenv("DEV_PERSONAS_PATH", "dev-personas.json"),
			Resolver:     &handler.DBTeamResolver{Q: queries},
		}))
		r.Use(handler.UpsertUserMiddleware(queries))

		permCache := handler.NewPermissionCache(queries)

		// User info (returns resolved claims)
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				handler.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			// Get group name and permissions
			groupName := ""
			if g, err := queries.GetGroup(r.Context(), claims.GroupID); err == nil {
				groupName = g.Name
			}
			perms := permCache.Get(r, claims.GroupID)
			handler.WriteJSON(w, http.StatusOK, map[string]any{
				"member_id":  claims.MemberID,
				"group_id":   claims.GroupID,
				"group_name": groupName,
				"name":       claims.Name,
				"email":      claims.Email,
				"teams":      claims.Teams,
				"max_access": claims.MaxAccess,
				"permissions": map[string]string{
					"image_upload":  perms.ImageUpload,
					"booking":       perms.Booking,
					"article_edit":  perms.ArticleEdit,
					"issue_resolve": perms.IssueResolve,
					"manager_notes": perms.ManagerNotes,
				},
			})
		})

		articles := &handler.ArticleHandler{Q: queries, Perms: permCache}
		locations := &handler.LocationHandler{Q: queries}
		categories := &handler.CategoryHandler{Q: queries}
		bookings := &handler.BookingHandler{Q: queries}
		teams := &handler.TeamHandler{Q: queries}
		groupSettings := &handler.GroupSettingsHandler{Q: queries, Perms: permCache}
		imageHandler := &images.Handler{Q: queries, ImageDir: imageDir}

		r.Mount("/articles", articles.Routes())
		r.Mount("/locations", locations.Routes())
		r.Mount("/categories", categories.Routes())
		r.Mount("/bookings", bookings.Routes())
		r.Mount("/teams", teams.Routes())
		r.Mount("/group-settings", groupSettings.Routes())
		r.Mount("/images", imageHandler.Routes())
	})

	addr := getenv("ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: r}

	// Background: clean up empty draft bookings every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				threshold := pgtype.Timestamptz{Time: time.Now().Add(-48 * time.Hour), Valid: true}
				deleted, err := queries.CleanupEmptyDrafts(ctx, threshold)
				if err != nil {
					slog.Error("draft cleanup failed", "error", err)
				} else if deleted > 0 {
					slog.Info("cleaned up empty drafts", "deleted", deleted)
				}
			}
		}
	}()

	go func() {
		slog.Info("starting server", "addr", addr, "dev_mode", devMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func runMigrations(dbURL string) error {
	migrationsDir := getenv("MIGRATIONS_DIR", "migrations")
	goose.SetBaseFS(os.DirFS(migrationsDir))

	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	for i := range 30 {
		if err := sqlDB.Ping(); err == nil {
			break
		}
		if i == 29 {
			slog.Error("database not ready after 30 attempts")
			os.Exit(1)
		}
		time.Sleep(time.Second)
	}
	slog.Info("database connected")

	if err := goose.Up(sqlDB, "."); err != nil {
		return err
	}
	slog.Info("migrations applied")
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
