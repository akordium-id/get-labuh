package repo

import (
	"database/sql"
	"time"
)

type SettingRepo struct {
	db *sql.DB
}

func NewSettingRepo(db *sql.DB) *SettingRepo {
	return &SettingRepo{db: db}
}

func (r *SettingRepo) Get(key string) (string, error) {
	var value string
	err := r.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (r *SettingRepo) Set(key, value string) error {
	_, err := r.db.Exec(
		"INSERT INTO settings (key, value, created_at, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at",
		key, value, time.Now(), time.Now(),
	)
	return err
}

func (r *SettingRepo) InitializeDefaults() error {
	defaults := map[string]string{
		"caddy_api_url":  "http://localhost:2019",
		"caddy_api_key":  "",
		"caddy_network":  "labuh-network",
	}

	for key, value := range defaults {
		if _, err := r.Get(key); err == sql.ErrNoRows {
			if err := r.Set(key, value); err != nil {
				return err
			}
		}
	}

	return nil
}
