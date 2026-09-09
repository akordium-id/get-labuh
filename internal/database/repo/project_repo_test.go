package repo

import (
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/models"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
	"github.com/stretchr/testify/assert"
)

//go:fix inline
func strPtr(s string) *string {
	return new(s)
}

func TestProjectRepo_Create(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	project, err := projectRepo.Create(models.CreateProjectInput{
		Name:        "Test Project",
		Slug:        "test-project",
		Description: new("desc"),
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, project.ID)
	assert.Equal(t, "Test Project", project.Name)
	assert.Equal(t, "test-project", project.Slug)
}

func TestProjectRepo_GetByID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	created, _ := projectRepo.Create(models.CreateProjectInput{
		Name: "Get By ID Project",
		Slug: "get-by-id",
	})

	found, err := projectRepo.GetByID(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "Get By ID Project", found.Name)
}

func TestProjectRepo_GetBySlug(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	created, _ := projectRepo.Create(models.CreateProjectInput{
		Name: "Get By Slug Project",
		Slug: "get-by-slug",
	})

	found, err := projectRepo.GetBySlug(created.Slug)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "get-by-slug", found.Slug)
}

func TestProjectRepo_GetAll(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	projectRepo.Create(models.CreateProjectInput{Name: "P1", Slug: "p1"})
	projectRepo.Create(models.CreateProjectInput{Name: "P2", Slug: "p2"})

	projects, err := projectRepo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, projects, 2)
}

func TestProjectRepo_Delete(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	created, _ := projectRepo.Create(models.CreateProjectInput{Name: "Delete Me", Slug: "delete-me"})

	err := projectRepo.Delete(created.ID)
	assert.NoError(t, err)

	_, err = projectRepo.GetByID(created.ID)
	assert.Error(t, err)
}

func TestProjectRepo_CreateEnvironment(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "Env Project", Slug: "env-project"})

	env, err := projectRepo.CreateEnvironment(project.ID, "Staging", "staging")
	assert.NoError(t, err)
	assert.NotEmpty(t, env.ID)
	assert.Equal(t, project.ID, env.ProjectID)
	assert.Equal(t, "Staging", env.Name)
	assert.Equal(t, "staging", env.Slug)
}

func TestProjectRepo_GetEnvironmentsByProjectID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "Env Project", Slug: "env-project"})
	projectRepo.CreateEnvironment(project.ID, "Staging", "staging")
	projectRepo.CreateEnvironment(project.ID, "Production", "production")

	envs, err := projectRepo.GetEnvironmentsByProjectID(project.ID)
	assert.NoError(t, err)
	assert.Len(t, envs, 2)
}

func TestProjectRepo_DuplicateSlug(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	projectRepo := NewProjectRepo(db)
	projectRepo.Create(models.CreateProjectInput{Name: "P1", Slug: "dup-slug"})

	_, err := projectRepo.Create(models.CreateProjectInput{Name: "P2", Slug: "dup-slug"})
	assert.Error(t, err)
}
