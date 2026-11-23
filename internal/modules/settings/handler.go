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
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/observability"
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

type AlertDispatcher interface {
	Dispatch(event models.NotificationEvent)
}

type Handler struct {
	settingRepo      *SettingRepo
	auditRepo        *AuditLogRepo
	storageRepo      *StorageDestinationRepo
	registryRepo     *RegistryRepo
	dockerClient     *docker.Client
	notifier         AlertDispatcher
	backupPath       string
	cleanupScheduler *CleanupScheduler
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

func (h *Handler) SetRegistryRepo(r *RegistryRepo) {
	h.registryRepo = r
}

func (h *Handler) SetDockerClient(c *docker.Client) {
	h.dockerClient = c
}

func (h *Handler) SetCleanupScheduler(s *CleanupScheduler) {
	h.cleanupScheduler = s
}

// Caddy & Mode handlers
func (h *Handler) Caddy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	caddyAPIURL, _ := h.settingRepo.Get(ctx, "caddy_api_url")
	caddyAPIKey, _ := h.settingRepo.Get(ctx, "caddy_api_key")
	caddyNetwork, _ := h.settingRepo.Get(ctx, "caddy_network")
	githubBotToken, _ := h.settingRepo.Get(ctx, "github_bot_token")
	gitlabBotToken, _ := h.settingRepo.Get(ctx, "gitlab_bot_token")

	templ.Handler(layouts.AppLayout(pagesettings.CaddySettingsPage(caddyAPIURL, caddyAPIKey, caddyNetwork, githubBotToken, gitlabBotToken))).ServeHTTP(w, r)
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

	if r.Form.Has("github_bot_token") {
		_ = h.settingRepo.Set(ctx, "github_bot_token", strings.TrimSpace(r.FormValue("github_bot_token")))
	}
	if r.Form.Has("gitlab_bot_token") {
		_ = h.settingRepo.Set(ctx, "gitlab_bot_token", strings.TrimSpace(r.FormValue("gitlab_bot_token")))
	}

	SetToast(w, "Pengaturan berhasil disimpan", ToastSuccess)
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

func (h *Handler) GetCleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	enabled, _ := h.settingRepo.Get(ctx, "docker_cleanup_enabled")
	if enabled == "" {
		enabled = "false"
	}
	schedule, _ := h.settingRepo.Get(ctx, "docker_cleanup_schedule")
	if schedule == "" {
		schedule = "0 2 * * *"
	}
	lastRun, _ := h.settingRepo.Get(ctx, "docker_cleanup_last_run")
	if lastRun == "" {
		lastRun = "-"
	}
	lastReclaimed, _ := h.settingRepo.Get(ctx, "docker_cleanup_last_reclaimed")
	if lastReclaimed == "" {
		lastReclaimed = "0 B"
	}

	templ.Handler(layouts.AppLayout(pagesettings.CleanupPage(enabled == "true", schedule, lastRun, lastReclaimed))).ServeHTTP(w, r)
}

func (h *Handler) UpdateCleanupSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	enabled := "false"
	if r.FormValue("docker_cleanup_enabled") == "true" || r.FormValue("docker_cleanup_enabled") == "on" {
		enabled = "true"
	}
	schedule := strings.TrimSpace(r.FormValue("docker_cleanup_schedule"))
	if schedule == "" {
		schedule = "0 2 * * *"
	}

	_ = h.settingRepo.Set(ctx, "docker_cleanup_enabled", enabled)
	_ = h.settingRepo.Set(ctx, "docker_cleanup_schedule", schedule)

	SetToast(w, "Docker cleanup settings saved successfully", ToastSuccess)
	http.Redirect(w, r, "/settings/cleanup", http.StatusFound)
}

