package applications

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/akordium-id/get-labuh/internal/models"
)

// StackType merepresentasikan deteksi stack aplikasi dari source code.
type StackType string

const (
	StackNodeJS  StackType = "Node.js"
	StackPython  StackType = "Python"
	StackGo      StackType = "Go"
	StackPHP     StackType = "PHP"
	StackStatic  StackType = "Static Web"
	StackDocker  StackType = "Dockerfile"
	StackGeneric StackType = "Generic App"
)

// DropzoneHandler menangani upload file zip proyek dan auto deploy.
type DropzoneHandler struct {
	appRepo      *Repo
	projectRepo  ProjectEnvironmentProvider
	deployRepo   DeploymentCreator
	envVarRepo   *EnvVarRepo
	deployWorker DeployEnqueuer
}

// NewDropzoneHandler membuat instance baru DropzoneHandler.
func NewDropzoneHandler(
	appRepo *Repo,
	projectRepo ProjectEnvironmentProvider,
	deployRepo DeploymentCreator,
	envVarRepo *EnvVarRepo,
) *DropzoneHandler {
	return &DropzoneHandler{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		deployRepo:  deployRepo,
		envVarRepo:  envVarRepo,
	}
}

// SetDeployWorker menyetel worker antrean deployment.
func (h *DropzoneHandler) SetDeployWorker(w DeployEnqueuer) {
	h.deployWorker = w
}

