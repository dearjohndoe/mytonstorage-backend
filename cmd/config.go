package main

import (
	"crypto/ed25519"
	"log"
	"log/slog"
	"time"

	"github.com/caarlos0/env/v11"
)

var logLevels = map[uint8]slog.Level{
	0: slog.LevelDebug,
	1: slog.LevelInfo,
	2: slog.LevelWarn,
	3: slog.LevelError,
}

type System struct {
	Port                      string             `env:"SYSTEM_PORT" envDefault:"9090"`
	Host                      string             `env:"SYSTEM_HOST" required:"true"`
	Key                       ed25519.PrivateKey `env:"SYSTEM_KEY" required:"false"`
	AuthPrivateKey            string             `env:"SYSTEM_PRIVATE_KEY" required:"true"`
	AuthSessionDuration       time.Duration      `env:"SYSTEM_AUTH_SESSION_DURATION" envDefault:"24h"`
	ADNLPort                  string             `env:"SYSTEM_ADNL_PORT" envDefault:"16167"`
	AdminAuthTokens           string             `env:"SYSTEM_ADMIN_AUTH_TOKENS" envDefault:""`
	LogLevel                  uint8              `env:"SYSTEM_LOG_LEVEL" envDefault:"1"` // 0 - debug, 1 - info, 2 - warn, 3 - error
	StoreHistoryDays          int                `env:"SYSTEM_STORE_HISTORY_DAYS" envDefault:"90"`
	UnpaidFilesLifetime       time.Duration      `env:"SYSTEM_UNPAID_FILES_LIFETIME" envDefault:"20m"`
	PaidFilesLifetime         time.Duration      `env:"SYSTEM_PAID_FILES_LIFETIME" envDefault:"48h"`
	UnpaidFilesLifetimePublic time.Duration      `env:"SYSTEM_UNPAID_FILES_LIFETIME_PUBLIC" envDefault:"15m"`
	MaxAllowedSpanDays        uint32             `env:"SYSTEM_MAX_ALLOWED_SPAN_DAYS" envDefault:"7"`
}

type Metrics struct {
	Namespace        string `env:"NAMESPACE" default:"ton-storage"`
	ServerSubsystem  string `env:"SERVER_SUBSYSTEM" default:"mtpo-server"`
	WorkersSubsystem string `env:"WORKERS_SUBSYSTEM" default:"mtpo-workers"`
	DbSubsystem      string `env:"DB_SUBSYSTEM" default:"mtpo-db"`
}

type TONStorage struct {
	BaseURL           string `env:"TON_STORAGE_BASE_URL" required:"true"`
	BagsDirForStorage string `env:"BAGS_DIR_FOR_STORAGE" required:"true"`
	Login             string `env:"TON_STORAGE_LOGIN" required:"true"`
	Password          string `env:"TON_STORAGE_PASSWORD" required:"true"`
}

type TON struct {
	ConfigURL string `env:"TON_CONFIG_URL" required:"true" envDefault:"https://ton-blockchain.github.io/global.config.json"`
}

type Postgress struct {
	Host     string `env:"DB_HOST" required:"true"`
	Port     string `env:"DB_PORT" required:"true"`
	User     string `env:"DB_USER" required:"true"`
	Password string `env:"DB_PASSWORD" required:"true"`
	Name     string `env:"DB_NAME" required:"true"`
}

type Config struct {
	System     System
	TONStorage TONStorage
	Metrics    Metrics
	TON        TON
	DB         Postgress
}

func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Parse(&cfg.System); err != nil {
		log.Fatalf("Failed to parse system config: %v", err)
	}
	if err := env.Parse(&cfg.TONStorage); err != nil {
		log.Fatalf("Failed to parse TONStorage config: %v", err)
	}
	if err := env.Parse(&cfg.Metrics); err != nil {
		log.Fatalf("Failed to parse metrics config: %v", err)
	}
	if err := env.Parse(&cfg.DB); err != nil {
		log.Fatalf("Failed to parse db config: %v", err)
	}
	if err := env.Parse(&cfg.TON); err != nil {
		log.Fatalf("Failed to parse TON config: %v", err)
	}

	if cfg.System.Key == nil {
		_, priv, _ := ed25519.GenerateKey(nil)
		key := priv.Seed()
		cfg.System.Key = ed25519.NewKeyFromSeed(key)
	}

	return cfg
}
