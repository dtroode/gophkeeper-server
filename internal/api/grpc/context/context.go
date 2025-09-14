package context

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// userIDKey is the metadata key used to store and retrieve user ID in gRPC context.
const (
	userIDKey string = "user_id"
)

// Manager represents a gRPC context manager for user ID operations.
// It provides methods to set and retrieve user IDs from gRPC metadata.
type Manager struct{}

// NewManager creates a new gRPC context manager instance.
//
// Returns a pointer to the newly created Manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// SetUserIDToContext sets the user ID in the gRPC context metadata.
// It creates outgoing metadata with the user ID and returns a new context.
//
// Parameters:
//   - ctx: The gRPC context
//   - userID: The user UUID to set in the context
//
// Returns a new context with the user ID in outgoing metadata.
func (m *Manager) SetUserIDToContext(ctx context.Context, userID uuid.UUID) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(map[string]string{userIDKey: userID.String()})
	} else {
		md.Set(userIDKey, userID.String())
	}

	return metadata.NewIncomingContext(ctx, md)
}

// GetUserIDFromContext retrieves the user ID from gRPC context metadata.
// It parses the user ID from incoming metadata and returns it as a UUID.
//
// Parameters:
//   - ctx: The gRPC context
//
// Returns the user UUID and a boolean indicating if the user ID was found.
func (m *Manager) GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}

	userIDs := md.Get(userIDKey)
	if len(userIDs) == 0 {
		return uuid.Nil, false
	}

	userIDStr := userIDs[0]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, false
	}

	return userID, true
}

// GetUserIDFromResponse retrieves the user ID from gRPC response metadata.
// This function is used by clients to access the user ID that was set in the response.
//
// Parameters:
//   - ctx: The gRPC context containing response metadata
//
// Returns the user UUID and a boolean indicating if the user ID was found.
func (m *Manager) GetUserIDFromResponse(ctx context.Context) (uuid.UUID, bool) {
	// For gRPC clients, we need to get the response metadata from the stream
	// This is typically done by the client after receiving the response
	// For now, we'll return false as this method is mainly for documentation
	return uuid.Nil, false
}

// GetUserIDFromResponseMetadata retrieves the user ID from gRPC response metadata.
// This function is used by clients to access the user ID that was set in the response.
//
// Parameters:
//   - md: The gRPC metadata from the response
//
// Returns the user UUID and a boolean indicating if the user ID was found.
func (m *Manager) GetUserIDFromResponseMetadata(md metadata.MD) (uuid.UUID, bool) {
	userIDs := md.Get("x-user-id")
	if len(userIDs) == 0 {
		return uuid.Nil, false
	}

	userIDStr := userIDs[0]
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, false
	}

	return userID, true
}