// UploadDropzone menangani multipart file upload (.zip) dari Magic Dropzone.
func (h *DropzoneHandler) UploadDropzone(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		http.Error(w, "File terlalu besar (maksimal 100MB)", http.StatusBadRequest)
		return
	}

	appName := strings.TrimSpace(r.FormValue("name"))
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File upload tidak ditemukan: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if appName == "" {
		appName = strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))
		if appName == "" {
			appName = "my-app"
		}
	}

	slug := GenerateSlug(appName)

	envID, err := h.getOrCreateDefaultEnvironment()
	if err != nil {
		http.Error(w, "Gagal menyiapkan environment default: "+err.Error(), http.StatusInternalServerError)
		return
	}

	app, err := h.appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: envID,
		Name:          appName,
		Slug:          slug,
		SourceType:    models.SourceTypeDropzone,
		AppPort:       8080,
	})
	if err != nil {
		http.Error(w, "Gagal membuat aplikasi: "+err.Error(), http.StatusInternalServerError)
		return
	}

	targetDir := filepath.Join("/tmp/labuh-sources", app.ID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		http.Error(w, "Gagal membuat direktori sumber: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tempZipPath := filepath.Join(os.TempDir(), fmt.Sprintf("upload-%s.zip", app.ID))
	outZip, err := os.Create(tempZipPath)
	if err != nil {
		http.Error(w, "Gagal menyimpan file upload: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(outZip, file); err != nil {
		_ = outZip.Close()
		http.Error(w, "Gagal menulis file upload: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = outZip.Close()
	defer os.Remove(tempZipPath)

	if err := extractZipArchive(tempZipPath, targetDir); err != nil {
		http.Error(w, "Gagal mengekstrak archive ZIP: "+err.Error(), http.StatusBadRequest)
		return
	}

	detectedStack, detectedPort := DetectAppStack(targetDir)
	if detectedPort > 0 && detectedPort != 8080 {
		app.AppPort = detectedPort
	}

	deployment, err := h.deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
	})
	if err != nil {
		http.Error(w, "Gagal membuat deployment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logPath := fmt.Sprintf("/tmp/labuh-logs/%s.log", deployment.ID)
	_ = h.appRepo.UpdateStatus(app.ID, models.AppStatusBuilding)
	_ = h.deployRepo.MarkStarted(deployment.ID)
	_ = h.deployRepo.UpdateStatus(deployment.ID, models.DeployStatusQueued)

	if h.deployWorker != nil {
		var envVars []*models.AppEnvVar
		if h.envVarRepo != nil {
			envVars, _ = h.envVarRepo.GetByApplicationID(app.ID)
		}
		job := BuildDeploymentJob(app, deployment.ID, envVars)
		job.LogPath = logPath
		if err := h.deployWorker.Enqueue(job); err != nil {
			slog.Error("failed to enqueue dropzone deploy job", "error", err, "deployment_id", deployment.ID)
		}
	}

	toastMsg := fmt.Sprintf("Aplikasi %s berhasil diunggah (%s terdeteksi). Deployment dimulai!", app.Name, detectedStack)
	setToast(w, toastMsg, "success")

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// DetectAppStack menganalisis isi folder proyek untuk menebak stack bahasa dan port.
func DetectAppStack(dir string) (StackType, int) {
	if fileExists(filepath.Join(dir, "Dockerfile")) {
		return StackDocker, 8080
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		if content, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
			var pkg map[string]any
			if json.Unmarshal(content, &pkg) == nil {
				if deps, ok := pkg["dependencies"].(map[string]any); ok {
					if _, hasNext := deps["next"]; hasNext {
						return "Next.js", 3000
					}
					if _, hasVite := deps["vite"]; hasVite {
						return "Vite / React", 80
					}
				}
			}
		}
		return StackNodeJS, 3000
	}
	if fileExists(filepath.Join(dir, "go.mod")) {
		return StackGo, 8080
	}
	if fileExists(filepath.Join(dir, "requirements.txt")) || fileExists(filepath.Join(dir, "Pipfile")) || fileExists(filepath.Join(dir, "pyproject.toml")) {
		return StackPython, 8000
	}
	if fileExists(filepath.Join(dir, "composer.json")) || fileExists(filepath.Join(dir, "index.php")) {
		return StackPHP, 80
	}
	if fileExists(filepath.Join(dir, "index.html")) {
		return StackStatic, 80
	}
	return StackGeneric, 8080
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func extractZipArchive(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	var rootPrefix string
	if len(r.File) > 0 {
		firstFile := r.File[0].Name
		parts := strings.Split(firstFile, "/")
		if len(parts) > 1 && parts[0] != "" {
			possiblePrefix := parts[0] + "/"
			allPrefixed := true
			for _, f := range r.File {
				if !strings.HasPrefix(f.Name, possiblePrefix) && f.Name != strings.TrimSuffix(possiblePrefix, "/") {
					allPrefixed = false
					break
				}
			}
			if allPrefixed {
				rootPrefix = possiblePrefix
			}
		}
	}

	for _, f := range r.File {
		cleanName := strings.TrimPrefix(f.Name, rootPrefix)
		if cleanName == "" {
			continue
		}

		targetPath := filepath.Join(destDir, cleanName)

		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(targetPath, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		_ = outFile.Close()
		_ = rc.Close()

		if copyErr != nil {
			return copyErr
		}
	}

	return nil
}

func (h *DropzoneHandler) getOrCreateDefaultEnvironment() (string, error) {
	if h.projectRepo == nil {
		return "default", nil
	}
	projects, err := h.projectRepo.GetAll()
	if err != nil {
		return "", err
	}

	var targetProject *models.Project
	if len(projects) > 0 {
		targetProject = projects[0]
	} else {
		desc := "Proyek bawaan untuk aplikasi Labuh Lite"
		newProj, err := h.projectRepo.Create(models.CreateProjectInput{
			Name:        "Default Project",
			Slug:        "default-project",
			Description: &desc,
		})
		if err != nil {
			return "", err
		}
		targetProject = newProj
	}

	envs, err := h.projectRepo.GetEnvironmentsByProjectID(targetProject.ID)
	if err != nil {
		return "", err
	}
	if len(envs) > 0 {
		return envs[0].ID, nil
	}

	newEnv, err := h.projectRepo.CreateEnvironment(targetProject.ID, "Production", "production")
	if err != nil {
		return "", err
	}
	return newEnv.ID, nil
}
