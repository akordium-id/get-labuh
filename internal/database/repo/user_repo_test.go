package repo

import (
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/stretchr/testify/assert"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestUserRepo_Create(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	userRepo := NewUserRepo(db)
	user, err := userRepo.Create(models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: "hash",
		Name:         "Test User",
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "hash", user.PasswordHash)
	assert.Equal(t, "Test User", user.Name)
}

func TestUserRepo_GetByID(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	userRepo := NewUserRepo(db)
	created, _ := userRepo.Create(models.CreateUserInput{
		Email:        "getbyid@example.com",
		PasswordHash: "hash",
		Name:         "Get By ID",
	})

	found, err := userRepo.GetByID(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "getbyid@example.com", found.Email)
}

func TestUserRepo_GetByEmail(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	userRepo := NewUserRepo(db)
	created, _ := userRepo.Create(models.CreateUserInput{
		Email:        "getbyemail@example.com",
		PasswordHash: "hash",
		Name:         "Get By Email",
	})

	found, err := userRepo.GetByEmail(created.Email)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "getbyemail@example.com", found.Email)
}

func TestUserRepo_GetAll(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	userRepo := NewUserRepo(db)
	userRepo.Create(models.CreateUserInput{Email: "user1@example.com", PasswordHash: "h1", Name: "U1"})
	userRepo.Create(models.CreateUserInput{Email: "user2@example.com", PasswordHash: "h2", Name: "U2"})

	users, err := userRepo.GetAll()
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserRepo_DuplicateEmail(t *stdtesting.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	userRepo := NewUserRepo(db)
	userRepo.Create(models.CreateUserInput{Email: "dup@example.com", PasswordHash: "h1", Name: "U1"})

	_, err := userRepo.Create(models.CreateUserInput{Email: "dup@example.com", PasswordHash: "h2", Name: "U2"})
	assert.Error(t, err)
}
