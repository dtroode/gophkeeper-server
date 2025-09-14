package handler

import (
	"context"
	"testing"
	"time"

	apiProto "github.com/dtroode/gophkeeper-api/proto"
	"github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/dtroode/gophkeeper-server/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRecord_ListRecords_All(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recs := []model.Record{{ID: uuid.New(), Name: "n1", Type: model.RecordTypeLogin}, {ID: uuid.New(), Name: "n2", Type: model.RecordTypeNote}}

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("GetRecords", mock.Anything, userID).Return(recs, nil)

	h := NewRecord(svc, cm, lg)

	resp, err := h.ListRecords(context.Background(), &apiProto.ListRecordsRequest{TypeFilter: apiProto.RecordType_UNKNOWN})
	assert.NoError(t, err)
	assert.Len(t, resp.Records, 2)
}

func TestRecord_ListRecords_Delta_WithTombstones(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	now := time.Now().Add(-time.Hour)
	recs := []model.Record{{ID: uuid.New(), Name: "n1", Type: model.RecordTypeBinary}}
	tombs := []model.Tombstone{{ID: uuid.New(), DeletedAt: time.Now().Add(-30 * time.Minute)}}

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("ListRecordsDelta", mock.Anything, userID, model.RecordTypeBinary, mock.AnythingOfType("time.Time"), true).Return(recs, tombs, time.Now(), nil)

	h := NewRecord(svc, cm, lg)

	resp, err := h.ListRecords(context.Background(), &apiProto.ListRecordsRequest{TypeFilter: apiProto.RecordType_BINARY, UpdatedAfter: now.Unix(), IncludeDeleted: true})
	assert.NoError(t, err)
	assert.Len(t, resp.Records, 1)
	assert.Len(t, resp.Tombstones, 1)
}

func TestRecord_GetRecord_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recID := uuid.New()
	rec := model.Record{ID: recID, Name: "n", Type: model.RecordTypeCard}

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("GetRecord", mock.Anything, userID, recID).Return(rec, nil)

	h := NewRecord(svc, cm, lg)

	resp, err := h.GetRecord(context.Background(), &apiProto.GetRecordRequest{RecordId: recID.String()})
	assert.NoError(t, err)
	assert.Equal(t, recID.String(), resp.Metadata.RecordId)
}

