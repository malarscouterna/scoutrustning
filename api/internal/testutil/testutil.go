package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"io"
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

type TestEnv struct {
	Pool         *pgxpool.Pool
	Queries      *db.Queries
	Server       *httptest.Server
	Router       *chi.Mux
	personasPath string
}

func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	ctx := context.Background()

	// Start Postgres container
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
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() { container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	dbURL := fmt.Sprintf("postgres://test:test@%s:%s/test?sslmode=disable", host, port.Port())

	// Run migrations
	migrationsDir := findMigrationsDir()
	goose.SetBaseFS(os.DirFS(migrationsDir))
	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	sqlDB.Close()

	// Create pgxpool
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	queries := db.New(pool)

	// Build router with dev mode auth
	personasPath := findPersonasPath()
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Tests mount their own routes on the returned router.
	// The V1 method provides a subrouter with auth + upsert middleware.

	srv := httptest.NewServer(r)
	t.Cleanup(func() { srv.Close() })

	return &TestEnv{
		Pool:         pool,
		Queries:      queries,
		Server:       srv,
		Router:       r,
		personasPath: personasPath,
	}
}

// MountV1 mounts an authenticated subrouter at /api/v0 with auth + user upsert middleware.
// The provided fn receives the subrouter to register routes on.
func (e *TestEnv) MountV1(fn func(r chi.Router)) {
	e.Router.Route("/api/v0", func(r chi.Router) {
		r.Use(auth.Middleware("", true, e.personasPath))
		r.Use(handler.UpsertUserMiddleware(e.Queries))
		fn(r)
	})
}

// V1 is a convenience that mounts the standard handlers (articles, locations, categories).
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

type TestClient struct {
	baseURL string
	persona string
	client  *http.Client
}

func (c *TestClient) Do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Dev-Role-Override", c.persona)
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

func findMigrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "migrations")
}

func findPersonasPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "dev-personas.json")
}
