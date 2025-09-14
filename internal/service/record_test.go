package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dtroode/gophkeeper-server/internal/logger"
	servermocks "github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/model"
)

// MockRecordStore mocks the RecordStore interface
type MockRecordStore struct {
	mock.Mock
}

func (m *MockRecordStore) Create(ctx context.Context, record model.Record) (model.Record, error) {
	args := m.Called(ctx, record)
	return args.Get(0).(model.Record), args.Error(1)
}

func (m *MockRecordStore) GetByID(ctx context.Context, id uuid.UUID) (model.Record, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.Record), args.Error(1)
}

func (m *MockRecordStore) GetByUserID(ctx context.Context, userID uuid.UUID) ([]model.Record, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.Record), args.Error(1)
}

func (m *MockRecordStore) GetByUserIDAndType(ctx context.Context, userID uuid.UUID, recordType model.RecordType) ([]model.Record, error) {
	args := m.Called(ctx, userID, recordType)
	return args.Get(0).([]model.Record), args.Error(1)
}

func (m *MockRecordStore) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRecordStore) GetUpdatedAfter(ctx context.Context, userID uuid.UUID, updatedAfter time.Time) ([]model.Record, error) {
	args := m.Called(ctx, userID, updatedAfter)
	return args.Get(0).([]model.Record), args.Error(1)
}

func (m *MockRecordStore) GetUpdatedAfterByType(ctx context.Context, userID uuid.UUID, recordType model.RecordType, updatedAfter time.Time) ([]model.Record, error) {
	args := m.Called(ctx, userID, recordType, updatedAfter)
	return args.Get(0).([]model.Record), args.Error(1)
}

func (m *MockRecordStore) GetDeletedAfter(ctx context.Context, userID uuid.UUID, deletedAfter time.Time) ([]model.Tombstone, error) {
	args := m.Called(ctx, userID, deletedAfter)
	return args.Get(0).([]model.Tombstone), args.Error(1)
}

func (m *MockRecordStore) GetDeletedAfterByType(ctx context.Context, userID uuid.UUID, recordType model.RecordType, deletedAfter time.Time) ([]model.Tombstone, error) {
	args := m.Called(ctx, userID, recordType, deletedAfter)
	return args.Get(0).([]model.Tombstone), args.Error(1)
}

// MockUserStore mocks the UserStore interface
type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) GetByEmail(ctx context.Context, email string) (model.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *MockUserStore) GetByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(model.User), args.Error(1)
}

func (m *MockUserStore) Create(ctx context.Context, user model.User) (model.User, error) {
	args := m.Called(ctx, user)
	return args.Get(0).(model.User), args.Error(1)
}

// MockStorage mocks the Storage interface
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Upload(ctx context.Context, key string, reader io.Reader) error {
	args := m.Called(ctx, key, reader)
	return args.Error(0)
}

