package model

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// RecordStore defines persistence operations for records.
type RecordStore interface {
	Create(ctx context.Context, record Record) (Record, error)
	GetByID(ctx context.Context, id uuid.UUID) (Record, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Record, error)
	GetByUserIDAndType(ctx context.Context, userID uuid.UUID, recordType RecordType) ([]Record, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	GetUpdatedAfter(ctx context.Context, userID uuid.UUID, updatedAfter time.Time) ([]Record, error)
	GetUpdatedAfterByType(ctx context.Context, userID uuid.UUID, recordType RecordType, updatedAfter time.Time) ([]Record, error)
	GetDeletedAfter(ctx context.Context, userID uuid.UUID, deletedAfter time.Time) ([]Tombstone, error)
	GetDeletedAfterByType(ctx context.Context, userID uuid.UUID, recordType RecordType, deletedAfter time.Time) ([]Tombstone, error)
}

// Record represents a stored record entity.
type Record struct {
	OwnerID            uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
	Name               string
	Description        string
	EncryptedKey       []byte
	Alg                string
	ID                 uuid.UUID
	Type               RecordType
	EncryptedData      []byte
	S3Key              string
	EncryptedChunkSize int
	RequestID          uuid.UUID
}

// RecordType enumerates record kinds.
type RecordType string

const (
	// RecordTypeLogin is a login/password record.
	RecordTypeLogin RecordType = "login"
	// RecordTypeNote is a text note record.
	RecordTypeNote RecordType = "note"
	// RecordTypeCard is a payment card record.
	RecordTypeCard RecordType = "card"
	// RecordTypeBinary is a binary (streamed) record.
	RecordTypeBinary RecordType = "binary"
)

// RecordMetadata represents metadata returned via APIs.
type RecordMetadata struct {
	ID           string
	Name         string
	Description  string
	Type         RecordType
	EncryptedKey []byte
	Alg          string
	ChunkSize    int
	RequestID    uuid.UUID
}

// CreateRecordStreamRequest represents a client-streamed request item.
type CreateRecordStreamRequest struct {
	Metadata   *RecordMetadata
	DataChunk  []byte
	IsMetadata bool
}

// StreamReader reads CreateRecordStreamRequest items from a stream.
type StreamReader interface {
	Recv() (*CreateRecordStreamRequest, error)
}

// GetRecordStreamResponse represents a server-streamed response item.
type GetRecordStreamResponse struct {
	Metadata    *RecordMetadata
	DataChunk   []byte
	IsLastChunk bool
}

// StreamWriter writes GetRecordStreamResponse items to a stream.
type StreamWriter interface {
	Send(*GetRecordStreamResponse) error
}

// CreateRecordParams contains parameters to create a record.
type CreateRecordParams struct {
	UserID        uuid.UUID
	Name          string
	Description   string
	EncryptedData []byte
	EncryptedKey  []byte
	Alg           string
	Type          RecordType
	RequestID     uuid.UUID
}

// Tombstone marks a deleted record and its timestamp.
type Tombstone struct {
	ID        uuid.UUID
	DeletedAt time.Time
}
