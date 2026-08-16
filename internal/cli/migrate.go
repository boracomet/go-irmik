package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/boracomet/go-irmik/irmik/db"
	"github.com/boracomet/go-irmik/irmik/migrate"
)

func cmdMigrate() *cobra.Command {
	c := &cobra.Command{
		Use:   "migrate",
		Short: "Database migrations (up, down, status, create)",
	}
	c.AddCommand(
		cmdMigrateUp(),
		cmdMigrateDown(),
		cmdMigrateStatus(),
		cmdMigrateCreate(),
	)
	return c
}

func cmdMigrateUp() *cobra.Command {
	var steps int
	c := &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations (or --steps N)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, closer, err := openMigrator(cmd)
			if err != nil {
				return err
			}
			defer closer()

			var runErr error
			if steps != 0 {
				runErr = m.Steps(steps)
			} else {
				runErr = m.Up()
			}
			if errors.Is(runErr, migrate.ErrNoChange) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "migrate: already up to date")
				return nil
			}
			if runErr != nil {
				return runErr
			}
			st, err := m.Status()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "migrate: up to version %d\n", st.Version)
			return nil
		},
	}
	c.Flags().IntVar(&steps, "steps", 0, "apply N migrations (positive=up); 0 means all")
	return c
}

func cmdMigrateDown() *cobra.Command {
	var steps int
	c := &cobra.Command{
		Use:   "down",
		Short: "Roll back migrations (default: all; or --steps N)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, closer, err := openMigrator(cmd)
			if err != nil {
				return err
			}
			defer closer()

			var runErr error
			if steps > 0 {
				runErr = m.Steps(-steps)
			} else if steps < 0 {
				runErr = m.Steps(steps)
			} else {
				runErr = m.Down()
			}
			if errors.Is(runErr, migrate.ErrNoChange) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "migrate: nothing to roll back")
				return nil
			}
			if runErr != nil {
				return runErr
			}
			st, err := m.Status()
			if err != nil {
				return err
			}
			if st.Empty {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "migrate: down to empty schema")
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "migrate: down to version %d\n", st.Version)
			return nil
		},
	}
	c.Flags().IntVar(&steps, "steps", 0, "roll back N migrations; 0 means all")
	return c
}

func cmdMigrateStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current migration version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, closer, err := openMigrator(cmd)
			if err != nil {
				return err
			}
			defer closer()
			st, err := m.Status()
			if err != nil {
				return err
			}
			if st.Empty {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "migrate: no version (empty)")
				return nil
			}
			dirty := "clean"
			if st.Dirty {
				dirty = "dirty"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "migrate: version %d (%s)\n", st.Version, dirty)
			return nil
		},
	}
}

func cmdMigrateCreate() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create empty up/down SQL migration files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCfg(cmd)
			if err != nil {
				return err
			}
			dir := cfg.Database.MigratePath
			if dir == "" {
				dir = "migrations"
			}
			up, down, err := migrate.Create(dir, args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created %s\ncreated %s\n", up, down)
			return nil
		},
	}
}

func openMigrator(cmd *cobra.Command) (*migrate.Migrator, func(), error) {
	cfg, err := loadCfg(cmd)
	if err != nil {
		return nil, nil, err
	}
	if cfg.Database.DSNOrURL() == "" {
		return nil, nil, fmt.Errorf("database DSN/URL not configured (set database.dsn, DATABASE_URL, or IRMIK_DB_DSN)")
	}
	driver := cfg.Database.DriverName()
	if driver == "" {
		return nil, nil, fmt.Errorf("database driver not configured (set database.driver or IRMIK_DB_DRIVER)")
	}
	path := cfg.Database.MigratePath
	if path == "" {
		path = "migrations"
	}

	database, err := db.OpenFromConfig(cfg.Database)
	if err != nil {
		return nil, nil, err
	}
	m, err := migrate.Open(migrate.Options{
		Driver: database.Driver(),
		DB:     database.DB(),
		Path:   path,
	})
	if err != nil {
		_ = database.Close()
		return nil, nil, err
	}
	// migrate.Close closes the *sql.DB.
	closer := func() { _ = m.Close() }
	return m, closer, nil
}
