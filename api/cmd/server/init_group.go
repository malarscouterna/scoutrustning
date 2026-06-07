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

	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

func runInitGroup(args []string) {
	fs := flag.NewFlagSet("init-group", flag.ExitOnError)
	groupID := fs.String("group-id", "", "Scoutnet group number (required)")
	groupName := fs.String("group-name", "", "Display name for the group (only used on first run; ignored if group already exists)")
	roleKey := fs.String("role-key", "", "Scoutnet role key granting manager access, e.g. it_manager (required)")
	teamName := fs.String("team-name", "", "Name for the manager team (optional; defaults to role key)")
	seedLocations := fs.Bool("seed-locations", false, "Create default locations for Mälarscouterna")
	fs.Parse(args)

	if *groupID == "" || *roleKey == "" {
		fs.Usage()
		fmt.Fprintln(os.Stderr, "\n--group-id and --role-key are required")
		os.Exit(1)
	}

	resolvedTeamName := *teamName
	if resolvedTeamName == "" {
		resolvedTeamName = *roleKey
	}

	scope := "group"
	claimID := *roleKey

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

	// 1. Create group (idempotent; name only applied on first run)
	if *groupName != "" {
		_, err = q.CreateGroup(ctx, db.CreateGroupParams{ID: *groupID, Name: *groupName})
		if err != nil {
			existing, getErr := q.GetGroup(ctx, *groupID)
			if getErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to create/get group: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Group already exists: %s (%s) — name not changed\n", existing.ID, existing.Name)
		} else {
			fmt.Printf("Created group: %s (%s)\n", *groupID, *groupName)
		}
	} else {
		existing, getErr := q.GetGroup(ctx, *groupID)
		if getErr != nil {
			fmt.Fprintln(os.Stderr, "Group does not exist and --group-name was not provided; cannot create it")
			os.Exit(1)
		}
		fmt.Printf("Group exists: %s (%s)\n", existing.ID, existing.Name)
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
	team, err := q.GetTeamByName(ctx, db.GetTeamByNameParams{GroupID: *groupID, Name: resolvedTeamName})
	if err != nil {
		team, err = q.CreateTeam(ctx, db.CreateTeamParams{
			GroupID: *groupID, Name: resolvedTeamName, Type: "role", AccessLevel: "manager",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create team: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created team: %s (manager)\n", resolvedTeamName)
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
