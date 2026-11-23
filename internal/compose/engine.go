package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

// DockerClient mendefinisikan antarmuka komunikasi Docker yang dibutuhkan Orchestrator.
type DockerClient interface {
	ListContainers(ctx context.Context, opts container.ListOptions) ([]types.Container, error)
	ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error)
}

// Orchestrator mengelola siklus hidup Docker Compose stack (write YAML/.env, up, down, restart, inspect).
type Orchestrator struct {
	baseDir      string
	dockerClient DockerClient
	mu           sync.Mutex
}

// NewOrchestrator membuat instance baru Docker Compose Orchestrator.
func NewOrchestrator(baseDir string, dockerClient DockerClient) *Orchestrator {
	if baseDir == "" {
		baseDir = "/var/lib/labuh/compose"
	}
	return &Orchestrator{
		baseDir:      baseDir,
		dockerClient: dockerClient,
	}
}

// getEffectiveBaseDir mengembalikan baseDir yang writable, fallback ke TempDir jika tidak bisa write.
func (o *Orchestrator) getEffectiveBaseDir() string {
	err := os.MkdirAll(o.baseDir, 0755)
	if err == nil {
		return o.baseDir
	}
	tmp := filepath.Join(os.TempDir(), "labuh-compose")
	_ = os.MkdirAll(tmp, 0755)
	return tmp
}

// GetProjectName mengembalikan nama project compose (default ke app.Slug jika ComposeProjectName kosong).
func GetProjectName(app *models.ComposeApplication) string {
	if app.ComposeProjectName != nil && *app.ComposeProjectName != "" {
		return *app.ComposeProjectName
	}
	if app.Slug != "" {
		return app.Slug
	}
	return "compose-" + app.ID[:8]
}

// PrepareWorkDir menyiapkan direktori kerja, menulis file docker-compose.yml dan .env ter-resolve.
func (o *Orchestrator) PrepareWorkDir(app *models.ComposeApplication, envVars []models.AppEnvVar) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	workDir := filepath.Join(o.getEffectiveBaseDir(), app.ID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("gagal membuat direktori kerja compose: %w", err)
	}

	// 1. Tulis docker-compose.yml
	yamlContent := strings.TrimSpace(app.RawYAML)
	if yamlContent == "" && app.ComposeFilePath != "" {
		// fallback jika raw_yaml belum ada tetapi ada compose file lokal
		if content, err := os.ReadFile(app.ComposeFilePath); err == nil {
			yamlContent = string(content)
		}
	}
	if yamlContent == "" {
		return "", fmt.Errorf("konten docker-compose.yml kosong")
	}

	composeFile := filepath.Join(workDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(yamlContent), 0644); err != nil {
		return "", fmt.Errorf("gagal menulis file docker-compose.yml: %w", err)
	}

	// 2. Tulis .env file untuk interpolasi variabel otomatis
	var envBuf bytes.Buffer
	for _, ev := range envVars {
		escaped := strings.ReplaceAll(ev.Value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		envBuf.WriteString(fmt.Sprintf("%s=\"%s\"\n", ev.Key, escaped))
	}

	envFile := filepath.Join(workDir, ".env")
	if err := os.WriteFile(envFile, envBuf.Bytes(), 0600); err != nil {
		return "", fmt.Errorf("gagal menulis file .env: %w", err)
	}

	return workDir, nil
}

// Up menjalankan 'docker compose up -d --build' untuk stack yang diberikan.
func (o *Orchestrator) Up(ctx context.Context, app *models.ComposeApplication, envVars []models.AppEnvVar) (string, error) {
	workDir, err := o.PrepareWorkDir(app, envVars)
	if err != nil {
		return "", err
	}

	projectName := GetProjectName(app)
	logDir := filepath.Join(os.TempDir(), "labuh-logs")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, fmt.Sprintf("compose-%s.log", app.ID))

	args := []string{"compose", "-p", projectName, "-f", "docker-compose.yml", "up", "-d", "--build"}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workDir

	output, cmdErr := cmd.CombinedOutput()
	logContent := fmt.Sprintf("[%s] docker %s\n%s\n", time.Now().Format("2006-01-02 15:04:05"), strings.Join(args, " "), string(output))
	_ = os.WriteFile(logPath, []byte(logContent), 0644)

	if cmdErr != nil {
		return string(output), fmt.Errorf("docker compose up gagal: %w (output: %s)", cmdErr, strings.TrimSpace(string(output)))
	}

	return string(output), nil
}

