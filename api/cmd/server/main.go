package main

import (
	"context"
	"database/sql"
	"encoding/json"
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

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/images"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
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
	demoMode := getenv("DEMO_MODE", "false") == "true"
	// The persona-switcher mechanism (X-Dev-Role-Override auth bypass, GChat message
	// labeling) is needed whenever either flag is set - demo mode uses it too, just with
	// the OIDC lock (no auto-fallback persona) layered on top. Don't gate it on devMode
	// alone, or a deployment that sets DEMO_MODE without also remembering DEV_MODE would
	// silently lose persona switching.
	personasEnabled := devMode || demoMode
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
			DevMode:      personasEnabled,
			PersonasPath: getenv("DEV_PERSONAS_PATH", "dev-personas.json"),
			Resolver:     &handler.DBTeamResolver{Q: queries},
		}))
		r.Use(handler.UpsertUserMiddleware(queries))

		permCache := handler.NewPermissionCache(queries)

		personaIDs := buildPersonaIDs(demoMode, getenv("DEV_PERSONAS_PATH", "dev-personas.json"))
		smtpNotifier := &notifications.SMTPNotifier{Q: queries}
		gchatNotifier := &notifications.GChatNotifier{Q: queries, LabelTeam: personasEnabled}

		// In demo mode, event sends from handlers are suppressed via NoopNotifier.
		// The test-email endpoint always uses smtpNotifier directly so demo visitors
		// can verify SMTP config is working.
		var eventNotifier notifications.Notifier = smtpNotifier
		var eventGChatNotifier notifications.Notifier = gchatNotifier
		if demoMode {
			eventNotifier = notifications.NoopNotifier{}
			eventGChatNotifier = notifications.NoopNotifier{}
		}

		notifPrefsHandler := &handler.NotificationPrefsHandler{Q: queries}
		appBaseURL := getenv("APP_BASE_URL", "http://localhost:5173")
		meHandler := &handler.MeHandler{Q: queries, Perms: permCache, NotifPrefs: notifPrefsHandler, Notifier: smtpNotifier, PersonaIDs: personaIDs, DemoMode: demoMode, BaseURL: appBaseURL}

		articles := &handler.ArticleHandler{Q: queries, Perms: permCache}
		locations := &handler.LocationHandler{Q: queries}
		categories := &handler.CategoryHandler{Q: queries}

		bookings := &handler.BookingHandler{Q: queries, Perms: permCache, Notifier: eventNotifier, GChatNotifier: eventGChatNotifier, BaseURL: appBaseURL}
		teams := &handler.TeamHandler{Q: queries, DemoMode: demoMode}
		groupSettings := &handler.GroupSettingsHandler{Q: queries, Pool: pool, Perms: permCache, DemoMode: demoMode}
		issueHandler := &handler.IssueHandler{Q: queries, Perms: permCache, Notifier: eventNotifier, GChatNotifier: eventGChatNotifier, BaseURL: appBaseURL}
		imageHandler := &images.Handler{Q: queries, ImageDir: imageDir}
		userHandler := &handler.UserHandler{Q: queries, Perms: permCache, DemoMode: demoMode, PersonaIDs: personaIDs}
		logoHandler := &handler.LogoHandler{Q: queries, ImageDir: imageDir}

		r.Mount("/me", meHandler.Routes())
		r.Mount("/articles", articles.Routes())
		r.Mount("/locations", locations.Routes())
		r.Mount("/categories", categories.Routes())
		r.Mount("/bookings", bookings.Routes())
		r.Mount("/teams", teams.Routes())
		r.Mount("/group-settings", groupSettings.Routes())
		r.Mount("/group-settings/notification-defaults", notifPrefsHandler.GroupRoutes())
		r.Mount("/group-settings/force-notification-defaults", notifPrefsHandler.ForceDefaultsRoute())
		r.Mount("/group-settings/logo", logoHandler.Routes())
		r.Mount("/group-settings/logo-square", logoHandler.SquareRoutes())
		r.Mount("/issues", issueHandler.Routes())
		r.Mount("/images", imageHandler.Routes())
		r.Mount("/users", userHandler.Routes())
	})

	// Public endpoints — no authentication required.
	logoHandler := &handler.LogoHandler{Q: queries, ImageDir: imageDir}
	r.Mount("/api/v0/public/groups", logoHandler.PublicLogoRoutes())

	// Group signup — authenticated (valid ScoutID JWT) but deliberately not
	// group-scoped, since applicants by definition have no registered group
	// yet. Uses its own AllowUnmapped auth instance instead of the strict
	// one above, and skips UpsertUserMiddleware (no group_id to upsert into).
	r.Route("/api/v0/join", func(r chi.Router) {
		r.Use(auth.Middleware(auth.MiddlewareConfig{
			JWKSURL:       getenv("JWKS_URL", ""),
			DevMode:       personasEnabled,
			PersonasPath:  getenv("DEV_PERSONAS_PATH", "dev-personas.json"),
			Resolver:      &handler.DBTeamResolver{Q: queries},
			AllowUnmapped: true,
		}))
		joinHandler := &handler.JoinHandler{
			Notifier: &notifications.SMTPNotifier{Q: queries},
			AdminTo:  getenv("ADMIN_EMAIL", ""),
		}
		r.Mount("/", joinHandler.Routes())
	})

	// Daily notification scheduler (reminders + overdue alerts).
	// In demo mode, use NoopNotifier so scheduled sends never fire.
	var schedulerNotifier notifications.Notifier = &notifications.SMTPNotifier{Q: queries}
	var schedulerGChatNotifier notifications.Notifier = &notifications.GChatNotifier{Q: queries}
	if demoMode {
		schedulerNotifier = notifications.NoopNotifier{}
		schedulerGChatNotifier = notifications.NoopNotifier{}
	}
	baseURL := getenv("APP_BASE_URL", "http://localhost:5173")
	notifications.StartScheduler(queries, schedulerNotifier, baseURL)

	addr := getenv("ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: r}

	// Background: cancel expired bookings and send archive warnings.
	// Runs immediately on startup (not just after the first tick) so a deploy/restart
	// doesn't leave a booking waiting up to a full interval for its first check - this
	// matters most for the archive warning, whose 23-24h detection window can otherwise
	// be missed entirely if a tick is delayed past it. In dev mode the interval is 1
	// minute instead of 1 hour, so changes to auto-archive settings are quick to verify
	// against Mailpit without waiting. This supersedes the old separate 48h empty-draft
	// cleanup job - the draft auto-archive deadline now runs from created_at, covering
	// empty drafts too.
	runBookingCleanupJobs := func() {
		archived, err := handler.ArchiveExpiredBookings(ctx, queries)
		if err != nil {
			slog.Error("booking auto-archive failed", "error", err)
		} else if archived > 0 {
			slog.Info("auto-archived expired bookings", "archived", archived)
		}

		notifications.SendArchiveWarnings(ctx, queries, schedulerNotifier, schedulerGChatNotifier, baseURL)

		swapped, err := handler.ResolveOverdueSwaps(ctx, queries)
		if err != nil {
			slog.Error("overdue swap resolution failed", "error", err)
		} else if swapped > 0 {
			slog.Info("auto-swapped overdue booking items", "swapped", swapped)
		}
	}
	go func() {
		interval := 1 * time.Hour
		if devMode && !demoMode {
			// devMode alone is also true in demo (it just gates the persona switcher
			// behind a real login there) - the fast interval should only apply to
			// genuine local dev, not demo deployments.
			interval = 1 * time.Minute
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		runBookingCleanupJobs()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runBookingCleanupJobs()
			}
		}
	}()

	go func() {
		slog.Info("starting server", "addr", addr, "dev_mode", devMode, "demo_mode", demoMode)
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

func buildPersonaIDs(demoMode bool, personasPath string) map[string]bool {
	if !demoMode {
		return nil
	}
	data, err := os.ReadFile(personasPath)
	if err != nil {
		slog.Warn("demo mode: could not load personas", "path", personasPath, "error", err)
		return map[string]bool{}
	}
	var pf struct {
		Personas map[string]struct {
			MemberID string `json:"member_id"`
		} `json:"personas"`
	}
	if err := json.Unmarshal(data, &pf); err != nil {
		slog.Warn("demo mode: could not parse personas", "error", err)
		return map[string]bool{}
	}
	ids := make(map[string]bool, len(pf.Personas))
	for _, p := range pf.Personas {
		ids[p.MemberID] = true
	}
	return ids
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
