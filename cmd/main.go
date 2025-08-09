package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"

	"mytonstorage-backend/pkg/httpServer"
	filesRepository "mytonstorage-backend/pkg/repositories/files"
	"mytonstorage-backend/pkg/services/auth"
	filesService "mytonstorage-backend/pkg/services/files"
	providersService "mytonstorage-backend/pkg/services/providers"
	tonstorage "mytonstorage-backend/pkg/ton-storage"
	"mytonstorage-backend/pkg/workers"
	"mytonstorage-backend/pkg/workers/cleaner"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() (err error) {
	// Tools
	config := loadConfig()
	if config == nil {
		fmt.Println("failed to load configuration")
		return
	}

	logLevel := slog.LevelInfo
	if level, ok := logLevels[config.System.LogLevel]; ok {
		logLevel = level
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// TON Connect Verifier initialization
	lsCfg, err := liteclient.GetConfigFromUrl(context.Background(), config.TON.ConfigURL)
	if err != nil {
		logger.Error("failed to get liteclient config", slog.String("error", err.Error()))
		return
	}

	client := liteclient.NewConnectionPool()
	err = client.AddConnectionsFromConfig(context.Background(), lsCfg)
	if err != nil {
		logger.Error("failed to add connections from config", slog.String("error", err.Error()))
		return
	}

	api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()
	verifier := wallet.NewTonConnectVerifier(config.System.Host, config.System.AuthSessionDuration, api)

	// Metrics
	dbRequestsCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Metrics.Namespace,
			Subsystem: config.Metrics.DbSubsystem,
			Name:      "db_requests_count",
			Help:      "Db requests count",
		},
		[]string{"method", "error"},
	)

	dbRequestsDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Metrics.Namespace,
			Subsystem: config.Metrics.DbSubsystem,
			Name:      "db_requests_duration",
			Help:      "Db requests duration",
		},
		[]string{"method", "error"},
	)

	workersRunCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: config.Metrics.Namespace,
			Subsystem: config.Metrics.DbSubsystem,
			Name:      "workers_requests_count",
			Help:      "Workers requests count",
		},
		[]string{"method", "error"},
	)

	workersRunDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: config.Metrics.Namespace,
			Subsystem: config.Metrics.DbSubsystem,
			Name:      "workers_requests_duration",
			Help:      "Workers requests duration",
		},
		[]string{"method", "error"},
	)

	prometheus.MustRegister(
		dbRequestsCount,
		dbRequestsDuration,
		workersRunCount,
		workersRunDuration,
	)

	// Postgres
	connPool, err := connectPostgres(context.Background(), config, logger)
	if err != nil {
		logger.Error("failed to connect to Postgres", slog.String("error", err.Error()))
		return
	}

	// Database
	filesRepo := filesRepository.NewRepository(connPool)
	filesRepo = filesRepository.NewMetrics(dbRequestsCount, dbRequestsDuration, filesRepo)

	// Workers
	cleanerWorker := cleaner.NewWorker(filesRepo, config.System.StoreHistoryDays, logger)
	cleanerWorker = cleaner.NewMetrics(workersRunCount, workersRunDuration, cleanerWorker)

	// Clients
	_, providerClient, err := newProviderClient(context.Background(), config.TON.ConfigURL, config.System.ADNLPort, config.System.Key)
	if err != nil {
		logger.Error("failed to create provider client", slog.String("error", err.Error()))
		return
	}

	creds := tonstorage.Credentials{
		Login:    config.TONStorage.Login,
		Password: config.TONStorage.Password,
	}
	storage := tonstorage.NewClient(config.TONStorage.BaseURL, config.TONStorage.BagsDirForStorage, &creds)

	// Services
	providersSvc := providersService.NewService(providerClient, storage, config.System.MaxAllowedSpanDays, logger)

	filesSvc := filesService.NewService(filesRepo, storage, config.TONStorage.BagsDirForStorage, logger)
	filesSvc = filesService.NewCacheMiddleware(filesSvc)

	seed, err := hex.DecodeString(config.System.AuthPrivateKey)
	if err != nil {
		logger.Error("failed to decode private key", slog.String("error", err.Error()))
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(seed) != ed25519.SeedSize {
		logger.Error("invalid private key length", slog.Int("expected", ed25519.SeedSize), slog.Int("got", len(seed)))
		return fmt.Errorf("invalid private key length: expected %d, got %d", ed25519.SeedSize, len(seed))
	}

	authSvc := auth.New(verifier, ed25519.NewKeyFromSeed(seed), config.System.Host, logger)

	// Start workers
	cancelCtx, cancel := context.WithCancel(context.Background())
	workers := workers.NewWorkers(cleanerWorker, logger)
	go func() {
		if wErr := workers.Start(cancelCtx); wErr != nil {
			logger.Error("failed to start workers", slog.String("error", wErr.Error()))
			err = wErr
			return
		}
	}()

	// HTTP Server
	adminAuthTokens := strings.Split(config.System.AdminAuthTokens, ",")
	app := fiber.New()
	server := httpServer.New(
		app,
		filesSvc,
		providersSvc,
		authSvc,
		adminAuthTokens,
		config.Metrics.Namespace,
		config.Metrics.ServerSubsystem,
		logger,
	)

	server.RegisterRoutes()

	go func() {
		if err := app.Listen(":" + config.System.Port); err != nil {
			logger.Error("error starting server", slog.String("err", err.Error()))
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	cancel()

	err = app.ShutdownWithTimeout(time.Second * 5)
	if err != nil {
		logger.Error("server shut down error", slog.String("err", err.Error()))
		return err
	}

	return err
}
