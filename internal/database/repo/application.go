package repo

import (
	"database/sql"

	"github.com/akordium-id/get-labuh/internal/modules/applications"
)

// ApplicationRepo adalah alias backward-compatible menuju applications.Repo.
type ApplicationRepo = applications.Repo

// NewApplicationRepo membuat instance baru Repo aplikasi.
func NewApplicationRepo(db *sql.DB) *ApplicationRepo {
	return applications.NewRepo(db)
}

// GenerateRandomSecret adalah alias fungsi pembuat random secret.
func GenerateRandomSecret(byteLen int) (string, error) {
	return applications.GenerateRandomSecret(byteLen)
}
