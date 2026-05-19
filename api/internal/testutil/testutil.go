package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
)

// shared holds the single Postgres container reused across all tests.
var shared struct {
	pool      *pgxpool.Pool
	container testcontainers.Container
}

// SetupShared starts a single Postgres container and runs migrations.
// Call from TestMain before m.Run().
func SetupShared() {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:17-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "test",
				"POSTGRES_PASSWORD": "test",
				"POSTGRES_DB":       "test",
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}
	shared.container = container

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	dbURL := fmt.Sprintf("postgres://test:test@%s:%s/test?sslmode=disable", host, port.Port())

	migrationsDir := findMigrationsDir()
	goose.SetBaseFS(os.DirFS(migrationsDir))
	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	sqlDB.Close()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	shared.pool = pool
}

// TeardownShared cleans up the shared container. Call from TestMain after m.Run().
func TeardownShared() {
	if shared.pool != nil {
		shared.pool.Close()
	}
	if shared.container != nil {
		shared.container.Terminate(context.Background())
	}
}

type TestEnv struct {
	Pool         *pgxpool.Pool
	Queries      *db.Queries
	Server       *httptest.Server
	Router       *chi.Mux
	personasPath string
}

// SetupTestEnv returns a TestEnv backed by the shared container.
// It truncates all tables to give each test a clean slate.
func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	ctx := context.Background()

	// Background goroutines from the previous test (e.g. async notification sends
	// writing to notification_log) can deadlock with TRUNCATE. Retry a few times
	// to let them finish.
	var err error
	for range 5 {
		_, err = shared.pool.Exec(ctx,
			`TRUNCATE groups, users, teams, team_claim_mappings, categories, locations, articles,
			 article_events, booking_events, bookings, booking_items, packages, package_items,
			 audit_log, group_settings, product_images, notification_log CASCADE`)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}

	// Re-insert seed data
	_, err = shared.pool.Exec(ctx, `
		INSERT INTO groups (id, name) VALUES ('766', 'Mälarscouterna'), ('999', 'Testkåren');
		INSERT INTO locations (group_id, name, sort_order) VALUES
			('766', 'Kammaren', 1), ('766', 'Östergården', 2), ('766', 'Ladan', 3),
			('766', 'Kallförrådet', 4), ('766', 'Hajkförrådet', 5), ('766', 'Magasinet', 6),
			('766', 'Verkstan', 7), ('999', 'Förrådet', 1);
		INSERT INTO categories (group_id, name, sort_order) VALUES
			('766', 'Övrigt', 1), ('999', 'Övrigt', 1);
		INSERT INTO group_settings (group_id) VALUES ('766'), ('999');

		-- Teams with access levels
		INSERT INTO teams (group_id, name, type, access_level) VALUES
			('766', 'Yggdrasil', 'troop', 'book'),
			('766', 'Spindlarna', 'troop', 'book'),
			('766', 'Flaskpostorné', 'troop', 'book'),
			('766', 'Valborgskommittén', 'role', 'trusted'),
			('766', 'Utrustningsgruppen', 'role', 'manager'),
			('766', 'IT-gruppen', 'role', 'manager'),
			('766', 'Läger', 'role', 'trusted'),
			('999', 'Avdelning 1', 'troop', 'book');
	`)
	if err != nil {
		t.Fatalf("failed to re-seed tables: %v", err)
	}
	queries := db.New(shared.pool)
	personasPath := findPersonasPath()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := httptest.NewServer(r)
	t.Cleanup(func() { srv.Close() })

	return &TestEnv{
		Pool:         shared.pool,
		Queries:      queries,
		Server:       srv,
		Router:       r,
		personasPath: personasPath,
	}
}

// MountV1 mounts an authenticated subrouter at /api/v0 with auth + user upsert middleware.
func (e *TestEnv) MountV1(fn func(r chi.Router)) {
	e.Router.Route("/api/v0", func(r chi.Router) {
		r.Use(auth.Middleware(auth.MiddlewareConfig{
			DevMode:      true,
			PersonasPath: e.personasPath,
			Resolver:     &handler.DBTeamResolver{Q: e.Queries},
		}))
		r.Use(handler.UpsertUserMiddleware(e.Queries))
		fn(r)
	})
}

// V1 is a convenience alias for MountV1.
func (e *TestEnv) V1(fn func(r chi.Router)) {
	e.MountV1(fn)
}

// ClientAs returns an HTTP client that sends requests as the given dev persona.
func (e *TestEnv) ClientAs(persona string) *TestClient {
	return &TestClient{
		baseURL: e.Server.URL,
		persona: persona,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// ClientWithClaims returns an HTTP client that injects arbitrary claims via
// a JSON-encoded X-Dev-Claims header. Use this for edge cases that don't
// warrant a permanent persona (e.g. unknown group, no roles, malformed claims).
func (e *TestEnv) ClientWithClaims(claims auth.Claims) *TestClient {
	data, _ := json.Marshal(claims)
	return &TestClient{
		baseURL:   e.Server.URL,
		rawClaims: string(data),
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

type TestClient struct {
	baseURL   string
	persona   string
	rawClaims string
	client    *http.Client
}

func (c *TestClient) Do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if c.rawClaims != "" {
		req.Header.Set("X-Dev-Claims", c.rawClaims)
	} else {
		req.Header.Set("X-Dev-Role-Override", c.persona)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

func (c *TestClient) Get(path string) (*http.Response, error) {
	return c.Do("GET", path, nil)
}

func (c *TestClient) Post(path string, body io.Reader) (*http.Response, error) {
	return c.Do("POST", path, body)
}

func (c *TestClient) Put(path string, body io.Reader) (*http.Response, error) {
	return c.Do("PUT", path, body)
}

func (c *TestClient) Delete(path string) (*http.Response, error) {
	return c.Do("DELETE", path, nil)
}

func (c *TestClient) BaseURL() string {
	return c.baseURL
}

func findMigrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

func findPersonasPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "dev-personas.json")
}
