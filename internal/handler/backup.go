package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/a-h/templ"

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

	dbPath := "labuh.db"
	if _, err := os.Stat(dbPath); err != nil {
		http.Error(w, "Database not found", http.StatusInternalServerError)
		return
	}

	src, err := os.Open(dbPath)
	if err != nil {
		http.Error(w, "Failed to open database", http.StatusInternalServerError)
		return
	}
	defer src.Close()

	dst, err := os.Create(backupFile)
	if err != nil {
		http.Error(w, "Failed to create backup", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(backupFile)
		http.Error(w, "Failed to copy database", http.StatusInternalServerError)
		return
	}

	if err := dst.Close(); err != nil {
		slog.Error("failed to close backup file", "error", err)
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

	backupFile := filepath.Join(h.backupPath, filename)
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
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
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

	dbPath := "labuh.db"
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
