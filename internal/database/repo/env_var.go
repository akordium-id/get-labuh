package repo

import (
	"database/sql"

	"github.com/akordium-id/get-labuh/internal/modules/applications"
)

// EnvVarRepo adalah alias backward-compatible menuju applications.EnvVarRepo.
type EnvVarRepo = applications.EnvVarRepo

// NewEnvVarRepo membuat instance baru EnvVarRepo aplikasi.
func NewEnvVarRepo(db *sql.DB) *EnvVarRepo {
	return applications.NewEnvVarRepo(db)
}
