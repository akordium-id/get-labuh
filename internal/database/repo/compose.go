package repo

import (
	"database/sql"

	"github.com/akordium-id/get-labuh/internal/modules/compose"
)

// ComposeRepo adalah type alias ke internal/modules/compose untuk kompatibilitas ke belakang.
type ComposeRepo = compose.Repo

func NewComposeRepo(db *sql.DB) *ComposeRepo {
	return compose.NewRepo(db)
}
