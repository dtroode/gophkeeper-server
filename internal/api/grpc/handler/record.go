package handler

import (
	"context"
	"fmt"
	"io"
	"time"

	apiErrors "github.com/dtroode/gophkeeper-api/errors"
	"github.com/dtroode/gophkeeper-api/proto"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecordService defines business operations for record management.
type RecordService interface {
	CreateRecord(ctx context.Context, params model.CreateRecordParams) (model.Record, error)
	CreateRecordStream(ctx context.Context, userID uuid.UUID, stream model.StreamReader) (model.Record, error)
	GetRecord(ctx context.Context, userID uuid.UUID, recordID uuid.UUID) (model.Record, error)
	GetRecords(ctx context.Context, userID uuid.UUID) ([]model.Record, error)
	GetLogins(ctx context.Context, userID uuid.UUID) ([]model.Record, error)
	GetNotes(ctx context.Context, userID uuid.UUID) ([]model.Record, error)
	GetCards(ctx context.Context, userID uuid.UUID) ([]model.Record, error)
	GetBinaries(ctx context.Context, userID uuid.UUID) ([]model.Record, error)
	GetRecordDataStream(ctx context.Context, s3Key string) (io.ReadCloser, error)
	StreamRecordToClient(ctx context.Context, userID uuid.UUID, recordID uuid.UUID, stream model.StreamWriter) error
	DeleteRecord(ctx context.Context, userID uuid.UUID, recordID uuid.UUID) error
	ListRecordsDelta(ctx context.Context, userID uuid.UUID, recordType model.RecordType, updatedAfter time.Time, includeDeleted bool) ([]model.Record, []model.Tombstone, time.Time, error)
}

// Record handles gRPC endpoints for records.
type Record struct {
	proto.UnimplementedAPIServer
	recordService  RecordService
	contextManager model.ContextManager
	logger         *logger.Logger
}

// NewRecord creates a new Record handler.
func NewRecord(recordService RecordService, contextManager model.ContextManager, logger *logger.Logger) *Record {
	return &Record{
		recordService:  recordService,
		contextManager: contextManager,
		logger:         logger,
	}
}

// ListRecords returns metadata for records with optional filtering and delta.
func (h *Record) ListRecords(ctx context.Context, req *proto.ListRecordsRequest) (*proto.ListRecordsResponse, error) {
	h.logger.Debug("Record handler: processing list records request",
		"type_filter", req.TypeFilter,
		"updated_after", req.UpdatedAfter,
		"include_deleted", req.IncludeDeleted)

	userID, err := h.extractUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	if req.UpdatedAfter > 0 || req.IncludeDeleted {
		var recordType model.RecordType
		if req.TypeFilter != proto.RecordType_UNKNOWN {
			recordType, err = convertProtoRecordType(req.TypeFilter)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}
		updatedAfter := time.Unix(req.UpdatedAfter, 0)
		records, tombs, serverTime, err := h.recordService.ListRecordsDelta(ctx, userID, recordType, updatedAfter, req.IncludeDeleted)
		if err != nil {
			h.logger.Error("Record handler: list records delta failed",
				"user_id", userID,
				"error", err.Error())
			return nil, handleError(err)
		}
		var metadata []*proto.RecordMetadata
		for _, record := range records {
			metadata = append(metadata, h.convertRecordToMetadata(record))
		}
		var protoTombs []*proto.Tombstone
		for _, t := range tombs {
			protoTombs = append(protoTombs, &proto.Tombstone{RecordId: t.ID.String(), DeletedAt: t.DeletedAt.Unix()})
		}
		return &proto.ListRecordsResponse{
			Records:       metadata,
			NextPageToken: "",
			ServerTime:    serverTime.Unix(),
			Tombstones:    protoTombs,
		}, nil
	}

	var records []model.Record
	if req.TypeFilter != proto.RecordType_UNKNOWN {
		recordType, err := convertProtoRecordType(req.TypeFilter)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		switch recordType {
		case model.RecordTypeLogin:
			records, err = h.recordService.GetLogins(ctx, userID)
		case model.RecordTypeNote:
			records, err = h.recordService.GetNotes(ctx, userID)
		case model.RecordTypeCard:
			records, err = h.recordService.GetCards(ctx, userID)
		case model.RecordTypeBinary:
			records, err = h.recordService.GetBinaries(ctx, userID)
		}
	} else {
		records, err = h.recordService.GetRecords(ctx, userID)
	}

	if err != nil {
		h.logger.Error("Record handler: list records failed",
			"user_id", userID,
			"type_filter", req.TypeFilter,
			"error", err.Error())
		return nil, handleError(err)
	}

	var metadata []*proto.RecordMetadata
	for _, record := range records {
		metadata = append(metadata, h.convertRecordToMetadata(record))
	}

	h.logger.Info("Record handler: records listed successfully",
		"user_id", userID,
		"type_filter", req.TypeFilter,
		"count", len(records))

	return &proto.ListRecordsResponse{
		Records: metadata,
	}, nil
}

// CreateRecordStream receives metadata and encrypted chunks and returns created record ID.
func (h *Record) CreateRecordStream(stream proto.API_CreateRecordStreamServer) error {
	h.logger.Debug("Record handler: processing create record stream request")

	userID, err := h.extractUserIDFromContext(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	adapter := &protoStreamReader{stream: stream}
	record, err := h.recordService.CreateRecordStream(stream.Context(), userID, adapter)
	if err != nil {
		h.logger.Error("Record handler: create record stream failed",
			"user_id", userID,
			"error", err.Error())
		return handleError(err)
	}

	h.logger.Info("Record handler: record stream created successfully",
		"record_id", record.ID,
		"user_id", userID,
		"s3_key", record.S3Key)

	return stream.SendAndClose(&proto.CreateRecordStreamResponse{
		RecordId: record.ID.String(),
		Success:  true,
		// BytesReceived: record.FileSize,
	})
}

// GetRecordStream streams back a record: metadata first, then encrypted chunks.
func (h *Record) GetRecordStream(req *proto.GetRecordStreamRequest, stream proto.API_GetRecordStreamServer) error {
	h.logger.Debug("Record handler: processing get record stream request",
		"record_id", req.RecordId)

	userID, err := h.extractUserIDFromContext(stream.Context())
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}

	recordID, err := uuid.Parse(req.RecordId)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid record ID")
	}

	adapter := &protoStreamWriter{stream: stream}

	return h.recordService.StreamRecordToClient(stream.Context(), userID, recordID, adapter)
}

