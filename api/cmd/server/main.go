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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dbURL := getenv("DATABASE_URL", "postgres://utrustning:utrustning@localhost:5432/utrustning?sslmode=disable")
	devMode := getenv("DEV_MODE", "false") == "true"

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
		r.Use(auth.Middleware(
			getenv("JWKS_URL", ""),
			devMode,
			getenv("DEV_PERSONAS_PATH", "dev-personas.json"),
		))
		r.Use(handler.UpsertUserMiddleware(queries))

		articles := &handler.ArticleHandler{Q: queries}
		locations := &handler.LocationHandler{Q: queries}
		categories := &handler.CategoryHandler{Q: queries}

		r.Mount("/articles", articles.Routes())
		r.Mount("/locations", locations.Routes())
		r.Mount("/categories", categories.Routes())
	})

	addr := getenv("ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: r}

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
