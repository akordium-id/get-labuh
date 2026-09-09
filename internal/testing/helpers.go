package testing

import (
	"database/sql"
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/database"
)

func SetupTestDB(t *stdtesting.T) *sql.DB {
	t.Helper()

	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}
