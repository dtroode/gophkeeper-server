//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/dtroode/gophkeeper-server/internal/model"
	repo "github.com/dtroode/gophkeeper-server/internal/repository/postgres"
)

var dsn string

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        "postgres:15-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "password",
				"POSTGRES_DB":       "gophkeeper_test",
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		panic(err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		panic(err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		panic(err)
	}
	dsn = fmt.Sprintf("postgres://postgres:password@%s:%s/gophkeeper_test?sslmode=disable", host, port.Port())

	code := m.Run()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

func TestRepositories_CRUD(t *testing.T) {
	ctx := context.Background()
	conn, err := repo.NewConection(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	t.Run("user_repository", func(t *testing.T) {
		ur := repo.NewUserRepository(conn)
		u := model.User{
			ID:        uuid.New(),
			Email:     "user@example.com",
			StoredKey: make([]byte, 32),
			ServerKey: make([]byte, 32),
			SaltRoot:  []byte("salt"),
			KDF:       []byte("{}"),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		saved, err := ur.Create(ctx, u)
		require.NoError(t, err)
		require.Equal(t, u.ID, saved.ID)

		byEmail, err := ur.GetByEmail(ctx, u.Email)
		require.NoError(t, err)
		require.Equal(t, u.ID, byEmail.ID)

		byID, err := ur.GetByID(ctx, u.ID)
		require.NoError(t, err)
		require.Equal(t, u.Email, byID.Email)
	})

	t.Run("record_repository", func(t *testing.T) {
		rr := repo.NewRecordRepository(conn)
		ur := repo.NewUserRepository(conn)
		owner := uuid.New()
		_, err := ur.Create(ctx, model.User{ID: owner, Email: "owner@example.com", StoredKey: make([]byte, 32), ServerKey: make([]byte, 32), SaltRoot: []byte("salt"), KDF: []byte("{}"), CreatedAt: time.Now(), UpdatedAt: time.Now()})
		require.NoError(t, err)
		r := model.Record{
			ID:            uuid.New(),
			OwnerID:       owner,
			Name:          "n",
			Description:   "d",
			EncryptedKey:  []byte("ek"),
			Alg:           "alg",
			Type:          model.RecordTypeNote,
			EncryptedData: []byte("ed"),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		saved, err := rr.Create(ctx, r)
		require.NoError(t, err)
		require.Equal(t, r.ID, saved.ID)

		got, err := rr.GetByID(ctx, r.ID)
		require.NoError(t, err)
		require.Equal(t, r.OwnerID, got.OwnerID)

		list, err := rr.GetByUserID(ctx, owner)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(list), 1)

		require.NoError(t, rr.SoftDelete(ctx, r.ID))
	})

	t.Run("login_repository", func(t *testing.T) {
		lr := repo.NewLoginRepository(conn)
		pl := model.PendingLogin{
			SessionID:   uuid.NewString(),
			Login:       "user@example.com",
			ClientNonce: []byte{1},
			ServerNonce: []byte{2},
			ExpiresAt:   time.Now().Add(time.Hour),
			Consumed:    false,
		}
		require.NoError(t, lr.Create(ctx, pl))
		got, err := lr.GetBySessionID(ctx, pl.SessionID)
		require.NoError(t, err)
		require.Equal(t, pl.Login, got.Login)
		require.NoError(t, lr.Consume(ctx, pl.SessionID))
	})
}

func TestRecordRepository_IdempotentCreateAndDeltas(t *testing.T) {
	ctx := context.Background()
	conn, err := repo.NewConection(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ur := repo.NewUserRepository(conn)
	rr := repo.NewRecordRepository(conn)

	owner := uuid.New()
	_, err = ur.Create(ctx, model.User{ID: owner, Email: "o@example.com", StoredKey: make([]byte, 32), ServerKey: make([]byte, 32), SaltRoot: []byte("salt"), KDF: []byte("{}"), CreatedAt: time.Now(), UpdatedAt: time.Now()})
	require.NoError(t, err)

	reqID := uuid.New()
	r1 := model.Record{
		ID:            uuid.New(),
		OwnerID:       owner,
		Name:          "r1",
		Description:   "d1",
		EncryptedKey:  []byte("ek1"),
		Alg:           "alg",
		Type:          model.RecordTypeLogin,
		EncryptedData: []byte("ed1"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		RequestID:     reqID,
	}
	saved1, err := rr.Create(ctx, r1)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	r2 := model.Record{
		ID:            uuid.New(),
		OwnerID:       owner,
		Name:          "r2",
		Description:   "d2",
		EncryptedKey:  []byte("ek2"),
		Alg:           "alg",
		Type:          model.RecordTypeLogin,
		EncryptedData: []byte("ed2"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		RequestID:     reqID,
	}
	saved2, err := rr.Create(ctx, r2)
	require.NoError(t, err)
	require.Equal(t, saved1.ID, saved2.ID)

	time.Sleep(10 * time.Millisecond)

	r3 := model.Record{
		ID:            uuid.New(),
		OwnerID:       owner,
		Name:          "r3",
		Description:   "d3",
		EncryptedKey:  []byte("ek3"),
		Alg:           "alg",
		Type:          model.RecordTypeNote,
		EncryptedData: []byte("ed3"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	saved3, err := rr.Create(ctx, r3)
	require.NoError(t, err)

	uaList, err := rr.GetUpdatedAfter(ctx, owner, saved1.UpdatedAt)
	require.NoError(t, err)
	require.NotEmpty(t, uaList)

	uaType, err := rr.GetUpdatedAfterByType(ctx, owner, model.RecordTypeNote, saved1.UpdatedAt)
	require.NoError(t, err)
	require.NotEmpty(t, uaType)

	_, err = rr.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, model.ErrNotFound)

	err = rr.SoftDelete(ctx, saved3.ID)
	require.NoError(t, err)

	tombs, err := rr.GetDeletedAfter(ctx, owner, time.Time{})
	require.NoError(t, err)
	require.NotEmpty(t, tombs)

	tombsType, err := rr.GetDeletedAfterByType(ctx, owner, model.RecordTypeNote, time.Time{})
	require.NoError(t, err)
	require.NotEmpty(t, tombsType)

	err = rr.SoftDelete(ctx, uuid.New())
	require.ErrorIs(t, err, model.ErrNotFound)
}
