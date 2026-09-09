package repo

import (
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/models"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
	"github.com/stretchr/testify/assert"
)

func TestApplicationRepo_Create(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Test App",
		Slug:          "test-app",
		SourceType:    models.SourceTypeGit,
		AppPort:       8080,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, app.ID)
	assert.Equal(t, "env-1", app.EnvironmentID)
	assert.Equal(t, "Test App", app.Name)
	assert.Equal(t, "test-app", app.Slug)
	assert.Equal(t, models.SourceTypeGit, app.SourceType)
	assert.Equal(t, models.AppStatusIdle, app.Status)
	assert.Equal(t, 8080, app.AppPort)
	assert.NotNil(t, app.ContainerName)
}

func TestApplicationRepo_GetByID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Get App",
		Slug:          "get-app",
		SourceType:    models.SourceTypeDockerfile,
		AppPort:       3000,
	})

	found, err := appRepo.GetByID(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Get App", found.Name)
	assert.Equal(t, models.SourceTypeDockerfile, found.SourceType)
}

func TestApplicationRepo_GetBySlug(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Slug App",
		Slug:          "slug-app",
		SourceType:    models.SourceTypeGit,
	})

	found, err := appRepo.GetBySlug(created.EnvironmentID, created.Slug)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestApplicationRepo_GetByContainerName(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Container App",
		Slug:          "container-app",
		SourceType:    models.SourceTypeGit,
	})

	found, err := appRepo.GetByContainerName(*created.ContainerName)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestApplicationRepo_GetByEnvironmentID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	appRepo.Create(models.CreateApplicationInput{EnvironmentID: "env-1", Name: "A1", Slug: "a1", SourceType: models.SourceTypeGit})
	appRepo.Create(models.CreateApplicationInput{EnvironmentID: "env-1", Name: "A2", Slug: "a2", SourceType: models.SourceTypeGit})
	appRepo.Create(models.CreateApplicationInput{EnvironmentID: "env-2", Name: "A3", Slug: "a3", SourceType: models.SourceTypeGit})

	apps, err := appRepo.GetByEnvironmentID("env-1")
	assert.NoError(t, err)
	assert.Len(t, apps, 2)
}

func TestApplicationRepo_UpdateStatus(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Status App",
		Slug:          "status-app",
		SourceType:    models.SourceTypeGit,
	})

	err := appRepo.UpdateStatus(created.ID, models.AppStatusRunning)
	assert.NoError(t, err)

	updated, _ := appRepo.GetByID(created.ID)
	assert.Equal(t, models.AppStatusRunning, updated.Status)
}

func TestApplicationRepo_UpdateContainer(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Container Update",
		Slug:          "container-update",
		SourceType:    models.SourceTypeGit,
	})

	err := appRepo.UpdateContainer(created.ID, "container-123")
	assert.NoError(t, err)

	updated, _ := appRepo.GetByID(created.ID)
	assert.Equal(t, "container-123", *updated.ContainerID)
}

func TestApplicationRepo_Delete(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Delete App",
		Slug:          "delete-app",
		SourceType:    models.SourceTypeGit,
	})

	err := appRepo.Delete(created.ID)
	assert.NoError(t, err)

	_, err = appRepo.GetByID(created.ID)
	assert.Error(t, err)
}

func TestApplicationRepo_Clone(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	src, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Original",
		Slug:          "original",
		SourceType:    models.SourceTypeGit,
		RepositoryURL: new("https://github.com/test/repo"),
	})

	cloned, err := appRepo.Clone(src, "env-2", "Cloned App")
	assert.NoError(t, err)
	assert.NotEmpty(t, cloned.ID)
	assert.Equal(t, "env-2", cloned.EnvironmentID)
	assert.Equal(t, "Cloned App", cloned.Name)
	assert.Equal(t, "cloned-app", cloned.Slug)
	assert.Equal(t, models.SourceTypeGit, cloned.SourceType)
	assert.Equal(t, *src.RepositoryURL, *cloned.RepositoryURL)
}

func TestApplicationRepo_Move(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Move App",
		Slug:          "move-app",
		SourceType:    models.SourceTypeGit,
	})

	err := appRepo.Move(created.ID, "env-2")
	assert.NoError(t, err)

	updated, _ := appRepo.GetByID(created.ID)
	assert.Equal(t, "env-2", updated.EnvironmentID)
}

func TestApplicationRepo_UpdateCustomDomain(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	appRepo := NewApplicationRepo(db)
	created, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Domain App",
		Slug:          "domain-app",
		SourceType:    models.SourceTypeGit,
	})

	domain := "example.com"
	err := appRepo.UpdateCustomDomain(created.ID, &domain)
	assert.NoError(t, err)

	updated, _ := appRepo.GetByID(created.ID)
	assert.NotNil(t, updated.CustomDomain)
	assert.Equal(t, "example.com", *updated.CustomDomain)
}
