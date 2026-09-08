package database

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
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

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
