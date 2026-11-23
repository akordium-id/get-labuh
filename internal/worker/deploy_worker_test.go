package worker_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/models"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
	"github.com/akordium-id/get-labuh/internal/worker"
)

func TestDeployWorker_EnvResolverIntegration(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 1. Setup project & team & environment
	teamID := "t1"
	projID := "p1"
	envID := "e1"
	_, err := db.ExecContext(ctx, "INSERT INTO teams (id, name) VALUES (?, ?)", teamID, "DevOps")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO projects (id, name, slug, team_id) VALUES (?, ?, ?, ?)", projID, "Proj 1", "proj-1", teamID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO environments (id, project_id, name, slug) VALUES (?, ?, ?, ?)", envID, projID, "Prod", "prod")
	require.NoError(t, err)

	sharedRepo := repo.NewSharedEnvVarRepo(db)
	appEnvRepo := repo.NewEnvVarRepo(db)

	// Set Project Shared Env: DB_HOST=postgres.local
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeProject, ScopeID: projID, Key: "DB_HOST", Value: "postgres.local",
	}))

	// Set Team Shared Env: REGISTRY=ghcr.io
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeTeam, ScopeID: teamID, Key: "REGISTRY", Value: "ghcr.io",
	}))

	resolver := deploy.NewEnvResolver(sharedRepo, appEnvRepo, db)

	w := worker.NewDeployWorker(10)
	w.SetEnvResolver(resolver)

	app := &models.Application{
		ID:            "app-1",
		Name:          "App 1",
		EnvironmentID: envID,
		AppPort:       3000,
	}

	job, err := worker.BuildDeploymentJobWithResolver(ctx, app, "dep-1", resolver)
	require.NoError(t, err)

	envMap := make(map[string]string)
	for _, ev := range job.EnvVars {
		envMap[ev.Key] = ev.Value
	}

	assert.Equal(t, "postgres.local", envMap["DB_HOST"])
	assert.Equal(t, "ghcr.io", envMap["REGISTRY"])
}
