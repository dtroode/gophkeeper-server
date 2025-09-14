package router

import (
	"testing"

	"github.com/dtroode/gophkeeper-server/internal/mocks"
	"github.com/dtroode/gophkeeper-server/internal/testutil"
)

func TestRouter_Register(t *testing.T) {
	t.Parallel()

	ctxMgr := mocks.NewContextManager(t)
	lg := testutil.MakeNoopLogger()

	r := New(nil, nil, nil, ctxMgr, lg)
	s := r.Register()
	if s == nil {
		t.Fatalf("expected non-nil grpc server")
	}
}
