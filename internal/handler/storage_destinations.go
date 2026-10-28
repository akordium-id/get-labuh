package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/backup"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type StorageDestinationsHandler struct {
	storageRepo *repo.StorageDestinationRepo
}

func NewStorageDestinationsHandler(storageRepo *repo.StorageDestinationRepo) *StorageDestinationsHandler {
	return &StorageDestinationsHandler{
		storageRepo: storageRepo,
	}
}

func (h *StorageDestinationsHandler) List(w http.ResponseWriter, r *http.Request) {
	destinations, err := h.storageRepo.List()
	if err != nil {
		http.Error(w, "Failed to load storage destinations", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.StorageDestinationsPage(destinations))).ServeHTTP(w, r)
}

func (h *StorageDestinationsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	_, err := h.storageRepo.Create(models.CreateStorageDestinationInput{
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

func (h *StorageDestinationsHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	if err := h.storageRepo.SetDefault(id); err != nil {
		http.Error(w, "Failed to set default storage destination", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/storage-destinations", http.StatusFound)
}

func (h *StorageDestinationsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	if err := h.storageRepo.Delete(id); err != nil {
		http.Error(w, "Failed to delete storage destination", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/storage-destinations", http.StatusFound)
}

func (h *StorageDestinationsHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	dest, err := h.storageRepo.GetByID(id)
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
	w.Write([]byte(`{"status":"success","message":"S3 Connection verified successfully"}`))
}
