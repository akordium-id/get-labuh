package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/akordium-id/get-labuh/internal/models"
)

// SharedEnvVarProvider mendefinisikan antarmuka pembacaan shared env vars.
type SharedEnvVarProvider interface {
	ListByScope(ctx context.Context, scope models.SharedEnvVarScope, scopeID string) ([]*models.SharedEnvVar, error)
}

// AppEnvVarProvider mendefinisikan antarmuka pembacaan app env vars.
type AppEnvVarProvider interface {
	GetByApplicationIDWithContext(ctx context.Context, appID string) ([]*models.AppEnvVar, error)
}

// EnvResolver bertanggung jawab melakukan cascading inheritance variabel lingkungan
// dengan urutan prioritas: Application > Project > Team.
type EnvResolver struct {
	sharedRepo SharedEnvVarProvider
	appEnvRepo AppEnvVarProvider
	db         *sql.DB
}

// NewEnvResolver membuat instance baru EnvResolver.
func NewEnvResolver(sharedRepo SharedEnvVarProvider, appEnvRepo AppEnvVarProvider, db *sql.DB) *EnvResolver {
	return &EnvResolver{
		sharedRepo: sharedRepo,
		appEnvRepo: appEnvRepo,
		db:         db,
	}
}

// ResolveDetailed menghasilkan daftar variabel gabungan lengkap dengan pelabelan asal (Source).
func (r *EnvResolver) ResolveDetailed(ctx context.Context, app *models.Application) ([]models.ResolvedEnvVar, error) {
	resolvedMap := make(map[string]models.ResolvedEnvVar)

	var projectID string
	var teamID string

	// 1. Dapatkan project_id dan team_id dari hierarki environment jika ada db
	if r.db != nil && app.EnvironmentID != "" {
		query := `
			SELECT e.project_id, COALESCE(p.team_id, '')
			FROM environments e
			LEFT JOIN projects p ON e.project_id = p.id
			WHERE e.id = ?
			LIMIT 1;
		`
		var tID string
		err := r.db.QueryRowContext(ctx, query, app.EnvironmentID).Scan(&projectID, &tID)
		if err == nil {
			teamID = tID
		}
	}

	// 2. Load Team Scope Variables
	if teamID != "" && r.sharedRepo != nil {
		teamVars, err := r.sharedRepo.ListByScope(ctx, models.ScopeTeam, teamID)
		if err != nil {
			return nil, fmt.Errorf("gagal memuat team shared env vars: %w", err)
		}
		for _, v := range teamVars {
			resolvedMap[v.Key] = models.ResolvedEnvVar{
				Key:      v.Key,
				Value:    v.Value,
				IsSecret: v.IsSecret,
				Source:   "team",
			}
		}
	}

	// 3. Load Project Scope Variables (overwrites team)
	if projectID != "" && r.sharedRepo != nil {
		projVars, err := r.sharedRepo.ListByScope(ctx, models.ScopeProject, projectID)
		if err != nil {
			return nil, fmt.Errorf("gagal memuat project shared env vars: %w", err)
		}
		for _, v := range projVars {
			resolvedMap[v.Key] = models.ResolvedEnvVar{
				Key:      v.Key,
				Value:    v.Value,
				IsSecret: v.IsSecret,
				Source:   "project",
			}
		}
	}

	// 4. Load Application Scope Variables (overwrites project and team)
	if app.ID != "" && r.appEnvRepo != nil {
		appVars, err := r.appEnvRepo.GetByApplicationIDWithContext(ctx, app.ID)
		if err != nil {
			return nil, fmt.Errorf("gagal memuat application env vars: %w", err)
		}
		for _, v := range appVars {
			resolvedMap[v.Key] = models.ResolvedEnvVar{
				Key:      v.Key,
				Value:    v.Value,
				IsSecret: v.IsSecret,
				Source:   "app",
			}
		}
	}

	// 5. Urutkan berdasarkan Key secara deterministik
	keys := make([]string, 0, len(resolvedMap))
	for k := range resolvedMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]models.ResolvedEnvVar, 0, len(keys))
	for _, k := range keys {
		result = append(result, resolvedMap[k])
	}

	return result, nil
}

// ResolveForApplication menghasilkan []models.AppEnvVar yang siap diinjeksikan ke Docker container.
func (r *EnvResolver) ResolveForApplication(ctx context.Context, app *models.Application) ([]models.AppEnvVar, error) {
	detailed, err := r.ResolveDetailed(ctx, app)
	if err != nil {
		return nil, err
	}

	result := make([]models.AppEnvVar, 0, len(detailed))
	for _, d := range detailed {
		result = append(result, models.AppEnvVar{
			ApplicationID: app.ID,
			Key:           d.Key,
			Value:         d.Value,
			IsSecret:      d.IsSecret,
		})
	}

	return result, nil
}
