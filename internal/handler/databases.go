package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/databases"
)

type DatabasesHandler struct {
	dbRepo       *repo.DatabaseRepo
	envRepo      *repo.ProjectRepo
	dockerClient *docker.Client
}

func NewDatabasesHandler(dbRepo *repo.DatabaseRepo, envRepo *repo.ProjectRepo) *DatabasesHandler {
	return &DatabasesHandler{
		dbRepo:  dbRepo,
		envRepo: envRepo,
	}
}

func (h *DatabasesHandler) SetDockerClient(client *docker.Client) {
	h.dockerClient = client
}

func (h *DatabasesHandler) List(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env_id")
	if envID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	page := parseInt(r.URL.Query().Get("page"), 1)
	search := r.URL.Query().Get("search")
	limit := 10
	offset := (page - 1) * limit

	var dbServices []*models.DatabaseService
	var total int
	var err error

	if search != "" {
		dbServices, total, err = h.dbRepo.SearchPaginated(envID, search, limit, offset)
	} else {
		dbServices, total, err = h.dbRepo.GetByEnvironmentIDPaginated(envID, limit, offset)
	}

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	templ.Handler(layouts.AppLayout(databases.DatabaseListPage(dbServices, page, totalPages, search))).ServeHTTP(w, r)
}

func (h *DatabasesHandler) Create(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env_id")
	if envID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	slug := r.FormValue("slug")
	engine := r.FormValue("engine")
	version := r.FormValue("version")
	username := r.FormValue("username")
	password := r.FormValue("password")
	databaseName := r.FormValue("database_name")
	port, _ := strconv.Atoi(r.FormValue("port"))

	if slug == "" {
		slug = repo.GenerateSlug(name)
	}
	if version == "" {
		version = "latest"
	}
	if username == "" {
		switch models.DatabaseEngine(engine) {
		case models.DatabaseEnginePostgres:
			username = "postgres"
		case models.DatabaseEngineMySQL:
			username = "root"
		case models.DatabaseEngineRedis:
			username = "default"
		case models.DatabaseEngineMongoDB:
			username = "admin"
		default:
			username = "root"
		}
	}
	if password == "" {
		password = generateSecurePassword(16)
	}
	if databaseName == "" {
		databaseName = strings.ReplaceAll(slug, "-", "_")
	}

	if port == 0 {
		switch models.DatabaseEngine(engine) {
		case models.DatabaseEnginePostgres:
			port = 5432
		case models.DatabaseEngineMySQL:
			port = 3306
		case models.DatabaseEngineRedis:
			port = 6379
		case models.DatabaseEngineMongoDB:
			port = 27017
		}
	}

	dbService, err := h.dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: envID,
		Name:          name,
		Slug:          slug,
		Engine:        models.DatabaseEngine(engine),
		Version:       version,
		Username:      username,
		Password:      password,
		DatabaseName:  databaseName,
		Port:          port,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Auto-provision container if docker client is available
	if h.dockerClient != nil {
		go func(db *models.DatabaseService) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			envSlug := "prod"
			if h.envRepo != nil {
				if env, envErr := h.envRepo.GetEnvironmentByID(db.EnvironmentID); envErr == nil && env != nil {
					envSlug = env.Slug
				}
			}

			imageName := GetDatabaseImage(db.Engine, db.Version)
			_ = h.dockerClient.PullImage(ctx, imageName)

			envVars := GetDatabaseEnvVars(db.Engine, db.Username, db.Password, db.DatabaseName)
			volumeName := fmt.Sprintf("labuh-vol-%s-%s", envSlug, db.Slug)
			containerName := fmt.Sprintf("labuh-db-%s-%s", envSlug, db.Slug)

			containerID, runErr := h.dockerClient.RunDatabaseContainer(ctx, envSlug, db.Slug, imageName, envVars, nil, volumeName)
			if runErr != nil {
				slog.Error("failed to start database container", "error", runErr, "db_id", db.ID)
				_ = h.dbRepo.UpdateStatus(db.ID, models.DatabaseStatusFailed)
				return
			}

			// Generate connection string accessible inside labuh-network
			connStr := GenerateConnectionString(db.Engine, db.Username, db.Password, containerName, db.Port, db.DatabaseName)
			_ = h.dbRepo.UpdateContainerAndStatus(db.ID, containerID, models.DatabaseStatusRunning)
			_ = h.dbRepo.UpdateConnectionString(db.ID, connStr)
		}(dbService)
	}

	w.Header().Set("HX-Redirect", "/projects/"+chi.URLParam(r, "project_id")+"/environments/"+envID+"/databases")
	w.WriteHeader(http.StatusOK)
}

