package repo

import (
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/stretchr/testify/assert"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestDeploymentRepo_Create(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	abc123 := "abc123"
	clone := "clone"
	cloning := "cloning..."
	deployment, err := deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: "app-1",
		CommitHash:    &abc123,
		Step:          &clone,
		Output:        &cloning,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, deployment.ID)
	assert.Equal(t, "app-1", deployment.ApplicationID)
	assert.Equal(t, models.DeployStatusQueued, deployment.Status)
	assert.Equal(t, "abc123", *deployment.CommitHash)
	assert.Equal(t, "clone", *deployment.Step)
	assert.Equal(t, "cloning...", *deployment.Output)
}

func TestDeploymentRepo_GetByID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	created, _ := deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: "app-1",
	})

	found, err := deployRepo.GetByID(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "app-1", found.ApplicationID)
}

func TestDeploymentRepo_GetByApplicationID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})
	deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})
	deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-2"})

	deployments, err := deployRepo.GetByApplicationID("app-1")
	assert.NoError(t, err)
	assert.Len(t, deployments, 2)
}

func TestDeploymentRepo_UpdateStatus(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	created, _ := deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})

	err := deployRepo.UpdateStatus(created.ID, models.DeployStatusSuccess)
	assert.NoError(t, err)

	updated, _ := deployRepo.GetByID(created.ID)
	assert.Equal(t, models.DeployStatusSuccess, updated.Status)
}

func TestDeploymentRepo_UpdateStatusWithError(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	created, _ := deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})

	err := deployRepo.UpdateStatusWithError(created.ID, models.DeployStatusFailed, "build failed")
	assert.NoError(t, err)

	updated, _ := deployRepo.GetByID(created.ID)
	assert.Equal(t, models.DeployStatusFailed, updated.Status)
	assert.Equal(t, "build failed", *updated.ErrorMessage)
}

func TestDeploymentRepo_UpdateStep(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	created, _ := deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})

	err := deployRepo.UpdateStep(created.ID, "build")
	assert.NoError(t, err)

	updated, _ := deployRepo.GetByID(created.ID)
	assert.Equal(t, "build", *updated.Step)
}

func TestDeploymentRepo_UpdateOutput(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	created, _ := deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})

	err := deployRepo.UpdateOutput(created.ID, "build output here")
	assert.NoError(t, err)

	updated, _ := deployRepo.GetByID(created.ID)
	assert.Equal(t, "build output here", *updated.Output)
}

func TestDeploymentRepo_MarkStarted(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	created, _ := deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})

	err := deployRepo.MarkStarted(created.ID)
	assert.NoError(t, err)

	updated, _ := deployRepo.GetByID(created.ID)
	assert.NotNil(t, updated.StartedAt)
}

func TestDeploymentRepo_MarkFinished(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	deployRepo := NewDeploymentRepo(db)
	created, _ := deployRepo.Create(models.CreateDeploymentInput{ApplicationID: "app-1"})

	err := deployRepo.MarkFinished(created.ID)
	assert.NoError(t, err)

	updated, _ := deployRepo.GetByID(created.ID)
	assert.NotNil(t, updated.FinishedAt)
}

func TestDeploymentRepo_GetByProjectID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	appRepo := NewApplicationRepo(db)
	deployRepo := NewDeploymentRepo(db)

	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app1, _ := appRepo.Create(models.CreateApplicationInput{EnvironmentID: env.ID, Name: "A1", Slug: "a1", SourceType: models.SourceTypeGit})
	app2, _ := appRepo.Create(models.CreateApplicationInput{EnvironmentID: env.ID, Name: "A2", Slug: "a2", SourceType: models.SourceTypeGit})

	deployRepo.Create(models.CreateDeploymentInput{ApplicationID: app1.ID})
	deployRepo.Create(models.CreateDeploymentInput{ApplicationID: app2.ID})

	deployments, err := deployRepo.GetByProjectID(project.ID)
	assert.NoError(t, err)
	assert.Len(t, deployments, 2)
}
