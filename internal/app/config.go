package app

import (
	"os"
	"strconv"
)

// Config menampung semua konfigurasi runtime aplikasi get-labuh.
type Config struct {
	Port          string
	DatabaseURL   string
	BackupDir     string
	TLSEnabled    bool
	TLSCertPath   string
	TLSKeyPath    string
	ProfilingAddr string
	MasterKeyEnv  string
}

// LoadConfig memuat konfigurasi dari environment variables dengan fallback default.
func LoadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "labuh.db"
	}

	backupDir := os.Getenv("LABUH_BACKUP_DIR")
	if backupDir == "" {
		if os.Geteuid() == 0 {
			backupDir = "/var/lib/labuh/backups"
		} else {
			backupDir = "./backups"
		}
	}

	tlsEnabled, _ := strconv.ParseBool(os.Getenv("LABUH_TLS_ENABLED"))

	return Config{
		Port:          port,
		DatabaseURL:   dbURL,
		BackupDir:     backupDir,
		TLSEnabled:    tlsEnabled,
		TLSCertPath:   os.Getenv("LABUH_TLS_CERT_PATH"),
		TLSKeyPath:    os.Getenv("LABUH_TLS_KEY_PATH"),
		ProfilingAddr: os.Getenv("LABUH_PPROF_ADDR"),
		MasterKeyEnv:  "LABUH_MASTER_KEY",
	}
}
