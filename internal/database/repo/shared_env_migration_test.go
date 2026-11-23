package repo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestMigration032_SharedEnvVars(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	var count int
	err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM shared_env_vars").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}
