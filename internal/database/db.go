package database

import (
	"database/sql"
	"embed"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var DB *sql.DB

func Connect(dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			dsn = "labuh.db"
		}
	}

	if !strings.Contains(dsn, "_loc=") {
		if strings.Contains(dsn, "?") {
			dsn += "&_loc=UTC"
		} else {
			dsn += "?_loc=UTC"
		}
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Configure SQLite pragmas for concurrency and safety (skip for in-memory unit tests)
	if !strings.Contains(dsn, ":memory:") {
		_, _ = db.Exec("PRAGMA foreign_keys = ON;")
		_, _ = db.Exec("PRAGMA journal_mode = WAL;")
		_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
	}

	DB = db
	return db, nil
}

func RunMigrations(db *sql.DB, migrationPath string) error {
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(content))
	return err
}

func RunAllMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}

	var filenames []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			filenames = append(filenames, entry.Name())
		}
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		var exists int
		err := db.QueryRow("SELECT COUNT(1) FROM schema_migrations WHERE version = ?", filename).Scan(&exists)
		if err != nil {
			return err
		}
		if exists > 0 {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return err
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", filename); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
