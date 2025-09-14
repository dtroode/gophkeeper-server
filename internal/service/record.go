package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/google/uuid"
)

type Record struct {
	recordStore model.RecordStore
	userStore   model.UserStore
	storage     model.Storage
	logger      *logger.Logger
}

func NewRecord(
	recordStore model.RecordStore,
	userStore model.UserStore,
	storage model.Storage,
	logger *logger.Logger,
) *Record {
	return &Record{
		recordStore: recordStore,
		userStore:   userStore,
		storage:     storage,
		logger:      logger,
	}
}

func (s *Record) CreateRecord(ctx context.Context, params model.CreateRecordParams) (model.Record, error) {
	_, err := s.userStore.GetByID(ctx, params.UserID)
	if errors.Is(err, model.ErrNotFound) {
		return model.Record{}, apiErrors.NewErrUserNotFound(params.UserID.String())
	}
	if err != nil {
		return model.Record{}, fmt.Errorf("failed to get user by id: %w", err)
	}

	record := model.Record{
		ID:            uuid.New(),
		OwnerID:       params.UserID,
		Name:          params.Name,
		Description:   params.Description,
		EncryptedKey:  params.EncryptedKey,
		EncryptedData: params.EncryptedData,
		Alg:           params.Alg,
		Type:          params.Type,
		RequestID:     params.RequestID,
	}

	record, err = s.saveRecord(ctx, record, nil)
	if err != nil {
		return model.Record{}, fmt.Errorf("failed to save record: %w", err)
	}

	return record, nil
}

func (s *Record) GetRecord(ctx context.Context, userID uuid.UUID, recordID uuid.UUID) (model.Record, error) {
	record, err := s.recordStore.GetByID(ctx, recordID)
	if err != nil {
		return model.Record{}, fmt.Errorf("failed to get record by id: %w", err)
	}

	if record.OwnerID != userID {
		return model.Record{}, apiErrors.NewErrRecordNotFound(recordID)
	}

	return record, nil
}

func (s *Record) GetRecords(ctx context.Context, userID uuid.UUID) ([]model.Record, error) {
	records, err := s.recordStore.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get records by user id: %w", err)
	}

	return records, nil
}

func (s *Record) GetLogins(ctx context.Context, userID uuid.UUID) ([]model.Record, error) {
	return s.getRecordsOfType(ctx, userID, model.RecordTypeLogin)
}

func (s *Record) GetNotes(ctx context.Context, userID uuid.UUID) ([]model.Record, error) {
	return s.getRecordsOfType(ctx, userID, model.RecordTypeNote)
}

func (s *Record) GetCards(ctx context.Context, userID uuid.UUID) ([]model.Record, error) {
	return s.getRecordsOfType(ctx, userID, model.RecordTypeCard)
}

func (s *Record) GetBinaries(ctx context.Context, userID uuid.UUID) ([]model.Record, error) {
	return s.getRecordsOfType(ctx, userID, model.RecordTypeBinary)
}

func (s *Record) GetRecordDataStream(ctx context.Context, s3Key string) (io.ReadCloser, error) {
	if s3Key == "" {
		return nil, fmt.Errorf("S3 key is empty")
	}

	reader, err := s.storage.Download(ctx, s3Key)
	if err != nil {
		return nil, fmt.Errorf("failed to download from storage: %w", err)
	}

	return reader, nil
}

func (s *Record) CreateRecordStream(ctx context.Context, userID uuid.UUID, stream model.StreamReader) (model.Record, error) {
	_, err := s.userStore.GetByID(ctx, userID)
	if errors.Is(err, model.ErrNotFound) {
		return model.Record{}, apiErrors.NewErrUserNotFound(userID.String())
	}
	if err != nil {
		return model.Record{}, fmt.Errorf("failed to get user by id: %w", err)
	}

	metadata, err := s.readMetadataFromStream(ctx, stream)
	if err != nil {
		return model.Record{}, fmt.Errorf("failed to read metadata: %w", err)
	}

	if err := s.validateMetadata(metadata); err != nil {
		return model.Record{}, fmt.Errorf("invalid metadata: %w", err)
	}

	dataReader, dataWriter := io.Pipe()

	go func() {
		defer func() {
			if err := dataWriter.Close(); err != nil {
				s.logger.Error("Failed to close data writer", "error", err)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				dataWriter.CloseWithError(fmt.Errorf("context cancelled: %w", ctx.Err()))
				return
			default:
			}

			req, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				dataWriter.CloseWithError(fmt.Errorf("failed to receive from stream: %w", err))
				return
			}

			if req.IsMetadata {
				s.logger.Warn("Received metadata after initial metadata, skipping")
				continue
			}

			if req.DataChunk == nil {
				s.logger.Warn("Received nil data chunk, skipping")
				continue
			}

			if _, err := dataWriter.Write(req.DataChunk); err != nil {
				dataWriter.CloseWithError(fmt.Errorf("failed to write to pipe: %w", err))
				return
			}
		}
	}()

	s3Key := s.generateS3Key(userID)
	if s3Key == "" {
		return model.Record{}, fmt.Errorf("failed to generate S3 key")
	}

	record := model.Record{
		ID:                 uuid.New(),
		OwnerID:            userID,
		Name:               metadata.Name,
		Description:        metadata.Description,
		EncryptedKey:       metadata.EncryptedKey,
		EncryptedData:      nil,
		Alg:                metadata.Alg,
		Type:               metadata.Type,
		S3Key:              s3Key,
		EncryptedChunkSize: metadata.ChunkSize,
		RequestID:          metadata.RequestID,
	}

	record, err = s.saveRecord(ctx, record, dataReader)
	if err != nil {
		return model.Record{}, fmt.Errorf("failed to save record: %w", err)
	}

	return record, nil
}

