package testing

import (
	"context"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/auth"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
	"github.com/stretchr/testify/assert"
)

func TestIntegration_AuthFlow(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	defer db.Close()

	authMod := auth.New(db, false)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	_, _ = authMod.UserRepo.Create(context.Background(), models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test User",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.PostForm = map[string][]string{
		"email":    {"test@example.com"},
		"password": {"password"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	authMod.Handler.Login(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cookies := resp.Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)

	req = httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: cookies[0].Value})
	w = httptest.NewRecorder()

	authMod.Handler.Logout(w, req)

	resp = w.Result()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/auth/login", resp.Header.Get("Location"))
}

func TestIntegration_ProjectCreationFlow(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	defer db.Close()

	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	projectsMod := projects.New(db, nil)

	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.PostForm = map[string][]string{
		"name": {"Integration Project"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	projectsMod.Handler.ProjectsCreate(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/projects", resp.Header.Get("HX-Redirect"))

	projects, _ := projectRepo.GetAll()
	assert.Len(t, projects, 1)
	assert.Equal(t, "Integration Project", projects[0].Name)

	env, _ := projectRepo.CreateEnvironment(projects[0].ID, "Production", "production")
	assert.NotEmpty(t, env.ID)

	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Integration App",
		Slug:          "integration-app",
		SourceType:    models.SourceTypeGit,
	})
	assert.NotEmpty(t, app.ID)
	assert.Equal(t, "Integration App", app.Name)
}

func TestIntegration_DeploymentFlow(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	defer db.Close()

	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)

	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "Deploy Project", Slug: "deploy-project"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Production", "production")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Deploy App",
		Slug:          "deploy-app",
		SourceType:    models.SourceTypeGit,
	})

	deployment, _ := deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
	})
	assert.Equal(t, models.DeployStatusQueued, deployment.Status)

	_ = deployRepo.MarkStarted(deployment.ID)
	_ = deployRepo.UpdateStatus(deployment.ID, models.DeployStatusCloning)
	_ = deployRepo.UpdateStep(deployment.ID, "clone")
	_ = deployRepo.UpdateOutput(deployment.ID, "cloning repository...")
	_ = deployRepo.UpdateStatus(deployment.ID, models.DeployStatusBuilding)
	_ = deployRepo.UpdateStep(deployment.ID, "build")
	_ = deployRepo.UpdateOutput(deployment.ID, "building docker image...")
	_ = deployRepo.UpdateStatus(deployment.ID, models.DeployStatusSuccess)
	_ = deployRepo.MarkFinished(deployment.ID)

	updated, _ := deployRepo.GetByID(deployment.ID)
	assert.Equal(t, models.DeployStatusSuccess, updated.Status)
	assert.NotNil(t, updated.StartedAt)
	assert.NotNil(t, updated.FinishedAt)

	deployments, _ := deployRepo.GetByApplicationID(app.ID)
	assert.Len(t, deployments, 1)
}

func TestIntegration_ApplicationVolumesAndFileMountsFlow(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)

	project, err := projectRepo.Create(models.CreateProjectInput{Name: "Storage Project", Slug: "storage-project"})
	assert.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(project.ID, "Production", "production")
	assert.NoError(t, err)

	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Storage Stateful App",
		Slug:          "storage-stateful-app",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       8080,
	})
	assert.NoError(t, err)

	// 1. Add Named Volume
	volNamed, err := appRepo.CreateVolume(ctx, models.CreateAppVolumeInput{
		ApplicationID: app.ID,
		Name:          "uploads",
		VolumeType:    models.VolumeTypeNamed,
		MountPath:     "/app/uploads",
		IsReadOnly:    false,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, volNamed.ID)
	assert.Equal(t, models.VolumeTypeNamed, volNamed.VolumeType)

	// 2. Add Valid Bind Volume
	validHostPath := "/opt/data/myapp"
	volBind, err := appRepo.CreateVolume(ctx, models.CreateAppVolumeInput{
		ApplicationID: app.ID,
		Name:          "bind-data",
		VolumeType:    models.VolumeTypeBind,
		HostPath:      &validHostPath,
		MountPath:     "/app/data",
		IsReadOnly:    true,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, volBind.ID)
	assert.Equal(t, models.VolumeTypeBind, volBind.VolumeType)

	// Verify volumes list
	vols, err := appRepo.GetVolumesByAppID(ctx, app.ID)
	assert.NoError(t, err)
	assert.Len(t, vols, 2)

	// 3. Add File Mount
	fm, err := appRepo.CreateFileMount(ctx, models.CreateAppFileMountInput{
		ApplicationID: app.ID,
		MountPath:     "/etc/nginx/conf.d/app.conf",
		Content:       "server { listen 80; }",
		FileMode:      "0644",
		IsReadOnly:    true,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, fm.ID)

	// 4. Update File Mount
	err = appRepo.UpdateFileMount(ctx, fm.ID, "server { listen 8080; }")
	assert.NoError(t, err)
	updatedFm, err := appRepo.GetFileMountByID(ctx, fm.ID)
	assert.NoError(t, err)
	assert.Equal(t, "server { listen 8080; }", updatedFm.Content)

	// Verify file mounts list
	fms, err := appRepo.GetFileMountsByAppID(ctx, app.ID)
	assert.NoError(t, err)
	assert.Len(t, fms, 1)

	// 5. Delete Volume & File Mount
	err = appRepo.DeleteVolume(ctx, volNamed.ID)
	assert.NoError(t, err)
	volsAfter, err := appRepo.GetVolumesByAppID(ctx, app.ID)
	assert.NoError(t, err)
	assert.Len(t, volsAfter, 1)

	err = appRepo.DeleteFileMount(ctx, fm.ID)
	assert.NoError(t, err)
	fmsAfter, err := appRepo.GetFileMountsByAppID(ctx, app.ID)
	assert.NoError(t, err)
	assert.Len(t, fmsAfter, 0)
}

