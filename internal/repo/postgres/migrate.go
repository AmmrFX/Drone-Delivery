package postgres

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"

	"drone-delivery/internal/repo/postgres/migrations"
)

func RunMigrationsUp(db *sqlx.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func RunMigrationsDown(db *sqlx.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}
	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func newMigrate(db *sqlx.DB) (*migrate.Migrate, error) {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}

	driver, err := migratepg.WithInstance(db.DB, &migratepg.Config{})
	if err != nil {
		return nil, fmt.Errorf("migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("migration instance: %w", err)
	}

	return m, nil
}
