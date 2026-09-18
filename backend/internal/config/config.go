package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
	JWTSecret      string
	TokenTTL       time.Duration
	RateLimit      int64
	RateWindow     time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Port:           env("PORT", "8080"),
		DatabaseURL:    env("DATABASE_URL", "postgres://biosample:biosample_dev_password@localhost:5432/biosample_custody?sslmode=disable"),
		RedisAddr:      env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		MinIOEndpoint:  env("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: env("MINIO_ACCESS_KEY", "biosample-admin"),
		MinIOSecretKey: env("MINIO_SECRET_KEY", "biosample-minio-password"),
		MinIOBucket:    env("MINIO_BUCKET", "protocol-documents"),
		MinIOUseSSL:    boolEnv("MINIO_USE_SSL", false),
		JWTSecret:      env("JWT_SECRET", "local-development-secret-change-me"),
		TokenTTL:       durationEnv("TOKEN_TTL", 12*time.Hour),
		RateLimit:      int64Env("RATE_LIMIT", 180),
		RateWindow:     durationEnv("RATE_WINDOW", time.Minute),
	}

	redisDB, err := strconv.Atoi(env("REDIS_DB", "0"))
	if err != nil || redisDB < 0 {
		return Config{}, fmt.Errorf("invalid REDIS_DB")
	}
	cfg.RedisDB = redisDB

	if strings.TrimSpace(cfg.JWTSecret) == "" {
		return Config{}, fmt.Errorf("JWT_SECRET cannot be empty")
	}
	if len(cfg.JWTSecret) < 16 {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 16 characters")
	}
	if strings.TrimSpace(cfg.MinIOEndpoint) == "" || strings.Contains(cfg.MinIOEndpoint, "://") {
		return Config{}, fmt.Errorf("MINIO_ENDPOINT must be host:port without a URL scheme")
	}
	if strings.TrimSpace(cfg.MinIOAccessKey) == "" || strings.TrimSpace(cfg.MinIOSecretKey) == "" {
		return Config{}, fmt.Errorf("MinIO credentials cannot be empty")
	}
	if strings.TrimSpace(cfg.MinIOBucket) == "" {
		return Config{}, fmt.Errorf("MINIO_BUCKET cannot be empty")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func int64Env(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
