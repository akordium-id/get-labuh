package handler

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
)

func createSampleZip(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return buf.Bytes()
}

func TestDetectAppStack(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "detect-stack-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Case 1: Dockerfile
	_ = os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte("FROM alpine"), 0644)
	stack, port := DetectAppStack(tempDir)
	if stack != StackDocker || port != 8080 {
		t.Errorf("expected Dockerfile/8080, got %s/%d", stack, port)
	}
	_ = os.Remove(filepath.Join(tempDir, "Dockerfile"))

	// Case 2: Node.js / Next.js
	packageJSON := `{"dependencies": {"next": "14.0.0"}}`
	_ = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644)
	stack, port = DetectAppStack(tempDir)
	if stack != "Next.js" || port != 3000 {
		t.Errorf("expected Next.js/3000, got %s/%d", stack, port)
	}
	_ = os.Remove(filepath.Join(tempDir, "package.json"))

	// Case 3: Python
	_ = os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte("fastapi"), 0644)
	stack, port = DetectAppStack(tempDir)
	if stack != StackPython || port != 8000 {
		t.Errorf("expected Python/8000, got %s/%d", stack, port)
	}
	_ = os.Remove(filepath.Join(tempDir, "requirements.txt"))

	// Case 4: Go
	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module myapp"), 0644)
	stack, port = DetectAppStack(tempDir)
	if stack != StackGo || port != 8080 {
		t.Errorf("expected Go/8080, got %s/%d", stack, port)
	}
}

func TestDropzoneUploadHandler(t *testing.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	defer db.Close()

	appRepo := repo.NewApplicationRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	envVarRepo := repo.NewEnvVarRepo(db)

	h := NewDropzoneHandler(appRepo, projectRepo, deployRepo, envVarRepo)

	// Buat payload multipart ZIP
	zipBytes := createSampleZip(t, map[string]string{
		"index.html": "<h1>Hello Labuh Lite</h1>",
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "Test Dropzone App")
	part, err := writer.CreateFormFile("file", "app.zip")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(zipBytes)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/lite/deploy/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	h.UploadDropzone(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	if redirect := rec.Header().Get("HX-Redirect"); redirect != "/dashboard" {
		t.Errorf("expected HX-Redirect to /dashboard, got %s", redirect)
	}

	// Verifikasi record app terbuat di repo
	apps, err := appRepo.GetAll()
	if err != nil || len(apps) == 0 {
		t.Fatalf("expected application created in DB, got err: %v, count: %d", err, len(apps))
	}
	if apps[0].Name != "Test Dropzone App" {
		t.Errorf("expected app name 'Test Dropzone App', got '%s'", apps[0].Name)
	}
	if apps[0].SourceType != models.SourceTypeDropzone {
		t.Errorf("expected sourceType 'dropzone', got '%s'", apps[0].SourceType)
	}
}
