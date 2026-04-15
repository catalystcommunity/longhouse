package cmd

import (
	"database/sql"
	"fmt"

	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/coredb"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/pressly/goose/v3"
	log "github.com/sirupsen/logrus"
)

func Migrate(flags map[string]string) error {
	config.ApplyFlags(flags)
	return RunMigrations()
}

func RunMigrations() error {
	log.Info("Running database migrations...")

	db, err := sql.Open("pgx", config.DbUri)
	if err != nil {
		return fmt.Errorf("opening database for migrations: %w", err)
	}
	defer db.Close()

	// Use advisory lock to prevent concurrent migrations
	if _, err := db.Exec("SELECT pg_advisory_lock(42)"); err != nil {
		log.WithError(err).Warn("Could not acquire advisory lock, proceeding without lock")
	} else {
		defer db.Exec("SELECT pg_advisory_unlock(42)") //nolint:errcheck
	}

	goose.SetBaseFS(coredb.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	log.Info("Migrations complete")
	return nil
}
