package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSignupRepository(t *testing.T) {
	db := &Connection{}
	repo := NewSignupRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestSignupRepository_Structure(t *testing.T) {
	repo := &SignupRepository{
		db: nil,
	}

	assert.NotNil(t, repo)
	assert.Nil(t, repo.db)
}