// GetRecord returns metadata and encrypted data for a small record.
func (h *Record) GetRecord(ctx context.Context, req *proto.GetRecordRequest) (*proto.GetRecordResponse, error) {
	h.logger.Debug("Record handler: processing get record request",
		"record_id", req.RecordId)

	userID, err := h.extractUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	recordID, err := uuid.Parse(req.RecordId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid record ID")
	}

	rec, err := h.recordService.GetRecord(ctx, userID, recordID)
	if err != nil {
		h.logger.Error("Record handler: get record failed",
			"user_id", userID,
			"record_id", req.RecordId,
			"error", err.Error())
		return nil, handleError(err)
	}

	meta := h.convertRecordToMetadata(rec)

	h.logger.Info("Record handler: record returned successfully",
		"user_id", userID,
		"record_id", req.RecordId,
		"type", rec.Type)

	return &proto.GetRecordResponse{
		Metadata:      meta,
		EncryptedData: rec.EncryptedData,
		Success:       true,
	}, nil
}

// CreateRecord creates a record from metadata and encrypted payload.
func (h *Record) CreateRecord(ctx context.Context, req *proto.CreateRecordRequest) (*proto.CreateRecordResponse, error) {
	h.logger.Debug("Record handler: processing create record request")

	userID, err := h.extractUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	recordType, err := convertProtoRecordType(req.Metadata.Type)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var requestID uuid.UUID
	if req.Metadata.RequestId != "" {
		requestID, err = uuid.Parse(req.Metadata.RequestId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request_id")
		}
	}

	params := model.CreateRecordParams{
		UserID:        userID,
		Name:          req.Metadata.Name,
		Description:   req.Metadata.Description,
		EncryptedData: req.EncryptedData,
		EncryptedKey:  req.Metadata.EncryptedKey,
		Alg:           req.Metadata.Alg,
		Type:          recordType,
		RequestID:     requestID,
	}

	record, err := h.recordService.CreateRecord(ctx, params)
	if err != nil {
		h.logger.Error("Record handler: create record failed", "user_id", userID, "error", err.Error())
		return nil, handleError(err)
	}

	h.logger.Info("Record handler: record created successfully", "record_id", record.ID, "user_id", userID)

	return &proto.CreateRecordResponse{RecordId: record.ID.String(), Success: true}, nil
}

// DeleteRecord deletes a record by ID.
func (h *Record) DeleteRecord(ctx context.Context, req *proto.DeleteRecordRequest) (*proto.DeleteRecordResponse, error) {
	h.logger.Debug("Record handler: processing delete record request", "record_id", req.RecordId)

	userID, err := h.extractUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	recordID, err := uuid.Parse(req.RecordId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid record ID")
	}

	if err := h.recordService.DeleteRecord(ctx, userID, recordID); err != nil {
		h.logger.Error("Record handler: delete record failed", "user_id", userID, "record_id", req.RecordId, "error", err.Error())
		return nil, handleError(err)
	}

	h.logger.Info("Record handler: record deleted successfully", "user_id", userID, "record_id", req.RecordId)
	return &proto.DeleteRecordResponse{Success: true}, nil
}

