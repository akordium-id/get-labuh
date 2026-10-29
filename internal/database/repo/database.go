package repo

import (
	"database/sql"

	"github.com/akordium-id/get-labuh/internal/modules/databases"
)

// DatabaseRepo adalah alias ke internal/modules/databases untuk kompatibilitas ke belakang.
type DatabaseRepo = databases.ServiceRepo

func NewDatabaseRepo(db *sql.DB) *DatabaseRepo {
	return databases.NewServiceRepo(db)
}
