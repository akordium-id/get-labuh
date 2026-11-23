package settings_test

import (
	"context"
	"os"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/settings"
)

func TestRegistryRepo(t *stdtesting.T) {
	os.Setenv("LABUH_MASTER_KEY", "test-master-key-for-registries-32b")
	err := encryption.Init("LABUH_MASTER_KEY")
	require.NoError(t, err)

	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	repo := settings.NewRegistryRepo(db)
	ctx := context.Background()

	// 1. Create
	input := models.CreateDockerRegistryInput{
		Name:          "GitHub Packages",
		ServerAddress: "ghcr.io",
		Username:      "faiq-akordium",
		Password:      "ghp_secrettoken12345",
		Email:         "faiq@example.com",
	}

	reg, err := repo.Create(ctx, input)
	require.NoError(t, err)
	require.NotEmpty(t, reg.ID)
	assert.Equal(t, "GitHub Packages", reg.Name)
	assert.Equal(t, "ghcr.io", reg.ServerAddress)
	assert.Equal(t, "faiq-akordium", reg.Username)
	assert.Equal(t, "ghp_secrettoken12345", reg.Password)

	// Verify that password in DB is encrypted
	var rawPassword string
	err = db.QueryRow("SELECT password FROM docker_registries WHERE id = ?", reg.ID).Scan(&rawPassword)
	require.NoError(t, err)
	assert.True(t, encryption.IsEncrypted(rawPassword), "DB password should have ENC: prefix")
	assert.NotEqual(t, "ghp_secrettoken12345", rawPassword)

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, reg.ID)
	require.NoError(t, err)
	assert.Equal(t, reg.ID, fetched.ID)
	assert.Equal(t, "ghp_secrettoken12345", fetched.Password, "GetByID should return decrypted password")

	// 3. List
	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, reg.ID, list[0].ID)

	// 4. Update
	newPass := "ghp_newtoken98765"
	updateInput := models.UpdateDockerRegistryInput{
		ID:            reg.ID,
		Name:          "GHCR Production",
		ServerAddress: "ghcr.io",
		Username:      "faiq-updated",
		Password:      &newPass,
		Email:         "faiq-new@example.com",
	}
	err = repo.Update(ctx, updateInput)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, reg.ID)
	require.NoError(t, err)
	assert.Equal(t, "GHCR Production", updated.Name)
	assert.Equal(t, "faiq-updated", updated.Username)
	assert.Equal(t, "ghp_newtoken98765", updated.Password)

	// Update without changing password
	updateWithoutPass := models.UpdateDockerRegistryInput{
		ID:            reg.ID,
		Name:          "GHCR Renamed",
		ServerAddress: "ghcr.io",
		Username:      "faiq-updated",
		Password:      nil,
		Email:         "faiq-new@example.com",
	}
	err = repo.Update(ctx, updateWithoutPass)
	require.NoError(t, err)

	updated2, err := repo.GetByID(ctx, reg.ID)
	require.NoError(t, err)
	assert.Equal(t, "GHCR Renamed", updated2.Name)
	assert.Equal(t, "ghp_newtoken98765", updated2.Password, "Password should remain unchanged when nil")

	// 5. Delete
	err = repo.Delete(ctx, reg.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, reg.ID)
	assert.Error(t, err, "Should return error after deletion")
}