func (h *Handler) RunCleanupNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.cleanupScheduler != nil {
		report, err := h.cleanupScheduler.RunCleanup(ctx)
		if err != nil {
			slog.Error("manual cleanup failed", "err", err)
			SetToast(w, "Failed to run Docker cleanup: "+err.Error(), ToastError)
		} else {
			SetToast(w, fmt.Sprintf("Cleanup finished: freed %s", report.SpaceFormatted), ToastSuccess)
		}
	} else {
		SetToast(w, "Cleanup scheduler is not initialized", ToastError)
	}
	http.Redirect(w, r, "/settings/cleanup", http.StatusFound)
}

// Docker Registries handlers
func (h *Handler) RegistriesPage(w http.ResponseWriter, r *http.Request) {
	if h.registryRepo == nil {
		http.Error(w, "Registry repository not initialized", http.StatusInternalServerError)
		return
	}
	registries, err := h.registryRepo.List(r.Context())
	if err != nil {
		http.Error(w, "Failed to load docker registries", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(pagesettings.RegistriesPage(registries))).ServeHTTP(w, r)
}

func (h *Handler) CreateRegistry(w http.ResponseWriter, r *http.Request) {
	if h.registryRepo == nil {
		http.Error(w, "Registry repository not initialized", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	serverAddress := strings.TrimSpace(r.FormValue("server_address"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	email := strings.TrimSpace(r.FormValue("email"))

	if name == "" || serverAddress == "" || username == "" || password == "" {
		http.Error(w, "Missing required fields (name, server_address, username, password)", http.StatusBadRequest)
		return
	}

	input := models.CreateDockerRegistryInput{
		Name:          name,
		ServerAddress: serverAddress,
		Username:      username,
		Password:      password,
		Email:         email,
	}

	reg, err := h.registryRepo.Create(r.Context(), input)
	if err != nil {
		http.Error(w, "Failed to create registry: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditRepo != nil {
		_, _ = h.auditRepo.Create(r.Context(), models.CreateAuditLogInput{
			Action:     "create_docker_registry",
			Resource:   "docker_registries",
			ResourceID: reg.ID,
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		})
	}

	http.Redirect(w, r, "/settings/registries", http.StatusSeeOther)
}

func (h *Handler) DeleteRegistry(w http.ResponseWriter, r *http.Request) {
	if h.registryRepo == nil {
		http.Error(w, "Registry repository not initialized", http.StatusInternalServerError)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	if err := h.registryRepo.Delete(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete registry: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditRepo != nil {
		_, _ = h.auditRepo.Create(r.Context(), models.CreateAuditLogInput{
			Action:     "delete_docker_registry",
			Resource:   "docker_registries",
			ResourceID: id,
			IPAddress:  r.RemoteAddr,
			UserAgent:  r.UserAgent(),
		})
	}

	http.Redirect(w, r, "/settings/registries", http.StatusSeeOther)
}

func (h *Handler) TestRegistryConnection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	serverAddress := strings.TrimSpace(r.FormValue("server_address"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))
	email := strings.TrimSpace(r.FormValue("email"))

	if serverAddress == "" || username == "" || password == "" {
		http.Error(w, "Missing required credentials for connection test", http.StatusBadRequest)
		return
	}

	if h.dockerClient == nil {
		SetToast(w, "Docker daemon client is not available", ToastError)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<div class="text-rose-600 text-xs font-semibold">Gagal: Docker daemon client tidak tersedia</div>`))
		return
	}

	auth := &docker.RegistryAuth{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
		Email:         email,
	}
	if err := h.dockerClient.VerifyRegistryAuth(r.Context(), auth); err != nil {
		SetToast(w, "Registry authentication failed: "+err.Error(), ToastError)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(fmt.Sprintf(`<div class="text-rose-600 text-xs font-semibold">Gagal: %v</div>`, err)))
		return
	}

	SetToast(w, "Koneksi ke Docker Registry berhasil!", ToastSuccess)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<div class="text-emerald-600 text-xs font-semibold">✓ Autentikasi Docker Registry berhasil!</div>`))
}

func (h *Handler) SetNotifier(n AlertDispatcher) {
	h.notifier = n
}

// MonitoringPage menampilkan visualisasi realtime metrik host server dan konfigurasi threshold alert.
func (h *Handler) MonitoringPage(w http.ResponseWriter, r *http.Request) {
	stats := observability.GetHostStats("")

	diskWarn, _ := h.settingRepo.Get(r.Context(), "host_disk_warning_pct")
	if diskWarn == "" {
		diskWarn = "80"
	}
	diskCrit, _ := h.settingRepo.Get(r.Context(), "host_disk_critical_pct")
	if diskCrit == "" {
		diskCrit = "90"
	}
	ramWarn, _ := h.settingRepo.Get(r.Context(), "host_ram_warning_pct")
	if ramWarn == "" {
		ramWarn = "90"
	}
	cooldown, _ := h.settingRepo.Get(r.Context(), "host_alert_cooldown_minutes")
	if cooldown == "" {
		cooldown = "30"
	}
	enabled, _ := h.settingRepo.Get(r.Context(), "host_monitoring_enabled")
	if enabled == "" {
		enabled = "true"
	}

	settingsMap := map[string]string{
		"host_monitoring_enabled":     enabled,
		"host_disk_warning_pct":       diskWarn,
		"host_disk_critical_pct":      diskCrit,
		"host_ram_warning_pct":        ramWarn,
		"host_alert_cooldown_minutes": cooldown,
	}

	templ.Handler(layouts.AppLayout(pagesettings.MonitoringPage(stats, settingsMap))).ServeHTTP(w, r)
}

// UpdateMonitoringSettings memperbarui ambang batas resource dan status aktif daemon monitoring.
func (h *Handler) UpdateMonitoringSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	enabled := "false"
	if r.FormValue("host_monitoring_enabled") == "on" || r.FormValue("host_monitoring_enabled") == "true" {
		enabled = "true"
	}
	_ = h.settingRepo.Set(r.Context(), "host_monitoring_enabled", enabled)

	if diskWarn := strings.TrimSpace(r.FormValue("host_disk_warning_pct")); diskWarn != "" {
		_ = h.settingRepo.Set(r.Context(), "host_disk_warning_pct", diskWarn)
	}
	if diskCrit := strings.TrimSpace(r.FormValue("host_disk_critical_pct")); diskCrit != "" {
		_ = h.settingRepo.Set(r.Context(), "host_disk_critical_pct", diskCrit)
	}
	if ramWarn := strings.TrimSpace(r.FormValue("host_ram_warning_pct")); ramWarn != "" {
		_ = h.settingRepo.Set(r.Context(), "host_ram_warning_pct", ramWarn)
	}
	if cooldown := strings.TrimSpace(r.FormValue("host_alert_cooldown_minutes")); cooldown != "" {
		_ = h.settingRepo.Set(r.Context(), "host_alert_cooldown_minutes", cooldown)
	}

	SetToast(w, "Pengaturan host monitoring berhasil disimpan", ToastSuccess)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/settings/monitoring")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/settings/monitoring", http.StatusSeeOther)
}

// TestMonitoringAlert memicu simulasi notifikasi alert untuk menguji konfigurasi channel notifikasi.
func (h *Handler) TestMonitoringAlert(w http.ResponseWriter, r *http.Request) {
	if h.notifier == nil {
		SetToast(w, "Notifier service belum diinisialisasi", ToastError)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	event := models.NotificationEvent{
		Title:       "🔔 Test Server Resource Alert",
		Description: "Ini adalah notifikasi uji coba dari sistem Server Resource Monitoring get-labuh.",
		Status:      "warning",
		EventType:   models.EventServerAlertWarning,
		Fields: map[string]string{
			"Trigger":   "Manual Test via Settings UI",
			"Timestamp": time.Now().Format("2006-01-02 15:04:05"),
		},
	}
	h.notifier.Dispatch(event)

	SetToast(w, "Notifikasi uji coba berhasil dikirim ke seluruh channel yang aktif!", ToastSuccess)
	w.WriteHeader(http.StatusOK)
}
