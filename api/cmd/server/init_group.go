package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

func runInitGroup(args []string) {
	fs := flag.NewFlagSet("init-group", flag.ExitOnError)
	orgType := fs.String("org-type", auth.OrgTypeGroup, "Scoutnet organisation type: group, district, region, organisation, corps, network or project")
	orgID := fs.String("org-id", "", "Scoutnet organisation ID (required; for groups this is the group number)")
	groupID := fs.String("group-id", "", "Alias for --org-id, kept for backwards compatibility")
	groupName := fs.String("group-name", "", "Display name for the organisation (only used on first run; ignored if it already exists)")
	roleKey := fs.String("role-key", "", "Scoutnet role key granting manager access, e.g. it_manager (required)")
	teamName := fs.String("team-name", "", "Name for the manager team (optional; defaults to role key)")
	seedLocations := fs.Bool("seed-locations", false, "Create default locations for Mälarscouterna")
	fs.Parse(args)

	scoutnetID := *orgID
	if scoutnetID == "" {
		scoutnetID = *groupID
	}
	if scoutnetID == "" || *roleKey == "" {
		fs.Usage()
		fmt.Fprintln(os.Stderr, "\n--org-id (or --group-id) and --role-key are required")
		os.Exit(1)
	}
	if !auth.IsOrgType(*orgType) {
		fmt.Fprintf(os.Stderr, "Unknown --org-type %q. Valid types: %s\n", *orgType, strings.Join(auth.OrgTypes, ", "))
		os.Exit(1)
	}

	resolvedTeamName := *teamName
	if resolvedTeamName == "" {
		resolvedTeamName = *roleKey
	}
	tenantID := auth.TenantKey(*orgType, scoutnetID)

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

	// 1. Create organisation (idempotent; name only applied on first run)
	if *groupName != "" {
		_, err = q.CreateGroup(ctx, db.CreateGroupParams{ID: tenantID, Name: *groupName, OrgType: *orgType, ScoutnetID: scoutnetID})
		if err != nil {
			existing, getErr := q.GetGroup(ctx, tenantID)
			if getErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to create/get group: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Organisation already exists: %s (%s, %s) — name not changed\n", existing.ID, existing.Name, existing.OrgType)
		} else {
			fmt.Printf("Created organisation: %s (%s, %s)\n", tenantID, *groupName, *orgType)
		}
	} else {
		existing, getErr := q.GetGroup(ctx, tenantID)
		if getErr != nil {
			fmt.Fprintln(os.Stderr, "Organisation does not exist and --group-name was not provided; cannot create it")
			os.Exit(1)
		}
		fmt.Printf("Organisation exists: %s (%s, %s)\n", existing.ID, existing.Name, existing.OrgType)
	}

	// 2. Create group_settings with defaults (idempotent)
	_, err = q.CreateGroupSettingsDefaults(ctx, tenantID)
	if err != nil {
		// Already exists
		fmt.Println("Group settings already exist")
	} else {
		fmt.Println("Created group settings with defaults")
	}

	// 3. Create default category "Övrigt" (idempotent — check first)
	cats, _ := q.ListCategories(ctx, tenantID)
	if len(cats) == 0 {
		_, err = q.CreateCategory(ctx, db.CreateCategoryParams{
			GroupID: tenantID, Name: "Övrigt", SortOrder: 1,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create default category: %v\n", err)
		} else {
			fmt.Println("Created default category: Övrigt")
		}
	}

	// 4. Create manager team (idempotent — check by name)
	team, err := q.GetTeamByName(ctx, db.GetTeamByNameParams{GroupID: tenantID, Name: resolvedTeamName})
	if err != nil {
		team, err = q.CreateTeam(ctx, db.CreateTeamParams{
			GroupID: tenantID, Name: resolvedTeamName, Type: "role", AccessLevel: "manager",
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
		GroupID: tenantID, TeamID: team.ID, ClaimScope: scope, ClaimID: claimID,
	})
	if err != nil || mapping.ID.Bytes == [16]byte{} {
		fmt.Printf("Claim mapping already exists: %s:%s → %s\n", scope, claimID, *teamName)
	} else {
		fmt.Printf("Created claim mapping: %s:%s → %s\n", scope, claimID, *teamName)
	}

	// 6. Optionally seed locations
	if *seedLocations {
		locs, _ := q.ListLocations(ctx, tenantID)
		if len(locs) == 0 {
			for i, name := range []string{"Kammaren", "Östergården", "Ladan", "Kallförrådet", "Hajkförrådet", "Magasinet", "Verkstan"} {
				q.CreateLocation(ctx, db.CreateLocationParams{
					GroupID: tenantID, Name: name, SortOrder: int32(i + 1),
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
