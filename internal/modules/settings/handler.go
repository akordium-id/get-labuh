package settings

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/backup"
	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	pagesettings "github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type ToastType string

const (
	ToastSuccess ToastType = "success"
	ToastInfo    ToastType = "info"
	ToastError   ToastType = "error"
)

func SetToast(w http.ResponseWriter, message string, toastType ToastType) {
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast":{"type":"%s","message":"%s"}}`, toastType, message))
}

type Handler struct {
	settingRepo *SettingRepo
	auditRepo   *AuditLogRepo
	storageRepo *StorageDestinationRepo
	backupPath  string
}

func NewHandler(
	settingRepo *SettingRepo,
	auditRepo *AuditLogRepo,
	storageRepo *StorageDestinationRepo,
	backupPath string,
) *Handler {
	if backupPath == "" {
		backupPath = "./backups"
	}
	_ = os.MkdirAll(backupPath, 0755)

	return &Handler{
		settingRepo: settingRepo,
		auditRepo:   auditRepo,
		storageRepo: storageRepo,
		backupPath:  backupPath,
	}
}

// Caddy & Mode handlers
func (h *Handler) Caddy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	caddyAPIURL, _ := h.settingRepo.Get(ctx, "caddy_api_url")
	caddyAPIKey, _ := h.settingRepo.Get(ctx, "caddy_api_key")
	caddyNetwork, _ := h.settingRepo.Get(ctx, "caddy_network")

	templ.Handler(layouts.AppLayout(pagesettings.CaddySettingsPage(caddyAPIURL, caddyAPIKey, caddyNetwork))).ServeHTTP(w, r)
}

func (h *Handler) UpdateCaddy(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	caddyAPIURL := r.FormValue("caddy_api_url")
	caddyAPIKey := r.FormValue("caddy_api_key")
	caddyNetwork := r.FormValue("caddy_network")

	if caddyAPIURL == "" {
		caddyAPIURL = "http://localhost:2019"
	}

	_ = h.settingRepo.Set(ctx, "caddy_api_url", caddyAPIURL)
	_ = h.settingRepo.Set(ctx, "caddy_api_key", caddyAPIKey)
	_ = h.settingRepo.Set(ctx, "caddy_network", caddyNetwork)

	SetToast(w, "Pengaturan Caddy berhasil disimpan", ToastSuccess)
	w.Header().Set("HX-Redirect", "/settings/caddy")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) TestCaddy(w http.ResponseWriter, r *http.Request) {
	caddyAPIURL, _ := h.settingRepo.Get(r.Context(), "caddy_api_url")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","caddy_url":"` + caddyAPIURL + `"}`))
}

func (h *Handler) SwitchMode(w http.ResponseWriter, r *http.Request) {
	mode := r.FormValue("mode")
	if mode != "pro" {
		mode = "lite"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "labuh_mode",
		Value:    mode,
		Path:     "/",
		MaxAge:   365 * 24 * 3600,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})

	label := "Mode Lite diaktifkan (Vibecoder Friendly)"
	if mode == "pro" {
		label = "Mode Pro diaktifkan (Advanced DevOps)"
	}
	SetToast(w, label, ToastInfo)

	// Pastikan client mereload halaman agar mode Pro/Lite aktif secara menyeluruh (sidebar, topbar, dan view)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// Audit Logs handlers
func (h *Handler) AuditLogsPage(w http.ResponseWriter, r *http.Request) {
	logs, err := h.auditRepo.GetAll(r.Context(), 100)
	if err != nil {
		logs = []*models.AuditLog{}
	}

	templ.Handler(layouts.AppLayout(pagesettings.AuditLogsPage(logs))).ServeHTTP(w, r)
}

// Storage Destinations handlers
func (h *Handler) StorageList(w http.ResponseWriter, r *http.Request) {
	destinations, err := h.storageRepo.List(r.Context())
	if err != nil {
		http.Error(w, "Failed to load storage destinations", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(pagesettings.StorageDestinationsPage(destinations))).ServeHTTP(w, r)
}

func (h *Handler) StorageCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	endpoint := r.FormValue("endpoint")
	bucket := r.FormValue("bucket")
	region := r.FormValue("region")
	accessKey := r.FormValue("access_key_id")
	secretKey := r.FormValue("secret_access_key")
	prefix := r.FormValue("prefix")
	isDefault := r.FormValue("is_default") == "true"

	if name == "" || endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	if region == "" {
		region = "us-east-1"
	}
	if prefix == "" {
		prefix = "backups"
	}

	_, err := h.storageRepo.Create(r.Context(), models.CreateStorageDestinationInput{
		Name:            name,
		IsDefault:       isDefault,
		Endpoint:        endpoint,
		Bucket:          bucket,
		Region:          region,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Prefix:          prefix,
		UseSSL:          true,
	})
	if err != nil {
		http.Error(w, "Failed to create storage destination: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/storage-destinations", http.StatusFound)
}

func (h *Handler) StorageSetDefault(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	if err := h.storageRepo.SetDefault(r.Context(), id); err != nil {
		http.Error(w, "Failed to set default storage destination", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/storage-destinations", http.StatusFound)
}

func (h *Handler) StorageDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	if err := h.storageRepo.Delete(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete storage destination", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/storage-destinations", http.StatusFound)
}

func (h *Handler) StorageTestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	dest, err := h.storageRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Storage destination not found", http.StatusNotFound)
		return
	}

	s3Client, err := backup.NewS3ClientFromDestination(dest)
	if err != nil {
		http.Error(w, "Failed to initialize client: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := s3Client.TestConnection(ctx); err != nil {
		http.Error(w, "Connection failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"success","message":"S3 Connection verified successfully"}`))
}

// System Backup handlers
func (h *Handler) BackupPage(w http.ResponseWriter, r *http.Request) {
	templ.Handler(layouts.AppLayout(pagesettings.BackupPage())).ServeHTTP(w, r)
}

func (h *Handler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("labuh-backup-%s.db", timestamp)
	backupFile := filepath.Join(h.backupPath, filename)

	if database.DB != nil {
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

	w.Header().Set("HX-Redirect", "/settings/backup")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	cleanName := filepath.Base(filepath.Clean(filename))
	if cleanName != filename || !strings.HasSuffix(cleanName, ".db") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	backupFile := filepath.Join(h.backupPath, cleanName)
	f, err := os.Open(backupFile)
	if err != nil {
		http.Error(w, "Backup not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", cleanName))
	_, _ = io.Copy(w, f)
}

func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
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

	if len(data) < 16 || !bytes.HasPrefix(data, []byte("SQLite format 3\x00")) {
		http.Error(w, "Invalid database backup file", http.StatusBadRequest)
		return
	}

	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "labuh.db"
	}

	_ = os.Rename(dbPath, dbPath+".bak")
	if err := os.WriteFile(dbPath, data, 0644); err != nil {
		_ = os.Rename(dbPath+".bak", dbPath)
		http.Error(w, "Failed to restore backup", http.StatusInternalServerError)
		return
	}
	_ = os.Remove(dbPath + ".bak")

	w.Header().Set("HX-Redirect", "/settings/backup")
	w.WriteHeader(http.StatusOK)
}