func (s *Record) StreamRecordToClient(ctx context.Context, userID uuid.UUID, recordID uuid.UUID, stream model.StreamWriter) error {
	if userID == uuid.Nil {
		return fmt.Errorf("userID cannot be nil")
	}
	if recordID == uuid.Nil {
		return fmt.Errorf("recordID cannot be nil")
	}

	record, err := s.GetRecord(ctx, userID, recordID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return apiErrors.NewErrRecordNotFound(recordID)
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	metadata := s.buildRecordMetadata(record)
	if metadata == nil {
		return fmt.Errorf("failed to convert record to metadata")
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before sending metadata: %w", ctx.Err())
	default:
	}

	if err := stream.Send(&model.GetRecordStreamResponse{
		Metadata:    metadata,
		DataChunk:   nil,
		IsLastChunk: false,
	}); err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	if record.S3Key != "" {
		reader, err := s.GetRecordDataStream(ctx, record.S3Key)
		if err != nil {
			return fmt.Errorf("failed to get data stream from S3: %w", err)
		}
		defer reader.Close()

		chunkSize := record.EncryptedChunkSize
		if chunkSize <= 0 {
			return fmt.Errorf("invalid encrypted chunk size: %d", chunkSize)
		}
		buffer := make([]byte, chunkSize)
		totalBytes := int64(0)

		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during streaming: %w", ctx.Err())
			default:
			}

			n, err := io.ReadFull(reader, buffer)
			if err == nil {
				totalBytes += int64(n)
				if sendErr := stream.Send(&model.GetRecordStreamResponse{
					Metadata:    nil,
					DataChunk:   buffer[:n],
					IsLastChunk: false,
				}); sendErr != nil {
					return fmt.Errorf("failed to send data chunk: %w", sendErr)
				}
				continue
			}
			if err == io.EOF && n == 0 {
				break
			}
			if err == io.ErrUnexpectedEOF {
				totalBytes += int64(n)
				if sendErr := stream.Send(&model.GetRecordStreamResponse{
					Metadata:    nil,
					DataChunk:   buffer[:n],
					IsLastChunk: true,
				}); sendErr != nil {
					return fmt.Errorf("failed to send last data chunk: %w", sendErr)
				}
				break
			}
			return fmt.Errorf("error reading from S3 stream: %w", err)
		}

		s.logger.Info("Record service: record stream sent successfully from S3",
			"record_id", record.ID,
			"user_id", userID,
			"s3_key", record.S3Key,
			"bytes_sent", totalBytes)
	} else {
		return fmt.Errorf("no data available for record")
	}

	return nil
}

func (s *Record) DeleteRecord(ctx context.Context, userID uuid.UUID, recordID uuid.UUID) error {
	record, err := s.recordStore.GetByID(ctx, recordID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return apiErrors.NewErrRecordNotFound(recordID)
		}
		return fmt.Errorf("failed to get record: %w", err)
	}
	if record.OwnerID != userID {
		return apiErrors.NewErrRecordNotFound(recordID)
	}
	if record.S3Key != "" {
		if err := s.storage.Delete(ctx, record.S3Key); err != nil {
			s.logger.Error("Failed to delete object from storage", "error", err)
		}
	}
	if err := s.recordStore.SoftDelete(ctx, recordID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return apiErrors.NewErrRecordNotFound(recordID)
		}
		return fmt.Errorf("failed to soft delete record: %w", err)
	}
	return nil
}

