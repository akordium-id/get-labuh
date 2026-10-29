package repo

import (
	"database/sql"

	"github.com/akordium-id/get-labuh/internal/modules/deployments"
)

// DeploymentRepo adalah alias backward-compatible menuju deployments.Repo.
type DeploymentRepo = deployments.Repo

// NewDeploymentRepo membuat instance baru Repo deployments.
func NewDeploymentRepo(db *sql.DB) *DeploymentRepo {
	return deployments.NewRepo(db)
}
