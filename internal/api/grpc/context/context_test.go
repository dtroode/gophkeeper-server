package context

import (
	stdctx "context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestManager_SetAndGetUserID(t *testing.T) {
	m := NewManager()
	uid := uuid.New()
	ctx := m.SetUserIDToContext(stdctx.Background(), uid)

	got, ok := m.GetUserIDFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, uid, got)
}

func TestManager_GetUserID_NotFound(t *testing.T) {
	m := NewManager()
	_, ok := m.GetUserIDFromContext(stdctx.Background())
	assert.False(t, ok)
}

func TestManager_GetUserIDFromResponseMetadata(t *testing.T) {
	m := NewManager()
	uid := uuid.New()
	md := metadata.New(map[string]string{"x-user-id": uid.String()})
	got, ok := m.GetUserIDFromResponseMetadata(md)
	assert.True(t, ok)
	assert.Equal(t, uid, got)
}

func TestManager_SetUserID_WithExistingMetadata(t *testing.T) {
    m := NewManager()
    uid := uuid.New()
    baseMD := metadata.New(map[string]string{"x-trace-id": "t"})
    ctxWithMD := metadata.NewIncomingContext(stdctx.Background(), baseMD)

    ctx := m.SetUserIDToContext(ctxWithMD, uid)
    got, ok := m.GetUserIDFromContext(ctx)
    assert.True(t, ok)
    assert.Equal(t, uid, got)
}

func TestManager_GetUserID_InvalidUUID(t *testing.T) {
    m := NewManager()
    md := metadata.New(map[string]string{"user_id": "not-a-uuid"})
    ctx := metadata.NewIncomingContext(stdctx.Background(), md)
    _, ok := m.GetUserIDFromContext(ctx)
    assert.False(t, ok)
}