// Down menjalankan 'docker compose down' untuk menghentikan seluruh container di stack.
func (o *Orchestrator) Down(ctx context.Context, app *models.ComposeApplication) (string, error) {
	workDir := filepath.Join(o.getEffectiveBaseDir(), app.ID)
	projectName := GetProjectName(app)

	args := []string{"compose", "-p", projectName, "-f", "docker-compose.yml", "down"}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("docker compose down gagal: %w", err)
	}

	return string(output), nil
}

// RestartService merestart satu sub-service tertentu di dalam compose stack.
func (o *Orchestrator) RestartService(ctx context.Context, app *models.ComposeApplication, serviceName string) (string, error) {
	workDir := filepath.Join(o.getEffectiveBaseDir(), app.ID)
	projectName := GetProjectName(app)

	args := []string{"compose", "-p", projectName, "-f", "docker-compose.yml", "restart", serviceName}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("docker compose restart %s gagal: %w", serviceName, err)
	}

	return string(output), nil
}

// InspectServices mengkorelasikan definisi YAML dengan runtime container via Docker SDK filter label.
func (o *Orchestrator) InspectServices(ctx context.Context, app *models.ComposeApplication) ([]models.ComposeServiceRuntime, error) {
	// 1. Dapatkan daftar definisi service
	servicesDef := app.ParsedServices
	if len(servicesDef) == 0 && app.RawYAML != "" {
		parsed, err := ParseComposeYAML(app.RawYAML)
		if err == nil {
			servicesDef = parsed
		}
	}

	// 2. Ambil container aktif via Docker SDK jika client tersedia
	containerMap := make(map[string]types.Container)
	if o.dockerClient != nil {
		projectName := GetProjectName(app)
		f := filters.NewArgs()
		f.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))

		containers, err := o.dockerClient.ListContainers(ctx, container.ListOptions{
			All:     true,
			Filters: f,
		})
		if err == nil {
			for _, c := range containers {
				if svcName, ok := c.Labels["com.docker.compose.service"]; ok {
					containerMap[svcName] = c
				}
			}
		}
	}

	// 3. Bangun runtime status
	var runtimes []models.ComposeServiceRuntime
	for _, def := range servicesDef {
		rt := models.ComposeServiceRuntime{
			Name:       def.Name,
			Definition: def,
			Status:     "not_created",
		}

		if c, found := containerMap[def.Name]; found {
			rt.ContainerID = c.ID
			rt.Status = c.State
			rt.State = c.Status
			if strings.Contains(strings.ToLower(c.Status), "healthy") {
				rt.Health = "healthy"
			} else if strings.Contains(strings.ToLower(c.Status), "unhealthy") {
				rt.Health = "unhealthy"
			}
			rt.CreatedAt = time.Unix(c.Created, 0).Format("2006-01-02 15:04:05")
		}

		runtimes = append(runtimes, rt)
	}

	return runtimes, nil
}

// GetServiceLogs membaca runtime log dari sub-service tertentu menggunakan Docker SDK.
func (o *Orchestrator) GetServiceLogs(ctx context.Context, app *models.ComposeApplication, serviceName string, tail int) (string, error) {
	if o.dockerClient == nil {
		return "", fmt.Errorf("docker client tidak tersedia")
	}

	services, err := o.InspectServices(ctx, app)
	if err != nil {
		return "", err
	}

	var targetID string
	for _, s := range services {
		if s.Name == serviceName {
			targetID = s.ContainerID
			break
		}
	}

	if targetID == "" {
		return "", fmt.Errorf("container untuk service '%s' belum dibuat atau tidak ditemukan", serviceName)
	}

	if tail <= 0 {
		tail = 100
	}
	if tail > 2000 {
		tail = 2000
	}

	rc, err := o.dockerClient.ContainerLogs(ctx, targetID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
		Timestamps: true,
	})
	if err != nil {
		return "", fmt.Errorf("gagal membaca logs container: %w", err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("gagal membaca logs container: %w", err)
	}

	var buf bytes.Buffer
	if err := docker.DemultiplexLogs(&buf, bytes.NewReader(raw)); err != nil || buf.Len() == 0 {
		// Fallback to raw string if stream is not multiplexed with stdcopy header
		return string(raw), nil
	}

	return buf.String(), nil
}
