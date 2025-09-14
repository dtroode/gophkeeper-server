package handler

import (
    "context"
    "testing"

    apiProto "github.com/dtroode/gophkeeper-api/proto"
    "github.com/dtroode/gophkeeper-server/internal/model"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// fakeCreateStream2 lets us feed Recv responses.
type fakeCreateStream2 struct {
    apiProto.API_CreateRecordStreamServer
    ctx  context.Context
    q    []*apiProto.CreateRecordStreamRequest
    sent *apiProto.CreateRecordStreamResponse
}

func (f *fakeCreateStream2) Context() context.Context { return f.ctx }
func (f *fakeCreateStream2) Recv() (*apiProto.CreateRecordStreamRequest, error) {
    if len(f.q) == 0 {
        return nil, ioEOF
    }
    r := f.q[0]
    f.q = f.q[1:]
    return r, nil
}
func (f *fakeCreateStream2) SendAndClose(resp *apiProto.CreateRecordStreamResponse) error {
    f.sent = resp
    return nil
}
func (f *fakeCreateStream2) SendMsg(m interface{}) error     { return nil }
func (f *fakeCreateStream2) RecvMsg(m interface{}) error     { return nil }
func (f *fakeCreateStream2) SendHeader(md metadata.MD) error { return nil }
func (f *fakeCreateStream2) SetTrailer(md metadata.MD)       {}
func (f *fakeCreateStream2) SetHeader(md metadata.MD) error  { return nil }

var ioEOF = status.Error(codes.Canceled, "eof")

func TestProtoStreamReader_Metadata_Success(t *testing.T) {
    uid := uuid.New().String()
    req := &apiProto.CreateRecordStreamRequest{Request: &apiProto.CreateRecordStreamRequest_Metadata{Metadata: &apiProto.RecordMetadata{
        Name: "n", Description: "d", Type: apiProto.RecordType_BINARY, EncryptedKey: []byte("k"), Alg: "a", ChunkSize: 8, RequestId: uid,
    }}}
    s := &fakeCreateStream2{ctx: context.Background(), q: []*apiProto.CreateRecordStreamRequest{req}}
    r := &protoStreamReader{stream: s}
    out, err := r.Recv()
    assert.NoError(t, err)
    assert.True(t, out.IsMetadata)
    assert.Equal(t, "n", out.Metadata.Name)
    assert.Equal(t, model.RecordTypeBinary, out.Metadata.Type)
    assert.Equal(t, 8, out.Metadata.ChunkSize)
}

func TestProtoStreamReader_Metadata_InvalidType(t *testing.T) {
    req := &apiProto.CreateRecordStreamRequest{Request: &apiProto.CreateRecordStreamRequest_Metadata{Metadata: &apiProto.RecordMetadata{Type: 999}}}
    s := &fakeCreateStream2{ctx: context.Background(), q: []*apiProto.CreateRecordStreamRequest{req}}
    r := &protoStreamReader{stream: s}
    out, err := r.Recv()
    assert.Error(t, err)
    assert.Nil(t, out)
}

func TestProtoStreamReader_Metadata_InvalidRequestID(t *testing.T) {
    req := &apiProto.CreateRecordStreamRequest{Request: &apiProto.CreateRecordStreamRequest_Metadata{Metadata: &apiProto.RecordMetadata{Type: apiProto.RecordType_LOGIN, RequestId: "bad"}}}
    s := &fakeCreateStream2{ctx: context.Background(), q: []*apiProto.CreateRecordStreamRequest{req}}
    r := &protoStreamReader{stream: s}
    out, err := r.Recv()
    st, _ := status.FromError(err)
    assert.Equal(t, codes.InvalidArgument, st.Code())
    assert.Nil(t, out)
}

func TestProtoStreamReader_DataChunk(t *testing.T) {
    req := &apiProto.CreateRecordStreamRequest{Request: &apiProto.CreateRecordStreamRequest_DataChunk{DataChunk: []byte{1,2,3}}}
    s := &fakeCreateStream2{ctx: context.Background(), q: []*apiProto.CreateRecordStreamRequest{req}}
    r := &protoStreamReader{stream: s}
    out, err := r.Recv()
    assert.NoError(t, err)
    assert.False(t, out.IsMetadata)
    assert.Equal(t, []byte{1,2,3}, out.DataChunk)
}

// fakeGetStream2 captures proto responses sent by protoStreamWriter.
type fakeGetStream2 struct {
    apiProto.API_GetRecordStreamServer
    ctx  context.Context
    sent []*apiProto.GetRecordStreamResponse
}
func (f *fakeGetStream2) Context() context.Context { return f.ctx }
func (f *fakeGetStream2) Send(resp *apiProto.GetRecordStreamResponse) error {
    f.sent = append(f.sent, resp)
    return nil
}
func (f *fakeGetStream2) SendMsg(m interface{}) error     { return nil }
func (f *fakeGetStream2) RecvMsg(m interface{}) error     { return nil }
func (f *fakeGetStream2) SendHeader(md metadata.MD) error { return nil }
func (f *fakeGetStream2) SetTrailer(md metadata.MD)       {}
func (f *fakeGetStream2) SetHeader(md metadata.MD) error  { return nil }

func TestProtoStreamWriter_MetadataAndChunk(t *testing.T) {
    s := &fakeGetStream2{ctx: context.Background()}
    w := &protoStreamWriter{stream: s}

    // send metadata
    err := w.Send(&model.GetRecordStreamResponse{Metadata: &model.RecordMetadata{ID: uuid.New().String(), Name: "n", Description: "d", Type: model.RecordTypeCard, EncryptedKey: []byte("k"), Alg: "a", ChunkSize: 4}})
    assert.NoError(t, err)
    // send chunk
    err = w.Send(&model.GetRecordStreamResponse{DataChunk: []byte{9}, IsLastChunk: true})
    assert.NoError(t, err)

    if assert.Len(t, s.sent, 2) {
        // first is metadata
        _, ok := s.sent[0].Response.(*apiProto.GetRecordStreamResponse_Metadata)
        assert.True(t, ok)
        assert.False(t, s.sent[0].IsLastChunk)
        // second is chunk
        dc, ok := s.sent[1].Response.(*apiProto.GetRecordStreamResponse_DataChunk)
        assert.True(t, ok)
        assert.Equal(t, []byte{9}, dc.DataChunk)
        assert.True(t, s.sent[1].IsLastChunk)
    }
}

func TestConverters_Defaults(t *testing.T) {
    // convertRecordTypeToProto default -> BINARY
    rt := convertRecordTypeToProto("weird")
    assert.Equal(t, apiProto.RecordType_BINARY, rt)

    // convertRecordToMetadata default type -> UNKNOWN
    meta := (&Record{}).convertRecordToMetadata(model.Record{Type: "unknown"})
    assert.Equal(t, apiProto.RecordType_UNKNOWN, meta.Type)
}

