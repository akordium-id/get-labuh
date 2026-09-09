package testing

import (
	stdtesting "testing"
	"net/http"
	"net/http/httptest"

	"golang.org/x/crypto/bcrypt"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/handler"
	"github.com/akordium-id/get-labuh/internal/models"
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

	userRepo := repo.NewUserRepo(db)
	sessionRepo := repo.NewSessionRepo(db)
	authHandler := handler.NewAuthHandler(userRepo, sessionRepo, false)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	userRepo.Create(models.CreateUserInput{
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

	authHandler.Login(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cookies := resp.Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)

	req = httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: cookies[0].Value})
	w = httptest.NewRecorder()

	authHandler.Logout(w, req)

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
	projectsHandler := handler.NewProjectsHandler(projectRepo)

	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.PostForm = map[string][]string{
		"name": {"Integration Project"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	projectsHandler.Create(w, req)

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
