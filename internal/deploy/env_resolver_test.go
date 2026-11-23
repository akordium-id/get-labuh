package deploy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/models"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestEnvResolver_CascadingPrecedence(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 1. Setup Team, Project, and Environment
	teamID := "team-1"
	projectID := "proj-alpha"
	envID := "env-alpha"

	_, err := db.ExecContext(ctx, "INSERT INTO teams (id, name) VALUES (?, ?)", teamID, "Engineering")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO projects (id, name, slug, team_id) VALUES (?, ?, ?, ?)",
		projectID, "Alpha", "alpha", teamID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO environments (id, project_id, name, slug) VALUES (?, ?, ?, ?)",
		envID, projectID, "Production", "production")
	require.NoError(t, err)

	sharedRepo := repo.NewSharedEnvVarRepo(db)
	appEnvRepo := repo.NewEnvVarRepo(db)

	// 2. Team Level: FOO=team_foo, SHARED_ALL=team_shared
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeTeam, ScopeID: teamID, Key: "FOO", Value: "team_foo",
	}))
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeTeam, ScopeID: teamID, Key: "SHARED_ALL", Value: "team_shared",
	}))

	// 3. Project Level: FOO=project_foo (overrides team), PROJ_ONLY=project_val
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeProject, ScopeID: projectID, Key: "FOO", Value: "project_foo",
	}))
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeProject, ScopeID: projectID, Key: "PROJ_ONLY", Value: "project_val",
	}))

	// 4. App Level: FOO=app_foo (overrides project & team), APP_ONLY=app_val
	appID := "app-web"
	_, err = db.ExecContext(ctx, `
		INSERT INTO applications (id, name, slug, environment_id, source_type, app_port, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		appID, "Web App", "web-app", envID, models.SourceTypeGit, 8080, models.AppStatusIdle,
	)
	require.NoError(t, err)

	_, err = appEnvRepo.CreateWithContext(ctx, models.CreateAppEnvVarInput{
		ApplicationID: appID, Key: "FOO", Value: "app_foo", IsSecret: false,
	})
	require.NoError(t, err)

	_, err = appEnvRepo.CreateWithContext(ctx, models.CreateAppEnvVarInput{
		ApplicationID: appID, Key: "APP_ONLY", Value: "app_val", IsSecret: false,
	})
	require.NoError(t, err)

	resolver := deploy.NewEnvResolver(sharedRepo, appEnvRepo, db)

	app := &models.Application{
		ID:            appID,
		Name:          "Web App",
		EnvironmentID: envID,
	}

	// 5. Test ResolveForApplication
	merged, err := resolver.ResolveForApplication(ctx, app)
	require.NoError(t, err)

	resMap := make(map[string]string)
	for _, v := range merged {
		resMap[v.Key] = v.Value
	}

	assert.Equal(t, "app_foo", resMap["FOO"])            // App overrides Project & Team
	assert.Equal(t, "project_val", resMap["PROJ_ONLY"])   // Inherited from Project
	assert.Equal(t, "team_shared", resMap["SHARED_ALL"]) // Inherited from Team
	assert.Equal(t, "app_val", resMap["APP_ONLY"])       // App specific

	// 6. Test ResolveDetailed with sources
	detailed, err := resolver.ResolveDetailed(ctx, app)
	require.NoError(t, err)
	sourceMap := make(map[string]string)
	for _, d := range detailed {
		sourceMap[d.Key] = d.Source
	}
	assert.Equal(t, "app", sourceMap["FOO"])
	assert.Equal(t, "project", sourceMap["PROJ_ONLY"])
	assert.Equal(t, "team", sourceMap["SHARED_ALL"])
	assert.Equal(t, "app", sourceMap["APP_ONLY"])
}
