package handler

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type BackupHandler struct {
	settingRepo *repo.SettingRepo
	backupPath  string
}

func NewBackupHandler(settingRepo *repo.SettingRepo, backupPath string) *BackupHandler {
	if backupPath == "" {
		backupPath = "/var/lib/labuh/backups"
	}

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		slog.Error("failed to create backup directory", "error", err)
	}

	return &BackupHandler{
		settingRepo: settingRepo,
		backupPath:  backupPath,
	}
}

func (h *BackupHandler) BackupPage(w http.ResponseWriter, r *http.Request) {
	templ.Handler(layouts.AppLayout(settings.BackupPage())).ServeHTTP(w, r)
}

func (h *BackupHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("labuh-backup-%s.db", timestamp)
	backupFile := filepath.Join(h.backupPath, filename)

	if database.DB != nil {
		// SQLite online atomic backup using VACUUM INTO
		_, err := database.DB.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupFile))
		if err != nil {
			slog.Error("failed to create atomic backup via vacuum", "error", err)
			http.Error(w, "Failed to create backup", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	if err := rotateBackups(h.backupPath, 5); err != nil {
		slog.Error("failed to rotate backups", "error", err)
	}

	w.Header().Set("HX-Redirect", "/settings/backup")
	w.WriteHeader(http.StatusOK)
}

func (h *BackupHandler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Strictly prevent path traversal: must be pure filename with .db extension
	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName != filename || !strings.HasSuffix(cleanName, ".db") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	backupFile := filepath.Join(h.backupPath, cleanName)
	rel, err := filepath.Rel(h.backupPath, backupFile)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, err := os.Stat(backupFile); err != nil {
		http.Error(w, "Backup not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(backupFile)
	if err != nil {
		http.Error(w, "Failed to open backup", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", cleanName))
	io.Copy(w, f)
}

func (h *BackupHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("backup_file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read backup file", http.StatusInternalServerError)
		return
	}

	// Validate SQLite file header magic bytes: "SQLite format 3\x00"
	if len(data) < 16 || !bytes.HasPrefix(data, []byte("SQLite format 3\x00")) {
		http.Error(w, "Invalid database backup file", http.StatusBadRequest)
		return
	}

	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "labuh.db"
	}

	if err := os.Rename(dbPath, dbPath+".bak"); err != nil {
		slog.Error("failed to backup current database", "error", err)
	}

	if err := os.WriteFile(dbPath, data, 0644); err != nil {
		os.Rename(dbPath+".bak", dbPath)
		http.Error(w, "Failed to restore backup", http.StatusInternalServerError)
		return
	}

	os.Remove(dbPath + ".bak")

	w.Header().Set("HX-Redirect", "/settings/backup")
	w.WriteHeader(http.StatusOK)
}

func rotateBackups(backupPath string, keepCount int) error {
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return err
	}

	var backups []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".db" {
			backups = append(backups, entry)
		}
	}

	if len(backups) <= keepCount {
		return nil
	}

	for i := 0; i < len(backups)-keepCount; i++ {
		path := filepath.Join(backupPath, backups[i].Name())
		if err := os.Remove(path); err != nil {
			slog.Error("failed to remove old backup", "error", err, "path", path)
		}
	}

	return nil
}