func (s *Record) ListRecordsDelta(ctx context.Context, userID uuid.UUID, recordType model.RecordType, updatedAfter time.Time, includeDeleted bool) ([]model.Record, []model.Tombstone, time.Time, error) {
	var (
		records []model.Record
		tombs   []model.Tombstone
		err     error
	)

	if recordType == "" {
		records, err = s.recordStore.GetUpdatedAfter(ctx, userID, updatedAfter)
		if err != nil {
			return nil, nil, time.Time{}, fmt.Errorf("get updated after failed: %w", err)
		}
		if includeDeleted {
			tombs, err = s.recordStore.GetDeletedAfter(ctx, userID, updatedAfter)
			if err != nil {
				return nil, nil, time.Time{}, fmt.Errorf("get deleted after failed: %w", err)
			}
		}
	} else {
		records, err = s.recordStore.GetUpdatedAfterByType(ctx, userID, recordType, updatedAfter)
		if err != nil {
			return nil, nil, time.Time{}, fmt.Errorf("get updated after by type failed: %w", err)
		}
		if includeDeleted {
			tombs, err = s.recordStore.GetDeletedAfterByType(ctx, userID, recordType, updatedAfter)
			if err != nil {
				return nil, nil, time.Time{}, fmt.Errorf("get deleted after by type failed: %w", err)
			}
		}
	}
	serverTime := time.Now()
	return records, tombs, serverTime, nil
}

func (s *Record) getRecordsOfType(ctx context.Context, userID uuid.UUID, recordType model.RecordType) ([]model.Record, error) {
	records, err := s.recordStore.GetByUserIDAndType(ctx, userID, recordType)
	if err != nil {
		return nil, fmt.Errorf("failed to get records of type %s by user id: %w", recordType, err)
	}

	return records, nil
}

func (s *Record) saveRecord(ctx context.Context, record model.Record, dataReader io.Reader) (model.Record, error) {
	if dataReader != nil {
		if record.S3Key == "" {
			return model.Record{}, fmt.Errorf("S3 key is empty")
		}

		err := s.storage.Upload(ctx, record.S3Key, dataReader)
		if err != nil {
			return model.Record{}, fmt.Errorf("failed to upload to storage: %w", err)
		}
	}

	record, err := s.recordStore.Create(ctx, record)
	if err != nil {
		if record.S3Key != "" {
			if err := s.storage.Delete(ctx, record.S3Key); err != nil {
				s.logger.Error("Failed to delete record from storage", "error", err)
			}
		}
		return model.Record{}, fmt.Errorf("failed to create record: %w", err)
	}

	return record, nil
}

func (s *Record) readMetadataFromStream(ctx context.Context, stream model.StreamReader) (*model.RecordMetadata, error) {
	const maxAttempts = 100
	attempts := 0

	for attempts < maxAttempts {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while reading metadata: %w", ctx.Err())
		default:
		}

		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("stream closed before metadata received")
			}
			return nil, fmt.Errorf("failed to receive from stream: %w", err)
		}

		if req.IsMetadata {
			if req.Metadata == nil {
				s.logger.Warn("Received metadata message with nil metadata, skipping")
				attempts++
				continue
			}
			return req.Metadata, nil
		}

		if req.DataChunk != nil && len(req.DataChunk) > 0 {
			return nil, fmt.Errorf("received data chunk before metadata - protocol violation")
		}

		attempts++
	}

	return nil, fmt.Errorf("metadata not received after %d attempts", maxAttempts)
}

func (s *Record) validateMetadata(metadata *model.RecordMetadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata is nil")
	}

	if metadata.Name == "" {
		return fmt.Errorf("record name is required")
	}

	if len(metadata.Name) > 255 {
		return fmt.Errorf("record name too long (max 255 characters)")
	}

	if metadata.Type == "" {
		return fmt.Errorf("record type is required")
	}

	validTypes := []model.RecordType{model.RecordTypeLogin, model.RecordTypeNote, model.RecordTypeCard, model.RecordTypeBinary}
	isValidType := slices.Contains(validTypes, metadata.Type)
	if !isValidType {
		return fmt.Errorf("invalid record type: %s", metadata.Type)
	}

	if len(metadata.EncryptedKey) == 0 {
		return fmt.Errorf("encrypted key is required")
	}

	if metadata.Alg == "" {
		return fmt.Errorf("encryption algorithm is required")
	}

	if metadata.Type == model.RecordTypeBinary {
		if metadata.ChunkSize <= 0 {
			return fmt.Errorf("chunk size must be > 0 for binary records")
		}
	}

	return nil
}

func (s *Record) generateS3Key(userID uuid.UUID) string {
	if userID == uuid.Nil {
		s.logger.Error("Cannot generate S3 key for nil userID")
		return ""
	}

	recordID := uuid.New()
	fileID := uuid.New()

	key := fmt.Sprintf("user-%s/record-%s/file-%s", userID.String(), recordID.String(), fileID.String())

	return key
}

func (s *Record) buildRecordMetadata(record model.Record) *model.RecordMetadata {
	return &model.RecordMetadata{
		ID:           record.ID.String(),
		Name:         record.Name,
		Description:  record.Description,
		Type:         record.Type,
		EncryptedKey: record.EncryptedKey,
		Alg:          record.Alg,
		ChunkSize:    record.EncryptedChunkSize,
	}
}