type protoStreamReader struct {
	stream proto.API_CreateRecordStreamServer
}

func (p *protoStreamReader) Recv() (*model.CreateRecordStreamRequest, error) {
	req, err := p.stream.Recv()
	if err != nil {
		return nil, err
	}

	switch r := req.Request.(type) {
	case *proto.CreateRecordStreamRequest_Metadata:
		recordType, err := convertProtoRecordType(r.Metadata.Type)
		if err != nil {
			return nil, err
		}
		var rid uuid.UUID
		if r.Metadata.RequestId != "" {
			rid, err = uuid.Parse(r.Metadata.RequestId)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid request_id")
			}
		}

		return &model.CreateRecordStreamRequest{
			Metadata: &model.RecordMetadata{
				Name:         r.Metadata.Name,
				Description:  r.Metadata.Description,
				Type:         recordType,
				EncryptedKey: r.Metadata.EncryptedKey,
				Alg:          r.Metadata.Alg,
				ChunkSize:    int(r.Metadata.ChunkSize),
				RequestID:    rid,
			},
			DataChunk:  nil,
			IsMetadata: true,
		}, nil
	case *proto.CreateRecordStreamRequest_DataChunk:
		return &model.CreateRecordStreamRequest{
			Metadata:   nil,
			DataChunk:  r.DataChunk,
			IsMetadata: false,
		}, nil
	default:
		return nil, fmt.Errorf("unknown request type")
	}
}

type protoStreamWriter struct {
	stream proto.API_GetRecordStreamServer
}

func (p *protoStreamWriter) Send(resp *model.GetRecordStreamResponse) error {
	var protoResp *proto.GetRecordStreamResponse

	if resp.Metadata != nil {
		protoResp = &proto.GetRecordStreamResponse{
			Response: &proto.GetRecordStreamResponse_Metadata{
				Metadata: &proto.RecordMetadata{
					RecordId:     resp.Metadata.ID,
					Name:         resp.Metadata.Name,
					Description:  resp.Metadata.Description,
					Type:         convertRecordTypeToProto(resp.Metadata.Type),
					EncryptedKey: resp.Metadata.EncryptedKey,
					Alg:          resp.Metadata.Alg,
					ChunkSize:    int64(resp.Metadata.ChunkSize),
				},
			},
			IsLastChunk: resp.IsLastChunk,
		}
	} else {
		protoResp = &proto.GetRecordStreamResponse{
			Response: &proto.GetRecordStreamResponse_DataChunk{
				DataChunk: resp.DataChunk,
			},
			IsLastChunk: resp.IsLastChunk,
		}
	}

	return p.stream.Send(protoResp)
}

func (h *Record) extractUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := h.contextManager.GetUserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, apiErrors.NewErrMissingAuthorizationToken()
	}
	return userID, nil
}

func convertProtoRecordType(protoType proto.RecordType) (model.RecordType, error) {
	switch protoType {
	case proto.RecordType_LOGIN:
		return model.RecordTypeLogin, nil
	case proto.RecordType_NOTE:
		return model.RecordTypeNote, nil
	case proto.RecordType_CARD:
		return model.RecordTypeCard, nil
	case proto.RecordType_BINARY:
		return model.RecordTypeBinary, nil
	default:
		return "", apiErrors.NewErrInvalidRecordType("unknown")
	}
}

func convertRecordTypeToProto(recordType model.RecordType) proto.RecordType {
	switch recordType {
	case model.RecordTypeLogin:
		return proto.RecordType_LOGIN
	case model.RecordTypeNote:
		return proto.RecordType_NOTE
	case model.RecordTypeCard:
		return proto.RecordType_CARD
	case model.RecordTypeBinary:
		return proto.RecordType_BINARY
	default:
		return proto.RecordType_BINARY
	}
}

func (h *Record) convertRecordToMetadata(record model.Record) *proto.RecordMetadata {
	var protoType proto.RecordType
	switch record.Type {
	case model.RecordTypeLogin:
		protoType = proto.RecordType_LOGIN
	case model.RecordTypeNote:
		protoType = proto.RecordType_NOTE
	case model.RecordTypeCard:
		protoType = proto.RecordType_CARD
	case model.RecordTypeBinary:
		protoType = proto.RecordType_BINARY
	default:
		protoType = proto.RecordType_UNKNOWN
	}

	return &proto.RecordMetadata{
		RecordId:     record.ID.String(),
		Name:         record.Name,
		Description:  record.Description,
		EncryptedKey: record.EncryptedKey,
		Alg:          record.Alg,
		Type:         protoType,
		ChunkSize:    int64(record.EncryptedChunkSize),
	}
}