func (h *DatabasesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	dbService, err := h.dbRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	templ.Handler(layouts.AppLayout(databases.DatabaseDetailPage(dbService))).ServeHTTP(w, r)
}

func (h *DatabasesHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	dbService, err := h.dbRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if h.dockerClient != nil && dbService.ContainerID != nil && *dbService.ContainerID != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if err := h.dockerClient.StartContainer(ctx, *dbService.ContainerID); err != nil {
			slog.Error("failed to start database container", "error", err)
		}
	}

	_ = h.dbRepo.UpdateStatus(id, models.DatabaseStatusRunning)
	w.Header().Set("HX-Redirect", "/databases/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *DatabasesHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	dbService, err := h.dbRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if h.dockerClient != nil && dbService.ContainerID != nil && *dbService.ContainerID != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		_ = h.dockerClient.StopContainer(ctx, *dbService.ContainerID, 10)
	}

	_ = h.dbRepo.UpdateStatus(id, models.DatabaseStatusStopped)
	w.Header().Set("HX-Redirect", "/databases/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *DatabasesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	dbService, err := h.dbRepo.GetByID(id)
	if err == nil && dbService != nil && h.dockerClient != nil && dbService.ContainerID != nil && *dbService.ContainerID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = h.dockerClient.StopContainer(ctx, *dbService.ContainerID, 5)
		_ = h.dockerClient.RemoveContainer(ctx, *dbService.ContainerID)
	}

	if err := h.dbRepo.Delete(id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects/"+chi.URLParam(r, "project_id")+"/environments/"+chi.URLParam(r, "env_id")+"/databases")
	w.WriteHeader(http.StatusOK)
}

func GetDatabaseImage(engine models.DatabaseEngine, version string) string {
	if version == "" || version == "latest" {
		switch engine {
		case models.DatabaseEnginePostgres:
			return "postgres:15-alpine"
		case models.DatabaseEngineMySQL:
			return "mysql:8"
		case models.DatabaseEngineRedis:
			return "redis:7-alpine"
		case models.DatabaseEngineMongoDB:
			return "mongo:6"
		}
	}
	return fmt.Sprintf("%s:%s", engine, version)
}

func GetDatabaseEnvVars(engine models.DatabaseEngine, username, password, databaseName string) map[string]string {
	envVars := map[string]string{}
	switch engine {
	case models.DatabaseEnginePostgres:
		envVars["POSTGRES_USER"] = username
		envVars["POSTGRES_PASSWORD"] = password
		envVars["POSTGRES_DB"] = databaseName
	case models.DatabaseEngineMySQL:
		envVars["MYSQL_ROOT_PASSWORD"] = password
		envVars["MYSQL_DATABASE"] = databaseName
		envVars["MYSQL_USER"] = username
		envVars["MYSQL_PASSWORD"] = password
	case models.DatabaseEngineMongoDB:
		envVars["MONGO_INITDB_ROOT_USERNAME"] = username
		envVars["MONGO_INITDB_ROOT_PASSWORD"] = password
		envVars["MONGO_INITDB_DATABASE"] = databaseName
	}
	return envVars
}

func GenerateConnectionString(engine models.DatabaseEngine, username, password, host string, port int, databaseName string) string {
	switch engine {
	case models.DatabaseEnginePostgres:
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", username, password, host, port, databaseName)
	case models.DatabaseEngineMySQL:
		return fmt.Sprintf("mysql://%s:%s@%s:%d/%s", username, password, host, port, databaseName)
	case models.DatabaseEngineRedis:
		if password != "" {
			return fmt.Sprintf("redis://:%s@%s:%d", password, host, port)
		}
		return fmt.Sprintf("redis://%s:%d", host, port)
	case models.DatabaseEngineMongoDB:
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin", username, password, host, port, databaseName)
	default:
		return fmt.Sprintf("%s://%s:%s@%s:%d/%s", engine, username, password, host, port, databaseName)
	}
}

func generateSecurePassword(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "labuh_secure_pass_123"
	}
	return hex.EncodeToString(bytes)
}
