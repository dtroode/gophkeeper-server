package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dtroode/gophkeeper-server/internal/logger"
	servermocks "github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/model"
)

// seqReader is a simple stub for StreamReader to control Recv sequence.
type seqReader struct {
	seq []struct {
		req *model.CreateRecordStreamRequest
		err error
	}
	i int
}

func (s *seqReader) Recv() (*model.CreateRecordStreamRequest, error) {
	if s.i >= len(s.seq) {
		return nil, io.EOF
	}
	r := s.seq[s.i]
	s.i++
	return r.req, r.err
}

func Test_readMetadataFromStream_Success(t *testing.T) {
	svc := NewRecord(nil, nil, nil, logger.New(0))
	md := &model.RecordMetadata{Name: "n", Type: model.RecordTypeNote, EncryptedKey: []byte("k"), Alg: "a"}
	sr := &seqReader{seq: []struct {
		req *model.CreateRecordStreamRequest
		err error
	}{
		{req: &model.CreateRecordStreamRequest{IsMetadata: true, Metadata: md}},
	}}
	out, err := svc.readMetadataFromStream(context.Background(), sr)
	require.NoError(t, err)
	assert.Equal(t, md, out)
}

func Test_readMetadataFromStream_DataBeforeMetadata(t *testing.T) {
	svc := NewRecord(nil, nil, nil, logger.New(0))
	sr := &seqReader{seq: []struct {
		req *model.CreateRecordStreamRequest
		err error
	}{
		{req: &model.CreateRecordStreamRequest{DataChunk: []byte{1, 2}, IsMetadata: false}},
	}}
	out, err := svc.readMetadataFromStream(context.Background(), sr)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "protocol violation")
}

func Test_readMetadataFromStream_StreamClosedBeforeMetadata(t *testing.T) {
	svc := NewRecord(nil, nil, nil, logger.New(0))
	sr := &seqReader{seq: nil}
	out, err := svc.readMetadataFromStream(context.Background(), sr)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "stream closed before metadata")
}

func Test_readMetadataFromStream_RecvError(t *testing.T) {
	svc := NewRecord(nil, nil, nil, logger.New(0))
	sr := &seqReader{seq: []struct {
		req *model.CreateRecordStreamRequest
		err error
	}{
		{req: nil, err: errors.New("boom")},
	}}
	out, err := svc.readMetadataFromStream(context.Background(), sr)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "failed to receive from stream")
}

func Test_readMetadataFromStream_MaxAttemptsExceeded(t *testing.T) {
	svc := NewRecord(nil, nil, nil, logger.New(0))
	// 100 times send metadata flag with nil metadata to exhaust attempts
	seq := make([]struct {
		req *model.CreateRecordStreamRequest
		err error
	}, 100)
	for i := range seq {
		seq[i] = struct {
			req *model.CreateRecordStreamRequest
			err error
		}{req: &model.CreateRecordStreamRequest{IsMetadata: true}}
	}
	sr := &seqReader{seq: seq}
	out, err := svc.readMetadataFromStream(context.Background(), sr)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "metadata not received after 100 attempts")
}

func Test_readMetadataFromStream_ContextCanceled(t *testing.T) {
	svc := NewRecord(nil, nil, nil, logger.New(0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Even if reader would return metadata, cancellation should win
	sr := &seqReader{seq: []struct {
		req *model.CreateRecordStreamRequest
		err error
	}{
		{req: &model.CreateRecordStreamRequest{IsMetadata: true, Metadata: &model.RecordMetadata{Name: "n", Type: model.RecordTypeNote, EncryptedKey: []byte("k"), Alg: "a"}}},
	}}
	out, err := svc.readMetadataFromStream(ctx, sr)
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestCreateRecordStream_SuccessBinary(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	usrStore.On("GetByID", mock.Anything, userID).Return(model.User{ID: userID}, nil)

	md := &model.RecordMetadata{
		Name: "file.bin", Description: "d", Type: model.RecordTypeBinary,
		EncryptedKey: []byte("k"), Alg: "alg", ChunkSize: 4, RequestID: uuid.New(),
	}

	sr := &servermocks.StreamReader{}
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: true, Metadata: md}, nil).Once()
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: false, DataChunk: []byte{1, 2, 3, 4}}, nil).Once()
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: false, DataChunk: []byte{5}}, io.EOF).Maybe()
	sr.On("Recv").Return(nil, io.EOF).Maybe()

	storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Run(func(args mock.Arguments) {
			// drain the pipe so writer goroutine can progress
			r := args.Get(2).(io.Reader)
			_, _ = io.Copy(io.Discard, r)
		}).Return(nil).Once()

	recStore.On("Create", mock.Anything, mock.MatchedBy(func(r model.Record) bool {
		return r.OwnerID == userID && r.Name == md.Name && r.Type == model.RecordTypeBinary && r.S3Key != "" && r.EncryptedChunkSize == md.ChunkSize
	})).Return(model.Record{ID: uuid.New(), OwnerID: userID, Name: md.Name, Type: model.RecordTypeBinary}, nil).Once()

	out, err := svc.CreateRecordStream(ctx, userID, sr)
	require.NoError(t, err)
	assert.Equal(t, userID, out.OwnerID)
	assert.Equal(t, md.Name, out.Name)
}

