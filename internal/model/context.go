package model

import (
	"context"

	"github.com/google/uuid"
)

type ContextManager interface {
	SetUserIDToContext(ctx context.Context, userID uuid.UUID) context.Context
	GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool)
}
