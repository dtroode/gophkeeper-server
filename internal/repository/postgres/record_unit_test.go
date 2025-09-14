package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRecordRepository(t *testing.T) {
	db := &Connection{}
	repo := NewRecordRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestRecordRepository_Structure(t *testing.T) {
	repo := &RecordRepository{
		db: nil,
	}

	assert.NotNil(t, repo)
	assert.Nil(t, repo.db)
}
