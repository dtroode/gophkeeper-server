package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLoginRepository(t *testing.T) {
	db := &Connection{}
	repo := NewLoginRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestLoginRepository_Structure(t *testing.T) {
	repo := &LoginRepository{
		db: nil,
	}

	assert.NotNil(t, repo)
	assert.Nil(t, repo.db)
}
