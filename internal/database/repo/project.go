package repo

import (
	"database/sql"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
)

// ProjectRepo adalah type alias ke internal/modules/projects untuk kompatibilitas ke belakang.
type ProjectRepo = projects.ProjectRepo

func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return projects.NewProjectRepo(db)
}

// DeployKeyRepo adalah type alias ke internal/modules/projects untuk kompatibilitas ke belakang.
type DeployKeyRepo = projects.DeployKeyRepo

func NewDeployKeyRepo(db *sql.DB) *DeployKeyRepo {
	return projects.NewDeployKeyRepo(db)
}

// TeamRepo adalah type alias ke internal/modules/projects untuk kompatibilitas ke belakang.
type TeamRepo = projects.TeamRepo

func NewTeamRepo(db *sql.DB) *TeamRepo {
	return projects.NewTeamRepo(db)
}

func GenerateSlug(name string) string {
	return projects.GenerateSlug(name)
}

func HasRole(userRole, requiredRole models.TeamRole) bool {
	return projects.HasRole(userRole, requiredRole)
}
