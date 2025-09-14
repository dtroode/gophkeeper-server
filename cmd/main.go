package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcctx "github.com/dtroode/gophkeeper-server/internal/api/grpc/context"
	"github.com/dtroode/gophkeeper-server/internal/api/grpc/router"
	grpcServer "github.com/dtroode/gophkeeper-server/internal/api/grpc/server"
	"github.com/dtroode/gophkeeper-server/internal/config"
	"github.com/dtroode/gophkeeper-server/internal/logger"
	"github.com/dtroode/gophkeeper-server/internal/model"
	"github.com/dtroode/gophkeeper-server/internal/repository/postgres"
	"github.com/dtroode/gophkeeper-server/internal/server"
	"github.com/dtroode/gophkeeper-server/internal/service"
	storage "github.com/dtroode/gophkeeper-server/internal/storage/minio"
	"github.com/dtroode/gophkeeper-server/internal/token"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	authmodel "github.com/dtroode/gophkeeper-auth/model"
)

var (
	buildVersion = "N/A" // set by ldflags
	buildDate    = "N/A" // set by ldflags
	buildCommit  = "N/A" // set by ldflags
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)
	defer stop()

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
	logger := logger.New(cfg.LogLevel)

	db, err := postgres.NewConection(ctx, cfg.Database.DSN)
	if err != nil {
		logger.Fatal("failed to initialize storage", "error", err)
	}
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	signupRepo := postgres.NewSignupRepository(db)
	loginRepo := postgres.NewLoginRepository(db)
	refreshTokenRepo := postgres.NewRefreshTokenRepository(db)
	tokenManager := token.NewJWT(cfg.JWT.Secret)

	kdf := authmodel.NewKDFParams(cfg.KDF.Time, cfg.KDF.MemKiB, cfg.KDF.Par)

	authService := service.NewAuth(userRepo, signupRepo, loginRepo, refreshTokenRepo, logger, tokenManager, kdf)
	tokenService := service.NewTokenService(tokenManager, refreshTokenRepo, logger)
	ctxMgr := grpcctx.NewManager()

	minioClient, err := minio.New(cfg.Storage.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
		Secure: cfg.Storage.UseSSL,
	})
	if err != nil {
		logger.Fatal("failed to create minio client", "error", err)
	}
	storageClient, err := storage.NewClient(ctx, minioClient, cfg.Storage.Bucket)
	if err != nil {
		logger.Fatal("failed to initialize storage client", "error", err)
	}

	recordRepo := postgres.NewRecordRepository(db)
	recordService := service.NewRecord(recordRepo, userRepo, storageClient, logger)

	grpcServer := registerGRPCServer(ctx, logger, authService, recordService, tokenService, ctxMgr, fmt.Sprintf(":%s", cfg.GRPC.Port))

	var sl model.SecurityLayer

	if cfg.GRPC.EnableHTTPS {
		sl = server.NewTLSListener(cfg.GRPC.CertFileName, cfg.GRPC.PrivateKeyFileName)
	} else {
		sl = server.NewPlainListener()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func(s model.Server) {
		defer wg.Done()
		logger.Info("Starting server on", "address", s.Address())
		err := s.Start(sl)
		if err != nil {
			logger.Error("failed to start server", "error", err)
		}
	}(grpcServer)

	logAppVersion()

	<-ctx.Done()
	logger.Info("received interruption signal, shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := grpcServer.Stop(shutdownCtx); err != nil {
		logger.Error("error during server shutdown", "error", err, "address", grpcServer.Address())
	}

	wg.Wait()
	logger.Info("shutdown complete")
}

func logAppVersion() {
	tmpl := `
Build version: %s
Build date: %s
Build commit: %s
`

	fmt.Printf(tmpl, buildVersion, buildDate, buildCommit)
}

func registerGRPCServer(
	ctx context.Context,
	logger *logger.Logger,
	authService *service.Auth,
	recordService *service.Record,
	tokenService *service.TokenService,
	ctxMgr model.ContextManager,
	addr string,
) *grpcServer.GRPCServer {
	r := router.New(authService, recordService, tokenService, ctxMgr, logger)
	s := r.Register()

	reflection.Register(s)

	return grpcServer.NewGRPCServer(s, addr)
}

func shutdownGRPCServer(ctx context.Context, logger *logger.Logger, grpcServer *grpc.Server, listener net.Listener) {
	logger.Info("shutting down gRPC server")
	grpcServer.GracefulStop()
	listener.Close()
}