func TestCreateRecordStream_InvalidMetadata(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	usrStore.On("GetByID", mock.Anything, userID).Return(model.User{ID: userID}, nil)

	sr := &servermocks.StreamReader{}
	// Missing name -> invalid metadata
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: true, Metadata: &model.RecordMetadata{Type: model.RecordTypeNote, EncryptedKey: []byte("k"), Alg: "a"}}, nil).Once()
	sr.On("Recv").Return(nil, io.EOF).Maybe()

	out, err := svc.CreateRecordStream(ctx, userID, sr)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid metadata"))
	assert.Equal(t, model.Record{}, out)
}

func TestCreateRecordStream_GenerateS3KeyFail(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	// uuid.Nil triggers empty s3 key
	usrStore.On("GetByID", mock.Anything, uuid.Nil).Return(model.User{ID: uuid.Nil}, nil)

	sr := &servermocks.StreamReader{}
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: true, Metadata: &model.RecordMetadata{Name: "n", Type: model.RecordTypeNote, EncryptedKey: []byte("k"), Alg: "a"}}, nil).Once()
	sr.On("Recv").Return(nil, io.EOF).Maybe()

	out, err := svc.CreateRecordStream(ctx, uuid.Nil, sr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate S3 key")
	assert.Equal(t, model.Record{}, out)
}

func TestCreateRecordStream_UploadError(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	usrStore.On("GetByID", mock.Anything, userID).Return(model.User{ID: userID}, nil)

	md := &model.RecordMetadata{Name: "n", Type: model.RecordTypeBinary, EncryptedKey: []byte("k"), Alg: "a", ChunkSize: 4}
	sr := &servermocks.StreamReader{}
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: true, Metadata: md}, nil).Once()
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: false, DataChunk: []byte{1, 2, 3, 4}}, nil).Once()
	sr.On("Recv").Return(nil, io.EOF).Maybe()

	storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Run(func(args mock.Arguments) { _, _ = io.Copy(io.Discard, args.Get(2).(io.Reader)) }).
		Return(errors.New("upload-fail"))

	out, err := svc.CreateRecordStream(ctx, userID, sr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save record")
	assert.Equal(t, model.Record{}, out)
}

func TestCreateRecordStream_RecordCreateErrorDeletesObject(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	usrStore.On("GetByID", mock.Anything, userID).Return(model.User{ID: userID}, nil)

	md := &model.RecordMetadata{Name: "n", Type: model.RecordTypeBinary, EncryptedKey: []byte("k"), Alg: "a", ChunkSize: 2}
	sr := &servermocks.StreamReader{}
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: true, Metadata: md}, nil).Once()
	sr.On("Recv").Return(&model.CreateRecordStreamRequest{IsMetadata: false, DataChunk: []byte{9}}, io.EOF).Maybe()
	sr.On("Recv").Return(nil, io.EOF).Maybe()

	// Upload drains and succeeds
	storage.On("Upload", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Run(func(args mock.Arguments) { _, _ = io.Copy(io.Discard, args.Get(2).(io.Reader)) }).Return(nil)
	// Create fails -> saveRecord should try to Delete
	recStore.On("Create", mock.Anything, mock.MatchedBy(func(r model.Record) bool { return r.S3Key != "" })).Return(model.Record{}, errors.New("db-fail"))
	storage.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil).Maybe()

	out, err := svc.CreateRecordStream(ctx, userID, sr)
	require.Error(t, err)
	assert.Equal(t, model.Record{}, out)
}

