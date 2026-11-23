package projects_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	projectsmod "github.com/akordium-id/get-labuh/internal/modules/projects"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestProjectsHandler_SharedEnvEndpoints(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	projectRepo := projectsmod.NewProjectRepo(db)
	sharedRepo := repo.NewSharedEnvVarRepo(db)

	proj, err := projectRepo.CreateWithContext(ctx, models.CreateProjectInput{
		Name: "Test Project",
		Slug: "test-proj",
	})
	require.NoError(t, err)

	mod := projectsmod.New(db, nil)
	mod.SetSharedEnvRepo(sharedRepo)

	r := chi.NewRouter()
	mod.RegisterRoutes(r)

	// 1. POST /projects/{id}/env - Set single variable
	form := url.Values{}
	form.Set("key", "API_BASE_URL")
	form.Set("value", "https://api.internal")
	form.Set("is_secret", "false")

	req := httptest.NewRequest(http.MethodPost, "/projects/"+proj.ID+"/env", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)

	// Verify in DB
	list, err := sharedRepo.ListByScope(ctx, models.ScopeProject, proj.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "API_BASE_URL", list[0].Key)
	assert.Equal(t, "https://api.internal", list[0].Value)

	// 2. POST /projects/{id}/env/bulk - Bulk import .env format
	bulkContent := "REDIS_HOST=redis.cluster\n# comment\nJWT_SECRET=supersecret123\n"
	bulkForm := url.Values{}
	bulkForm.Set("env_content", bulkContent)

	reqBulk := httptest.NewRequest(http.MethodPost, "/projects/"+proj.ID+"/env/bulk", strings.NewReader(bulkForm.Encode()))
	reqBulk.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wBulk := httptest.NewRecorder()
	r.ServeHTTP(wBulk, reqBulk)

	assert.Equal(t, http.StatusSeeOther, wBulk.Code)

	listBulk, err := sharedRepo.ListByScope(ctx, models.ScopeProject, proj.ID)
	require.NoError(t, err)
	assert.Len(t, listBulk, 3) // API_BASE_URL, REDIS_HOST, JWT_SECRET

	// 3. POST /projects/{id}/env/delete
	delForm := url.Values{}
	delForm.Set("key", "REDIS_HOST")

	reqDel := httptest.NewRequest(http.MethodPost, "/projects/"+proj.ID+"/env/delete", strings.NewReader(delForm.Encode()))
	reqDel.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, reqDel)

	assert.Equal(t, http.StatusSeeOther, wDel.Code)

	itemAfterDel, err := sharedRepo.GetByKey(ctx, models.ScopeProject, proj.ID, "REDIS_HOST")
	require.NoError(t, err)
	assert.Nil(t, itemAfterDel)

	// 4. GET /projects/{id}/env
	reqGet := httptest.NewRequest(http.MethodGet, "/projects/"+proj.ID+"/env", nil)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)

	assert.Equal(t, http.StatusOK, wGet.Code)
	assert.Contains(t, wGet.Body.String(), "API_BASE_URL")
	assert.Contains(t, wGet.Body.String(), "JWT_SECRET")
}
