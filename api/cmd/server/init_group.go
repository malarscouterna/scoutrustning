package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

func runInitGroup(args []string) {
	fs := flag.NewFlagSet("init-group", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Keycloak org ID (required)")
	groupName := fs.String("group-name", "", "Display name for the group (required)")
	managerClaim := fs.String("manager-claim", "", "OIDC claim for the manager team, e.g. group:766:it_manager (required)")
	teamName := fs.String("team-name", "", "Name for the manager team (required)")
	seedLocations := fs.Bool("seed-locations", false, "Create default locations for Mälarscouterna")
	fs.Parse(args)

	if *groupID == "" || *groupName == "" || *managerClaim == "" || *teamName == "" {
		fs.Usage()
		fmt.Fprintln(os.Stderr, "\nAll of --group-id, --group-name, --manager-claim, --team-name are required")
		os.Exit(1)
	}

	// Parse claim: "group:766:it_manager" → scope="group", id="766"
	scope, claimID, err := parseClaim(*managerClaim)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid --manager-claim: %v\n", err)
		os.Exit(1)
	}

	dbURL := getenv("DATABASE_URL", "postgres://utrustning:utrustning@localhost:5432/utrustning?sslmode=disable")

	// Run migrations first
	if err := runMigrationsForInit(dbURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	q := db.New(pool)

	// 1. Create group (idempotent)
	_, err = q.CreateGroup(ctx, db.CreateGroupParams{ID: *groupID, Name: *groupName})
	if err != nil {
		// CreateGroup uses ON CONFLICT DO NOTHING, so no row returned means it exists
		existing, getErr := q.GetGroup(ctx, *groupID)
		if getErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to create/get group: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Group already exists: %s (%s)\n", existing.ID, existing.Name)
	} else {
		fmt.Printf("Created group: %s (%s)\n", *groupID, *groupName)
	}

	// 2. Create group_settings with defaults (idempotent)
	_, err = q.CreateGroupSettingsDefaults(ctx, *groupID)
	if err != nil {
		// Already exists
		fmt.Println("Group settings already exist")
	} else {
		fmt.Println("Created group settings with defaults")
	}

	// 3. Create default category "Övrigt" (idempotent — check first)
	cats, _ := q.ListCategories(ctx, *groupID)
	if len(cats) == 0 {
		_, err = q.CreateCategory(ctx, db.CreateCategoryParams{
			GroupID: *groupID, Name: "Övrigt", SortOrder: 1,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create default category: %v\n", err)
		} else {
			fmt.Println("Created default category: Övrigt")
		}
	}

	// 4. Create manager team (idempotent — check by name)
	team, err := q.GetTeamByName(ctx, db.GetTeamByNameParams{GroupID: *groupID, Name: *teamName})
	if err != nil {
		team, err = q.CreateTeam(ctx, db.CreateTeamParams{
			GroupID: *groupID, Name: *teamName, Type: "role", AccessLevel: "manager",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create team: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created team: %s (manager)\n", *teamName)
	} else {
		fmt.Printf("Team already exists: %s (access_level=%s)\n", team.Name, team.AccessLevel)
	}

	// 5. Create claim mapping (idempotent via ON CONFLICT DO NOTHING)
	mapping, err := q.CreateTeamClaimMapping(ctx, db.CreateTeamClaimMappingParams{
		GroupID: *groupID, TeamID: team.ID, ClaimScope: scope, ClaimID: claimID,
	})
	if err != nil || mapping.ID.Bytes == [16]byte{} {
		fmt.Printf("Claim mapping already exists: %s:%s → %s\n", scope, claimID, *teamName)
	} else {
		fmt.Printf("Created claim mapping: %s:%s → %s\n", scope, claimID, *teamName)
	}

	// 6. Optionally seed locations
	if *seedLocations {
		locs, _ := q.ListLocations(ctx, *groupID)
		if len(locs) == 0 {
			for i, name := range []string{"Kammaren", "Östergården", "Ladan", "Kallförrådet", "Hajkförrådet", "Magasinet", "Verkstan"} {
				q.CreateLocation(ctx, db.CreateLocationParams{
					GroupID: *groupID, Name: name, SortOrder: int32(i + 1),
				})
			}
			fmt.Println("Created default locations")
		} else {
			fmt.Println("Locations already exist, skipping")
		}
	}

	fmt.Println("Done.")
}

func parseClaim(claim string) (scope, claimID string, err error) {
	// "group:766:it_manager" → scope="group", claimID="it_manager"
	// "troop:17443:leader"   → scope="troop", claimID="17443"
	// For group claims, the claim_id is the role name (identifies the team within the group).
	// For troop claims, the claim_id is the troop ID (identifies which troop).
	parts := splitN(claim, ":", 3)
	if len(parts) < 3 {
		return "", "", fmt.Errorf("expected format scope:id:role, got %q", claim)
	}
	scope = parts[0]
	if scope != "group" && scope != "troop" {
		return "", "", fmt.Errorf("scope must be 'group' or 'troop', got %q", scope)
	}
	switch scope {
	case "group":
		claimID = parts[2] // role name
	case "troop":
		claimID = parts[1] // troop ID
	}
	if claimID == "" {
		return "", "", fmt.Errorf("claim ID is empty")
	}
	return scope, claimID, nil
}

func splitN(s, sep string, n int) []string {
	var result []string
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	result = append(result, s)
	return result
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func runMigrationsForInit(dbURL string) error {
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
			return fmt.Errorf("database not ready after 30 attempts")
		}
		time.Sleep(time.Second)
	}

	return goose.Up(sqlDB, ".")
}
