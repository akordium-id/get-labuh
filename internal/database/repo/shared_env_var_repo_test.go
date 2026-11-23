package repo_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/models"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestSharedEnvVarRepo_CRUD(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	_ = os.Setenv("TEST_MASTER_KEY", "super-secret-master-key-123456789")
	_ = encryption.Init("TEST_MASTER_KEY")

	r := repo.NewSharedEnvVarRepo(db)
	ctx := context.Background()

	// 1. Set item
	item := &models.SharedEnvVar{
		Scope:    models.ScopeProject,
		ScopeID:  "proj-123",
		Key:      "SHARED_SECRET",
		Value:    "super-secret-token",
		IsSecret: true,
	}
	err := r.Set(ctx, item)
	require.NoError(t, err)

	// 2. List & Verify Decryption
	list, err := r.ListByScope(ctx, models.ScopeProject, "proj-123")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "SHARED_SECRET", list[0].Key)
	assert.Equal(t, "super-secret-token", list[0].Value)
	assert.True(t, list[0].IsEncrypted)

	// 3. Upsert update
	item.Value = "updated-token"
	err = r.Set(ctx, item)
	require.NoError(t, err)

	updated, err := r.GetByKey(ctx, models.ScopeProject, "proj-123", "SHARED_SECRET")
	require.NoError(t, err)
	assert.Equal(t, "updated-token", updated.Value)

	// 4. Bulk Upsert
	bulk := []*models.SharedEnvVar{
		{Key: "K1", Value: "V1"},
		{Key: "K2", Value: "V2", IsSecret: true},
	}
	err = r.BulkUpsert(ctx, models.ScopeProject, "proj-123", bulk)
	require.NoError(t, err)

	listAll, err := r.ListByScope(ctx, models.ScopeProject, "proj-123")
	require.NoError(t, err)
	assert.Len(t, listAll, 3) // SHARED_SECRET, K1, K2

	// 5. DeleteByKey
	err = r.DeleteByKey(ctx, models.ScopeProject, "proj-123", "SHARED_SECRET")
	require.NoError(t, err)

	notFound, err := r.GetByKey(ctx, models.ScopeProject, "proj-123", "SHARED_SECRET")
	assert.NoError(t, err)
	assert.Nil(t, notFound)
}
