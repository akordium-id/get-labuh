package models

import (
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUserStruct(t *stdtesting.T) {
	now := time.Now()
	user := &User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hash",
		Name:         "Test",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.Equal(t, "user-1", user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "hash", user.PasswordHash)
	assert.Equal(t, "Test", user.Name)
}

func TestCreateUserInput(t *stdtesting.T) {
	input := CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: "hash",
		Name:         "Test",
	}

	assert.Equal(t, "test@example.com", input.Email)
	assert.Equal(t, "hash", input.PasswordHash)
	assert.Equal(t, "Test", input.Name)
}

func TestUserFields(t *stdtesting.T) {
	user := &User{}
	assert.Empty(t, user.ID)
	assert.Empty(t, user.Email)
	assert.Empty(t, user.PasswordHash)
	assert.Empty(t, user.Name)
	assert.True(t, user.CreatedAt.IsZero())
	assert.True(t, user.UpdatedAt.IsZero())
}