func (m *MockStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockStorage) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockStorage) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func TestRecordService_CreateRecord(t *testing.T) {
	tests := []struct {
		name      string
		params    model.CreateRecordParams
		mockSetup func(*MockRecordStore, *MockUserStore, *MockStorage)
		wantErr   bool
	}{
		{
			name: "successful record creation",
			params: model.CreateRecordParams{
				UserID:        uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
				Name:          "Test Login",
				Description:   "Test login record",
				EncryptedData: []byte("encrypted data"),
				EncryptedKey:  []byte("encrypted key"),
				Alg:           "AES-256",
				Type:          model.RecordTypeLogin,
				RequestID:     uuid.New(),
			},
			mockSetup: func(recordStore *MockRecordStore, userStore *MockUserStore, storage *MockStorage) {
				userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
				user := model.User{
					ID:    userID,
					Email: "test@example.com",
				}

				userStore.On("GetByID", mock.Anything, userID).Return(user, nil)

				recordStore.On("Create", mock.Anything, mock.MatchedBy(func(r model.Record) bool {
					return r.Name == "Test Login" && r.Type == model.RecordTypeLogin && r.OwnerID == userID
				})).Return(model.Record{
					ID:            uuid.New(),
					OwnerID:       userID,
					Name:          "Test Login",
					Description:   "Test login record",
					EncryptedData: []byte("encrypted data"),
					EncryptedKey:  []byte("encrypted key"),
					Alg:           "AES-256",
					Type:          model.RecordTypeLogin,
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "user not found",
			params: model.CreateRecordParams{
				UserID: uuid.New(),
				Name:   "Test Login",
				Type:   model.RecordTypeLogin,
			},
			mockSetup: func(recordStore *MockRecordStore, userStore *MockUserStore, storage *MockStorage) {
				userStore.On("GetByID", mock.Anything, mock.Anything).Return(model.User{}, model.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "record store error",
			params: model.CreateRecordParams{
				UserID: uuid.New(),
				Name:   "Test Login",
				Type:   model.RecordTypeLogin,
			},
			mockSetup: func(recordStore *MockRecordStore, userStore *MockUserStore, storage *MockStorage) {
				user := model.User{ID: uuid.New(), Email: "test@example.com"}
				userStore.On("GetByID", mock.Anything, mock.Anything).Return(user, nil)
				recordStore.On("Create", mock.Anything, mock.Anything).Return(model.Record{}, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRecordStore := &MockRecordStore{}
			mockUserStore := &MockUserStore{}
			mockStorage := &MockStorage{}
			tt.mockSetup(mockRecordStore, mockUserStore, mockStorage)

			service := NewRecord(mockRecordStore, mockUserStore, mockStorage, logger.New(0))

			ctx := context.Background()
			result, err := service.CreateRecord(ctx, tt.params)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result.ID)
				assert.Equal(t, tt.params.Name, result.Name)
				assert.Equal(t, tt.params.Type, result.Type)
			}

			mockRecordStore.AssertExpectations(t)
			mockUserStore.AssertExpectations(t)
		})
	}
}

func TestRecordService_GetRecord(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	recordID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")

	tests := []struct {
		name      string
		userID    uuid.UUID
		recordID  uuid.UUID
		mockSetup func(*MockRecordStore)
		wantErr   bool
	}{
		{
			name:     "successful record retrieval",
			userID:   userID,
			recordID: recordID,
			mockSetup: func(recordStore *MockRecordStore) {
				record := model.Record{
					ID:          recordID,
					OwnerID:     userID,
					Name:        "Test Record",
					Description: "Test Description",
					S3Key:       "test-key",
					Type:        model.RecordTypeLogin,
				}
				recordStore.On("GetByID", mock.Anything, recordID).Return(record, nil)
			},
			wantErr: false,
		},
		{
			name:     "record not found",
			userID:   userID,
			recordID: recordID,
			mockSetup: func(recordStore *MockRecordStore) {
				recordStore.On("GetByID", mock.Anything, recordID).Return(model.Record{}, model.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name:     "access denied - wrong owner",
			userID:   uuid.New(), // Different user
			recordID: recordID,
			mockSetup: func(recordStore *MockRecordStore) {
				record := model.Record{
					ID:      recordID,
					OwnerID: uuid.New(), // Different owner
				}
				recordStore.On("GetByID", mock.Anything, recordID).Return(record, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRecordStore := &MockRecordStore{}
			mockUserStore := &MockUserStore{}
			mockStorage := &MockStorage{}
			tt.mockSetup(mockRecordStore)

			service := NewRecord(mockRecordStore, mockUserStore, mockStorage, logger.New(0))

			ctx := context.Background()
			result, err := service.GetRecord(ctx, tt.userID, tt.recordID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.recordID, result.ID)
				assert.Equal(t, tt.userID, result.OwnerID)
			}

			mockRecordStore.AssertExpectations(t)
		})
	}
}

func TestRecordService_GetRecords(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	tests := []struct {
		name      string
		userID    uuid.UUID
		mockSetup func(*MockRecordStore)
		wantLen   int
		wantErr   bool
	}{
		{
			name:   "successful records retrieval",
			userID: userID,
			mockSetup: func(recordStore *MockRecordStore) {
				records := []model.Record{
					{
						ID:      uuid.New(),
						OwnerID: userID,
						Name:    "Login 1",
						Type:    model.RecordTypeLogin,
					},
					{
						ID:      uuid.New(),
						OwnerID: userID,
						Name:    "Note 1",
						Type:    model.RecordTypeNote,
					},
				}
				recordStore.On("GetByUserID", mock.Anything, userID).Return(records, nil)
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:   "no records found",
			userID: userID,
			mockSetup: func(recordStore *MockRecordStore) {
				recordStore.On("GetByUserID", mock.Anything, userID).Return([]model.Record{}, nil)
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:   "database error",
			userID: userID,
			mockSetup: func(recordStore *MockRecordStore) {
				recordStore.On("GetByUserID", mock.Anything, userID).Return([]model.Record{}, errors.New("database error"))
			},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRecordStore := &MockRecordStore{}
			mockUserStore := &MockUserStore{}
			mockStorage := &MockStorage{}
			tt.mockSetup(mockRecordStore)

			service := NewRecord(mockRecordStore, mockUserStore, mockStorage, logger.New(0))

			ctx := context.Background()
			result, err := service.GetRecords(ctx, tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.wantLen)
				for _, record := range result {
					assert.Equal(t, tt.userID, record.OwnerID)
				}
			}

			mockRecordStore.AssertExpectations(t)
		})
	}
}

func TestRecordService_GetLogins(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	mockRecordStore := &MockRecordStore{}
	mockUserStore := &MockUserStore{}
	mockStorage := &MockStorage{}

	service := NewRecord(mockRecordStore, mockUserStore, mockStorage, logger.New(0))

	expectedRecords := []model.Record{
		{
			ID:      uuid.New(),
			OwnerID: userID,
			Name:    "Login 1",
			Type:    model.RecordTypeLogin,
		},
	}

	mockRecordStore.On("GetByUserIDAndType", mock.Anything, userID, model.RecordTypeLogin).
		Return(expectedRecords, nil)

	ctx := context.Background()
	result, err := service.GetLogins(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Login 1", result[0].Name)
	assert.Equal(t, model.RecordTypeLogin, result[0].Type)

	mockRecordStore.AssertExpectations(t)
}

func TestRecordService_GetNotes(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	mockRecordStore := &MockRecordStore{}
	mockUserStore := &MockUserStore{}
	mockStorage := &MockStorage{}

	service := NewRecord(mockRecordStore, mockUserStore, mockStorage, logger.New(0))

	expectedRecords := []model.Record{
		{
			ID:      uuid.New(),
			OwnerID: userID,
			Name:    "Note 1",
			Type:    model.RecordTypeNote,
		},
	}

	mockRecordStore.On("GetByUserIDAndType", mock.Anything, userID, model.RecordTypeNote).
		Return(expectedRecords, nil)

	ctx := context.Background()
	result, err := service.GetNotes(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Note 1", result[0].Name)
	assert.Equal(t, model.RecordTypeNote, result[0].Type)

	mockRecordStore.AssertExpectations(t)
}

func TestRecordService_GetCards(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	mockRecordStore := &MockRecordStore{}
	mockUserStore := &MockUserStore{}
	mockStorage := &MockStorage{}

	service := NewRecord(mockRecordStore, mockUserStore, mockStorage, logger.New(0))

	expectedRecords := []model.Record{
		{
			ID:      uuid.New(),
			OwnerID: userID,
			Name:    "Card 1",
			Type:    model.RecordTypeCard,
		},
	}

	mockRecordStore.On("GetByUserIDAndType", mock.Anything, userID, model.RecordTypeCard).
		Return(expectedRecords, nil)

	ctx := context.Background()
	result, err := service.GetCards(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Card 1", result[0].Name)
	assert.Equal(t, model.RecordTypeCard, result[0].Type)

	mockRecordStore.AssertExpectations(t)
}

func TestRecordService_GetBinaries(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	mockRecordStore := &MockRecordStore{}
	mockUserStore := &MockUserStore{}
	mockStorage := &MockStorage{}

	service := NewRecord(mockRecordStore, mockUserStore, mockStorage, logger.New(0))

	expectedRecords := []model.Record{
		{
			ID:      uuid.New(),
			OwnerID: userID,
			Name:    "Binary 1",
			Type:    model.RecordTypeBinary,
		},
	}

	mockRecordStore.On("GetByUserIDAndType", mock.Anything, userID, model.RecordTypeBinary).
		Return(expectedRecords, nil)

	ctx := context.Background()
	result, err := service.GetBinaries(ctx, userID)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Binary 1", result[0].Name)
	assert.Equal(t, model.RecordTypeBinary, result[0].Type)

	mockRecordStore.AssertExpectations(t)
}

func TestRecord_CreateRecord_Success(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	log := logger.New(0)

	userID := uuid.New()
	usrStore.On("GetByID", mock.Anything, userID).Return(model.User{ID: userID}, nil)
	recStore.On("Create", mock.Anything, mock.MatchedBy(func(r model.Record) bool { return r.OwnerID == userID && r.Name == "n" })).Return(model.Record{ID: uuid.New(), OwnerID: userID, Name: "n", Type: model.RecordTypeNote}, nil)

	svc := NewRecord(recStore, usrStore, storage, log)
	out, err := svc.CreateRecord(ctx, model.CreateRecordParams{UserID: userID, Name: "n", Type: model.RecordTypeNote})
	require.NoError(t, err)
	assert.Equal(t, userID, out.OwnerID)
}

func TestRecord_CreateRecord_UserNotFound(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}

	usrStore.On("GetByID", mock.Anything, mock.Anything).Return(model.User{}, model.ErrNotFound)

	svc := NewRecord(recStore, usrStore, storage, logger.New(0))
	_, err := svc.CreateRecord(ctx, model.CreateRecordParams{UserID: uuid.New(), Name: "n", Type: model.RecordTypeNote})
	require.Error(t, err)
}

func TestRecord_GetRecord_AccessDenied(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}

	userID := uuid.New()
	recID := uuid.New()
	recStore.On("GetByID", mock.Anything, recID).Return(model.Record{ID: recID, OwnerID: uuid.New()}, nil)

	svc := NewRecord(recStore, usrStore, storage, logger.New(0))
	_, err := svc.GetRecord(ctx, userID, recID)
	require.Error(t, err)
}

func TestRecord_GetRecordDataStream_Errors(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	_, err := svc.GetRecordDataStream(ctx, "")
	require.Error(t, err)

	storage.On("Download", mock.Anything, "k").Return(nil, errors.New("boom"))
	_, err = svc.GetRecordDataStream(ctx, "k")
	require.Error(t, err)
}

func TestRecord_StreamRecordToClient_SendsMetadataThenData(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	stream := &servermocks.StreamWriter{}
	userID := uuid.New()
	recID := uuid.New()

	rec := model.Record{ID: recID, OwnerID: userID, Name: "bin", Type: model.RecordTypeBinary, S3Key: "s3", EncryptedChunkSize: 4}
	recStore.On("GetByID", mock.Anything, recID).Return(rec, nil)

	var sent [][]byte
	stream.On("Send", mock.MatchedBy(func(resp *model.GetRecordStreamResponse) bool { return resp.Metadata != nil && !resp.IsLastChunk })).Return(nil).Once()
	storage.On("Download", mock.Anything, "s3").Return(io.NopCloser(bytes.NewReader([]byte{1, 2, 3, 4, 5})), nil)
	stream.On("Send", mock.MatchedBy(func(resp *model.GetRecordStreamResponse) bool { sent = append(sent, resp.DataChunk); return true })).Return(nil).Twice()

	svc := NewRecord(recStore, usrStore, storage, logger.New(0))
	require.NoError(t, svc.StreamRecordToClient(ctx, userID, recID, stream))
	require.GreaterOrEqual(t, len(sent), 1)
}

func TestRecord_DeleteRecord(t *testing.T) {
	userID := uuid.New()
	recordID := uuid.New()

	tests := []struct {
		name      string
		getRecord model.Record
		getErr    error
		delErr    error
		softErr   error
		wantErr   bool
	}{
		{
			name:      "ok_with_s3_and_softdelete",
			getRecord: model.Record{ID: recordID, OwnerID: userID, S3Key: "k"},
		},
		{
			name:    "not_found_maps_to_api_error",
			getErr:  model.ErrNotFound,
			wantErr: true,
		},
		{
			name:      "softdelete_not_found",
			getRecord: model.Record{ID: recordID, OwnerID: userID},
			softErr:   model.ErrNotFound,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recStore := &servermocks.RecordStore{}
			usrStore := &servermocks.UserStore{}
			storage := &servermocks.Storage{}
			log := logger.New(0)

			if tt.getErr != nil {
				recStore.On("GetByID", mock.Anything, recordID).Return(model.Record{}, tt.getErr)
			} else {
				recStore.On("GetByID", mock.Anything, recordID).Return(tt.getRecord, nil)
			}

			if tt.getErr == nil && tt.getRecord.S3Key != "" {
				storage.On("Delete", mock.Anything, tt.getRecord.S3Key).Return(tt.delErr).Maybe()
			}
			if tt.getErr == nil {
				recStore.On("SoftDelete", mock.Anything, recordID).Return(tt.softErr)
			}

			svc := NewRecord(recStore, usrStore, storage, log)
			err := svc.DeleteRecord(context.Background(), userID, recordID)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRecord_ListRecordsDelta(t *testing.T) {
	userID := uuid.New()
	now := time.Now().Add(-time.Hour)

	tests := []struct {
		name           string
		recordType     model.RecordType
		includeDeleted bool
		setup          func(r *servermocks.RecordStore)
		wantErr        bool
	}{
		{
			name:       "all_types_no_deleted",
			recordType: "",
			setup: func(r *servermocks.RecordStore) {
				r.On("GetUpdatedAfter", mock.Anything, userID, mock.AnythingOfType("time.Time")).Return([]model.Record{{ID: uuid.New()}}, nil)
			},
		},
		{
			name:           "all_types_with_deleted",
			recordType:     "",
			includeDeleted: true,
			setup: func(r *servermocks.RecordStore) {
				r.On("GetUpdatedAfter", mock.Anything, userID, mock.AnythingOfType("time.Time")).Return([]model.Record{{ID: uuid.New()}}, nil)
				r.On("GetDeletedAfter", mock.Anything, userID, mock.AnythingOfType("time.Time")).Return([]model.Tombstone{{ID: uuid.New()}}, nil)
			},
		},
		{
			name:       "by_type_no_deleted",
			recordType: model.RecordTypeLogin,
			setup: func(r *servermocks.RecordStore) {
				r.On("GetUpdatedAfterByType", mock.Anything, userID, model.RecordTypeLogin, mock.AnythingOfType("time.Time")).Return([]model.Record{{ID: uuid.New()}}, nil)
			},
		},
		{
			name:           "by_type_with_deleted_err",
			recordType:     model.RecordTypeLogin,
			includeDeleted: true,
			setup: func(r *servermocks.RecordStore) {
				r.On("GetUpdatedAfterByType", mock.Anything, userID, model.RecordTypeLogin, mock.AnythingOfType("time.Time")).Return([]model.Record{}, nil)
				r.On("GetDeletedAfterByType", mock.Anything, userID, model.RecordTypeLogin, mock.AnythingOfType("time.Time")).Return(nil, errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recStore := &servermocks.RecordStore{}
			usrStore := &servermocks.UserStore{}
			storage := &servermocks.Storage{}
			log := logger.New(0)
			if tt.setup != nil {
				tt.setup(recStore)
			}
			svc := NewRecord(recStore, usrStore, storage, log)
			_, _, _, err := svc.ListRecordsDelta(context.Background(), userID, tt.recordType, now, tt.includeDeleted)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRecord_generateS3Key_Extended(t *testing.T) {
	logger := logger.New(0)
	recordService := NewRecord(nil, nil, nil, logger)

	validUserID := uuid.New()
	result1 := recordService.generateS3Key(validUserID)
	assert.NotEmpty(t, result1)
	assert.Contains(t, result1, "user-")

	result2 := recordService.generateS3Key(uuid.Nil)
	assert.Empty(t, result2)
}

func TestRecord_validateMetadata_Extended(t *testing.T) {
	logger := logger.New(0)
	recordService := NewRecord(nil, nil, nil, logger)

	err1 := recordService.validateMetadata(nil)
	assert.Error(t, err1)

	validMeta := &model.RecordMetadata{
		Name:         "Test",
		Type:         model.RecordTypeLogin,
		EncryptedKey: []byte("key"),
		Alg:          "aes256",
	}
	err2 := recordService.validateMetadata(validMeta)
	assert.NoError(t, err2)
}