func TestStreamRecordToClient_Errors(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	stream := &servermocks.StreamWriter{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	// invalid userID
	err := svc.StreamRecordToClient(ctx, uuid.Nil, uuid.New(), stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "userID cannot be nil")

	// invalid recordID
	err = svc.StreamRecordToClient(ctx, uuid.New(), uuid.Nil, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recordID cannot be nil")
}

func TestStreamRecordToClient_SendMetadataError(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	stream := &servermocks.StreamWriter{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	recID := uuid.New()
	rec := model.Record{ID: recID, OwnerID: userID, Name: "n", Type: model.RecordTypeBinary, S3Key: "s3", EncryptedChunkSize: 4}
	recStore.On("GetByID", mock.Anything, recID).Return(rec, nil)
	stream.On("Send", mock.Anything).Return(errors.New("send-fail")).Once()

	err := svc.StreamRecordToClient(ctx, userID, recID, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send metadata")
}

func TestStreamRecordToClient_DownloadError_NoData(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	stream := &servermocks.StreamWriter{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	recID := uuid.New()

	// No data available path
	rec := model.Record{ID: recID, OwnerID: userID, Name: "n", Type: model.RecordTypeNote, S3Key: ""}
	recStore.On("GetByID", mock.Anything, recID).Return(rec, nil)
	// metadata send still happens before the function observes there's no data
	stream.On("Send", mock.MatchedBy(func(resp *model.GetRecordStreamResponse) bool { return resp.Metadata != nil })).Return(nil).Once()

	err := svc.StreamRecordToClient(ctx, userID, recID, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no data available for record")
}

func TestStreamRecordToClient_InvalidChunkSize(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	stream := &servermocks.StreamWriter{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	recID := uuid.New()
	rec := model.Record{ID: recID, OwnerID: userID, Name: "n", Type: model.RecordTypeBinary, S3Key: "s3", EncryptedChunkSize: 0}
	recStore.On("GetByID", mock.Anything, recID).Return(rec, nil)
	// metadata send ok to reach chunk size validation
	stream.On("Send", mock.MatchedBy(func(resp *model.GetRecordStreamResponse) bool { return resp.Metadata != nil })).Return(nil).Once()
	// mock storage download to avoid panic
	storage.On("Download", mock.Anything, "s3").Return(io.NopCloser(strings.NewReader("test data")), nil)

	err := svc.StreamRecordToClient(ctx, userID, recID, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid encrypted chunk size")
}

func TestStreamRecordToClient_DownloadError(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	stream := &servermocks.StreamWriter{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	userID := uuid.New()
	recID := uuid.New()
	rec := model.Record{ID: recID, OwnerID: userID, Name: "n", Type: model.RecordTypeBinary, S3Key: "s3", EncryptedChunkSize: 4}
	recStore.On("GetByID", mock.Anything, recID).Return(rec, nil)
	stream.On("Send", mock.Anything).Return(nil).Once() // metadata ok
	storage.On("Download", mock.Anything, "s3").Return(nil, errors.New("dl-fail"))

	err := svc.StreamRecordToClient(ctx, userID, recID, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get data stream from S3")
}

func Test_saveRecord_Variants(t *testing.T) {
	ctx := context.Background()
	recStore := &servermocks.RecordStore{}
	usrStore := &servermocks.UserStore{}
	storage := &servermocks.Storage{}
	svc := NewRecord(recStore, usrStore, storage, logger.New(0))

	t.Run("reader_with_empty_s3_key", func(t *testing.T) {
		_, err := svc.saveRecord(ctx, model.Record{S3Key: ""}, bytes.NewReader([]byte("x")))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "S3 key is empty")
	})

	t.Run("upload_error", func(t *testing.T) {
		storage.On("Upload", mock.Anything, "k", mock.Anything).Return(errors.New("upl-fail")).Once()
		_, err := svc.saveRecord(ctx, model.Record{S3Key: "k"}, bytes.NewReader([]byte("x")))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to upload to storage")
	})

	t.Run("create_error_triggers_delete", func(t *testing.T) {
		storage.On("Upload", mock.Anything, "k2", mock.Anything).Run(func(args mock.Arguments) { _, _ = io.Copy(io.Discard, args.Get(2).(io.Reader)) }).Return(nil).Once()
		recStore.On("Create", mock.Anything, mock.MatchedBy(func(r model.Record) bool { return r.S3Key == "k2" })).Return(model.Record{}, errors.New("db-fail")).Once()
		storage.On("Delete", mock.Anything, "k2").Return(nil).Once()
		_, err := svc.saveRecord(ctx, model.Record{S3Key: "k2"}, bytes.NewReader([]byte("xyz")))
		require.Error(t, err)
	})

	t.Run("success_no_reader", func(t *testing.T) {
		recStore.On("Create", mock.Anything, mock.MatchedBy(func(r model.Record) bool { return r.Name == "n" })).Return(model.Record{Name: "n"}, nil).Once()
		rec, err := svc.saveRecord(ctx, model.Record{Name: "n"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "n", rec.Name)
	})
}

func Test_validateMetadata_Errors(t *testing.T) {
	svc := NewRecord(nil, nil, nil, logger.New(0))
	cases := []struct {
		name string
		md   *model.RecordMetadata
		want string
	}{
		{"empty_name", &model.RecordMetadata{Name: "", Type: model.RecordTypeNote, EncryptedKey: []byte("k"), Alg: "a"}, "record name is required"},
		{"too_long_name", &model.RecordMetadata{Name: strings.Repeat("a", 256), Type: model.RecordTypeNote, EncryptedKey: []byte("k"), Alg: "a"}, "too long"},
		{"empty_type", &model.RecordMetadata{Name: "n", Type: "", EncryptedKey: []byte("k"), Alg: "a"}, "record type is required"},
		{"invalid_type", &model.RecordMetadata{Name: "n", Type: "weird", EncryptedKey: []byte("k"), Alg: "a"}, "invalid record type"},
		{"no_key", &model.RecordMetadata{Name: "n", Type: model.RecordTypeNote, EncryptedKey: nil, Alg: "a"}, "encrypted key is required"},
		{"no_alg", &model.RecordMetadata{Name: "n", Type: model.RecordTypeNote, EncryptedKey: []byte("k"), Alg: ""}, "encryption algorithm is required"},
		{"binary_no_chunk", &model.RecordMetadata{Name: "n", Type: model.RecordTypeBinary, EncryptedKey: []byte("k"), Alg: "a", ChunkSize: 0}, "chunk size must be > 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.validateMetadata(tc.md)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
