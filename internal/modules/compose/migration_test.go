package compose_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestMigration033_ComposeRawYAML(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Verify columns raw_yaml, parsed_services, last_deployed_at, error_message exist
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(compose_applications);")
	require.NoError(t, err)
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notNull   int
			dfltValue interface{}
			pk        int
		)
		err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk)
		require.NoError(t, err)
		columns[name] = true
	}

	assert.True(t, columns["raw_yaml"], "expected column raw_yaml to exist")
	assert.True(t, columns["parsed_services"], "expected column parsed_services to exist")
	assert.True(t, columns["last_deployed_at"], "expected column last_deployed_at to exist")
	assert.True(t, columns["error_message"], "expected column error_message to exist")
}
