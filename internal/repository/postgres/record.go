package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dtroode/gophkeeper-server/internal/model"
)

var _ model.RecordStore = (*RecordRepository)(nil)

type RecordRepository struct {
	db *Connection
}

func NewRecordRepository(db *Connection) *RecordRepository {
	return &RecordRepository{
		db: db,
	}
}

func (r *RecordRepository) Create(ctx context.Context, record model.Record) (model.Record, error) {
	// Try to insert with request_id; on conflict (owner_id, request_id) return existing row
	query := `
		WITH ins AS (
			INSERT INTO records (id, owner_id, name, description, encrypted_data, s3_key, encrypted_key, alg, type, encrypted_chunk_size, request_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11::uuid, '00000000-0000-0000-0000-000000000000'))
			ON CONFLICT (owner_id, request_id) WHERE request_id IS NOT NULL DO NOTHING
			RETURNING id, name, description, encrypted_data, s3_key, encrypted_key, alg, type, encrypted_chunk_size, created_at, updated_at, deleted_at
		)
		SELECT id, name, description, encrypted_data, s3_key, encrypted_key, alg, type, encrypted_chunk_size, created_at, updated_at, deleted_at
		FROM ins
		UNION ALL
		SELECT r.id, r.name, r.description, r.encrypted_data, r.s3_key, r.encrypted_key, r.alg, r.type, r.encrypted_chunk_size, r.created_at, r.updated_at, r.deleted_at
		FROM records r
		WHERE NOT EXISTS (SELECT 1 FROM ins) AND r.owner_id = $2 AND r.request_id = NULLIF($11::uuid, '00000000-0000-0000-0000-000000000000')
		LIMIT 1`

	var savedRecord model.Record
	err := r.db.QueryRow(ctx, query,
		record.ID, record.OwnerID, record.Name, record.Description,
		record.EncryptedData, record.S3Key, record.EncryptedKey, record.Alg, string(record.Type), record.EncryptedChunkSize,
		record.RequestID,
	).Scan(
		&savedRecord.ID, &savedRecord.Name, &savedRecord.Description,
		&savedRecord.EncryptedData, &savedRecord.S3Key, &savedRecord.EncryptedKey, &savedRecord.Alg,
		&savedRecord.Type, &savedRecord.EncryptedChunkSize, &savedRecord.CreatedAt, &savedRecord.UpdatedAt, &savedRecord.DeletedAt,
	)
	if err != nil {
		return model.Record{}, err
	}

	// Set the owner ID from the input record
	savedRecord.OwnerID = record.OwnerID

	return savedRecord, nil
}

func (r *RecordRepository) GetByID(ctx context.Context, id uuid.UUID) (model.Record, error) {
	query := `
		SELECT r.id, r.owner_id, r.name, r.description, r.encrypted_data, r.s3_key,
		       r.encrypted_key, r.alg, r.type, r.encrypted_chunk_size, r.created_at, r.updated_at, r.deleted_at
		FROM records r
		WHERE r.id = $1 AND r.deleted_at IS NULL`

	var record model.Record
	err := r.db.QueryRow(ctx, query, id).Scan(
		&record.ID, &record.OwnerID, &record.Name, &record.Description,
		&record.EncryptedData, &record.S3Key,
		&record.EncryptedKey, &record.Alg, &record.Type, &record.EncryptedChunkSize,
		&record.CreatedAt, &record.UpdatedAt, &record.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Record{}, model.ErrNotFound
		}
		return model.Record{}, err
	}

	return record, nil
}

