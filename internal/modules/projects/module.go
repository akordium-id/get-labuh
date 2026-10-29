package projects

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
)

// Module membungkus repository dan HTTP handlers untuk domain projects, deploy keys, dan teams.
type Module struct {
	ProjectRepo   *ProjectRepo
	DeployKeyRepo *DeployKeyRepo
	TeamRepo      *TeamRepo
	Handler       *Handler
}

func New(db *sql.DB, userProvider UserProvider) *Module {
	projectRepo := NewProjectRepo(db)
	deployKeyRepo := NewDeployKeyRepo(db)
	teamRepo := NewTeamRepo(db)
	handler := NewHandler(projectRepo, deployKeyRepo, teamRepo, userProvider)

	return &Module{
		ProjectRepo:   projectRepo,
		DeployKeyRepo: deployKeyRepo,
		TeamRepo:      teamRepo,
		Handler:       handler,
	}
}

// RegisterRoutes mendaftarkan rute Chi untuk project list, project create, deploy keys, dan teams.
func (m *Module) RegisterRoutes(r chi.Router) {
	// Projects
	r.Get("/projects", m.Handler.ProjectsList)
	r.Post("/projects", m.Handler.ProjectsCreate)
	r.Get("/projects/create", m.Handler.ProjectsCreatePage)
	r.Get("/projects/{id}", m.Handler.ProjectsGet)
	r.Post("/projects/{id}/delete", m.Handler.ProjectsDelete)

	// Deploy Keys
	r.Get("/projects/{id}/deploy-keys", m.Handler.DeployKeysList)
	r.Post("/projects/{id}/deploy-keys", m.Handler.DeployKeysCreate)
	r.Post("/deploy-keys/{id}/delete", m.Handler.DeployKeysDelete)

	// Teams (UI & Actions)
	r.Get("/settings/teams", m.Handler.TeamsList)
	r.Post("/settings/teams", m.Handler.TeamsCreate)
	r.Get("/teams/{id}", m.Handler.TeamsDetail)
	r.Post("/teams/{id}/members", m.Handler.TeamsAddMember)
	r.Post("/teams/{id}/members/{user_id}/remove", m.Handler.TeamsRemoveMember)
	r.Post("/teams/{id}/members/{user_id}/role", m.Handler.TeamsUpdateMemberRole)
}