func TestRecord_GetRecord_InvalidID(t *testing.T) {
	t.Parallel()

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(uuid.New(), true)
	h := NewRecord(svc, cm, lg)
	resp, err := h.GetRecord(context.Background(), &apiProto.GetRecordRequest{RecordId: "not-a-uuid"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestRecord_CreateDelete_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recID := uuid.New()

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)

	svc.On("CreateRecord", mock.Anything, mock.AnythingOfType("model.CreateRecordParams")).Return(model.Record{ID: recID}, nil)

	h := NewRecord(svc, cm, lg)

	// CreateRecord
	respCreate, err := h.CreateRecord(context.Background(), &apiProto.CreateRecordRequest{Metadata: &apiProto.RecordMetadata{Type: apiProto.RecordType_LOGIN}})
	assert.NoError(t, err)
	assert.Equal(t, recID.String(), respCreate.RecordId)

	// DeleteRecord
	svc.ExpectedCalls = nil
	svc.On("DeleteRecord", mock.Anything, userID, recID).Return(nil)

	_, err = h.DeleteRecord(context.Background(), &apiProto.DeleteRecordRequest{RecordId: recID.String()})
	assert.NoError(t, err)
}

func TestRecord_DeleteRecord_InvalidID(t *testing.T) {
	t.Parallel()

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(uuid.New(), true)
	h := NewRecord(svc, cm, lg)
	resp, err := h.DeleteRecord(context.Background(), &apiProto.DeleteRecordRequest{RecordId: "bad"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestRecord_ListRecords_InvalidType(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)

	h := NewRecord(svc, cm, lg)
	resp, err := h.ListRecords(context.Background(), &apiProto.ListRecordsRequest{TypeFilter: 999})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestRecord_CreateRecord_InvalidRequestID(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)

	h := NewRecord(svc, cm, lg)
	resp, err := h.CreateRecord(context.Background(), &apiProto.CreateRecordRequest{
		Metadata: &apiProto.RecordMetadata{Type: apiProto.RecordType_LOGIN, RequestId: "bad-uuid"},
	})
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestRecord_ListRecords_ByTypes_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()
	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)

	rec := model.Record{ID: uuid.New(), Name: "n"}
	svc.On("GetLogins", mock.Anything, userID).Return([]model.Record{rec}, nil)
	svc.On("GetNotes", mock.Anything, userID).Return([]model.Record{rec}, nil)
	svc.On("GetCards", mock.Anything, userID).Return([]model.Record{rec}, nil)
	svc.On("GetBinaries", mock.Anything, userID).Return([]model.Record{rec}, nil)

	h := NewRecord(svc, cm, lg)

	for _, tp := range []apiProto.RecordType{apiProto.RecordType_LOGIN, apiProto.RecordType_NOTE, apiProto.RecordType_CARD, apiProto.RecordType_BINARY} {
		resp, err := h.ListRecords(context.Background(), &apiProto.ListRecordsRequest{TypeFilter: tp})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Records, 1)
	}
}

func TestRecord_GetRecord_NotFound(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()
	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("GetRecord", mock.Anything, userID, recID).Return(model.Record{}, model.ErrNotFound)

	h := NewRecord(svc, cm, lg)
	resp, err := h.GetRecord(context.Background(), &apiProto.GetRecordRequest{RecordId: recID.String()})
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

type fakeCreateStream struct {
	ctx    context.Context
	closed bool
	resp   *apiProto.CreateRecordStreamResponse
}

func (f *fakeCreateStream) Context() context.Context                           { return f.ctx }
func (f *fakeCreateStream) Recv() (*apiProto.CreateRecordStreamRequest, error) { return nil, nil }
func (f *fakeCreateStream) SendAndClose(resp *apiProto.CreateRecordStreamResponse) error {
	f.resp = resp
	f.closed = true
	return nil
}
func (f *fakeCreateStream) SendMsg(m interface{}) error     { return nil }
func (f *fakeCreateStream) RecvMsg(m interface{}) error     { return nil }
func (f *fakeCreateStream) SendHeader(md metadata.MD) error { return nil }
func (f *fakeCreateStream) SetTrailer(md metadata.MD)       {}
func (f *fakeCreateStream) SetHeader(md metadata.MD) error  { return nil }

type fakeGetStream struct {
	ctx  context.Context
	sent []*apiProto.GetRecordStreamResponse
}

func (f *fakeGetStream) Context() context.Context { return f.ctx }
func (f *fakeGetStream) Send(resp *apiProto.GetRecordStreamResponse) error {
	f.sent = append(f.sent, resp)
	return nil
}
func (f *fakeGetStream) SendMsg(m interface{}) error     { return nil }
func (f *fakeGetStream) RecvMsg(m interface{}) error     { return nil }
func (f *fakeGetStream) SendHeader(md metadata.MD) error { return nil }
func (f *fakeGetStream) SetTrailer(md metadata.MD)       {}
func (f *fakeGetStream) SetHeader(md metadata.MD) error  { return nil }

func TestRecord_CreateRecordStream_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recID := uuid.New()

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("CreateRecordStream", mock.Anything, userID, mock.Anything).Return(model.Record{ID: recID, S3Key: "k"}, nil)

	h := NewRecord(svc, cm, lg)

	stream := &fakeCreateStream{ctx: context.Background()}
	err := h.CreateRecordStream(stream)
	assert.NoError(t, err)
	assert.True(t, stream.closed)
	assert.NotNil(t, stream.resp)
	assert.Equal(t, recID.String(), stream.resp.RecordId)
}

func TestRecord_GetRecordStream_Success(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recID := uuid.New()

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("StreamRecordToClient", mock.Anything, userID, recID, mock.Anything).Return(nil)

	h := NewRecord(svc, cm, lg)

	stream := &fakeGetStream{ctx: context.Background()}
	err := h.GetRecordStream(&apiProto.GetRecordStreamRequest{RecordId: recID.String()}, stream)
	assert.NoError(t, err)
}

func TestRecord_ListRecords_ErrorGeneral(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("GetRecords", mock.Anything, userID).Return(nil, assert.AnError)

	h := NewRecord(svc, cm, lg)
	resp, err := h.ListRecords(context.Background(), &apiProto.ListRecordsRequest{TypeFilter: apiProto.RecordType_UNKNOWN})
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestRecord_ListRecords_Delta_Error(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("ListRecordsDelta", mock.Anything, userID, model.RecordType(""), mock.AnythingOfType("time.Time"), true).Return(nil, nil, time.Now(), assert.AnError)

	h := NewRecord(svc, cm, lg)
	resp, err := h.ListRecords(context.Background(), &apiProto.ListRecordsRequest{TypeFilter: apiProto.RecordType_UNKNOWN, UpdatedAfter: time.Now().Add(-time.Hour).Unix(), IncludeDeleted: true})
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestRecord_CreateRecord_Error(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("CreateRecord", mock.Anything, mock.AnythingOfType("model.CreateRecordParams")).Return(model.Record{}, assert.AnError)

	h := NewRecord(svc, cm, lg)
	resp, err := h.CreateRecord(context.Background(), &apiProto.CreateRecordRequest{Metadata: &apiProto.RecordMetadata{Type: apiProto.RecordType_LOGIN}})
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestRecord_CreateRecordStream_Error(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("CreateRecordStream", mock.Anything, userID, mock.Anything).Return(model.Record{}, assert.AnError)

	h := NewRecord(svc, cm, lg)
	stream := &fakeCreateStream{ctx: context.Background()}
	err := h.CreateRecordStream(stream)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestRecord_GetRecordStream_Error(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("StreamRecordToClient", mock.Anything, userID, recID, mock.Anything).Return(assert.AnError)

	h := NewRecord(svc, cm, lg)
	stream := &fakeGetStream{ctx: context.Background()}
	err := h.GetRecordStream(&apiProto.GetRecordStreamRequest{RecordId: recID.String()}, stream)
	assert.Error(t, err)
}

func TestRecord_DeleteRecord_Error(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	recID := uuid.New()
	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	cm.On("GetUserIDFromContext", mock.Anything).Return(userID, true)
	svc.On("DeleteRecord", mock.Anything, userID, recID).Return(assert.AnError)

	h := NewRecord(svc, cm, lg)
	resp, err := h.DeleteRecord(context.Background(), &apiProto.DeleteRecordRequest{RecordId: recID.String()})
	assert.Nil(t, resp)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestRecord_Unauthenticated_All(t *testing.T) {
	t.Parallel()

	svc := mocks.NewRecordService(t)
	cm := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()
	cm.On("GetUserIDFromContext", mock.Anything).Return(uuid.Nil, false)
	h := NewRecord(svc, cm, lg)

	// ListRecords
	r1, e1 := h.ListRecords(context.Background(), &apiProto.ListRecordsRequest{})
	assert.Nil(t, r1)
	st1, _ := status.FromError(e1)
	assert.Equal(t, codes.Unauthenticated, st1.Code())

	// CreateRecord
	r2, e2 := h.CreateRecord(context.Background(), &apiProto.CreateRecordRequest{Metadata: &apiProto.RecordMetadata{Type: apiProto.RecordType_LOGIN}})
	assert.Nil(t, r2)
	st2, _ := status.FromError(e2)
	assert.Equal(t, codes.Unauthenticated, st2.Code())

	// GetRecord
	r3, e3 := h.GetRecord(context.Background(), &apiProto.GetRecordRequest{RecordId: uuid.New().String()})
	assert.Nil(t, r3)
	st3, _ := status.FromError(e3)
	assert.Equal(t, codes.Unauthenticated, st3.Code())

	// DeleteRecord
	r4, e4 := h.DeleteRecord(context.Background(), &apiProto.DeleteRecordRequest{RecordId: uuid.New().String()})
	assert.Nil(t, r4)
	st4, _ := status.FromError(e4)
	assert.Equal(t, codes.Unauthenticated, st4.Code())

	// Streams
	e5 := h.CreateRecordStream(&fakeCreateStream{ctx: context.Background()})
	st5, _ := status.FromError(e5)
	assert.Equal(t, codes.Unauthenticated, st5.Code())

	e6 := h.GetRecordStream(&apiProto.GetRecordStreamRequest{RecordId: uuid.New().String()}, &fakeGetStream{ctx: context.Background()})
	st6, _ := status.FromError(e6)
	assert.Equal(t, codes.Unauthenticated, st6.Code())
}
