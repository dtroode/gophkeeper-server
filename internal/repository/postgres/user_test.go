package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUserRepository(t *testing.T) {
	db := &Connection{}
	repo := NewUserRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestUserRepository_Structure(t *testing.T) {
	repo := &UserRepository{
		db: nil,
	}

	assert.NotNil(t, repo)
	assert.Nil(t, repo.db)
}
