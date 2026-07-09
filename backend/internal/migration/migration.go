package migration

import (
	"errors"
	"log"

	"github.com/golang-migrate/migrate/v4"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Run(databaseURL string) error {
	if databaseURL == "" {
		log.Println("DATABASE_URL is empty. Skipping migrations.")
		return nil
	}

	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		return err
	}
	defer m.Close()

	err = m.Up()

	if errors.Is(err, migrate.ErrNoChange) {
		log.Println("Database migrations: no change.")
		return nil
	}

	if err != nil {
		return err
	}

	log.Println("Database migrations applied successfully.")

	return nil
}