func (r *RecordRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const query = `UPDATE records SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	cmd, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *RecordRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Record, error) {
	query := `
		SELECT r.id, r.owner_id, r.name, r.description, r.s3_key,
		       r.encrypted_key, r.alg, r.type, r.encrypted_chunk_size, r.created_at, r.updated_at, r.deleted_at
		FROM records r
		WHERE r.owner_id = $1 AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.Record
	for rows.Next() {
		var record model.Record
		err := rows.Scan(
			&record.ID, &record.OwnerID, &record.Name, &record.Description,
			&record.S3Key,
			&record.EncryptedKey, &record.Alg, &record.Type, &record.EncryptedChunkSize,
			&record.CreatedAt, &record.UpdatedAt, &record.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func (r *RecordRepository) GetByUserIDAndType(ctx context.Context, userID uuid.UUID, recordType model.RecordType) ([]model.Record, error) {
	query := `
		SELECT r.id, r.owner_id, r.name, r.description, r.s3_key,
		       r.encrypted_key, r.alg, r.type, r.encrypted_chunk_size, r.created_at, r.updated_at, r.deleted_at
		FROM records r
		WHERE r.owner_id = $1 AND r.type = $2 AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC`

	rows, err := r.db.Query(ctx, query, userID, string(recordType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.Record
	for rows.Next() {
		var record model.Record
		err := rows.Scan(
			&record.ID, &record.OwnerID, &record.Name, &record.Description,
			&record.S3Key,
			&record.EncryptedKey, &record.Alg, &record.Type, &record.EncryptedChunkSize,
			&record.CreatedAt, &record.UpdatedAt, &record.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func (r *RecordRepository) GetUpdatedAfter(ctx context.Context, userID uuid.UUID, updatedAfter time.Time) ([]model.Record, error) {
	query := `
		SELECT r.id, r.owner_id, r.name, r.description, r.s3_key,
		       r.encrypted_key, r.alg, r.type, r.encrypted_chunk_size, r.created_at, r.updated_at, r.deleted_at
		FROM records r
		WHERE r.owner_id = $1 AND r.deleted_at IS NULL AND r.updated_at > $2
		ORDER BY r.updated_at ASC`

	rows, err := r.db.Query(ctx, query, userID, updatedAfter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.Record
	for rows.Next() {
		var record model.Record
		err := rows.Scan(
			&record.ID, &record.OwnerID, &record.Name, &record.Description,
			&record.S3Key,
			&record.EncryptedKey, &record.Alg, &record.Type, &record.EncryptedChunkSize,
			&record.CreatedAt, &record.UpdatedAt, &record.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *RecordRepository) GetUpdatedAfterByType(ctx context.Context, userID uuid.UUID, recordType model.RecordType, updatedAfter time.Time) ([]model.Record, error) {
	query := `
		SELECT r.id, r.owner_id, r.name, r.description, r.s3_key,
		       r.encrypted_key, r.alg, r.type, r.encrypted_chunk_size, r.created_at, r.updated_at, r.deleted_at
		FROM records r
		WHERE r.owner_id = $1 AND r.type = $2 AND r.deleted_at IS NULL AND r.updated_at > $3
		ORDER BY r.updated_at ASC`

	rows, err := r.db.Query(ctx, query, userID, string(recordType), updatedAfter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.Record
	for rows.Next() {
		var record model.Record
		err := rows.Scan(
			&record.ID, &record.OwnerID, &record.Name, &record.Description,
			&record.S3Key,
			&record.EncryptedKey, &record.Alg, &record.Type, &record.EncryptedChunkSize,
			&record.CreatedAt, &record.UpdatedAt, &record.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *RecordRepository) GetDeletedAfter(ctx context.Context, userID uuid.UUID, deletedAfter time.Time) ([]model.Tombstone, error) {
	query := `
		SELECT id, deleted_at FROM records WHERE owner_id = $1 AND deleted_at IS NOT NULL AND deleted_at > $2
		ORDER BY deleted_at ASC`
	rows, err := r.db.Query(ctx, query, userID, deletedAfter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Tombstone
	for rows.Next() {
		var t model.Tombstone
		if err := rows.Scan(&t.ID, &t.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *RecordRepository) GetDeletedAfterByType(ctx context.Context, userID uuid.UUID, recordType model.RecordType, deletedAfter time.Time) ([]model.Tombstone, error) {
	query := `
		SELECT id, deleted_at FROM records WHERE owner_id = $1 AND type = $2 AND deleted_at IS NOT NULL AND deleted_at > $3
		ORDER BY deleted_at ASC`
	rows, err := r.db.Query(ctx, query, userID, string(recordType), deletedAfter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Tombstone
	for rows.Next() {
		var t model.Tombstone
		if err := rows.Scan(&t.ID, &t.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
