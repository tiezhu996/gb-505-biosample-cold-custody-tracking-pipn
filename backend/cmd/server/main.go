package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"biosample-cold-custody-tracking/backend/internal/config"
	"biosample-cold-custody-tracking/backend/internal/router"
	"biosample-cold-custody-tracking/backend/internal/util"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	db, err := util.OpenDatabase(cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	if err := util.Migrate(db); err != nil {
		slog.Error("migrate database", "error", err)
		os.Exit(1)
	}
	if err := util.SeedDemoData(context.Background(), db); err != nil {
		slog.Error("seed demo data", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	defer redisClient.Close()
	dependencyContext, cancelDependencies := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDependencies()
	if err := redisClient.Ping(dependencyContext).Err(); err != nil {
		slog.Error("connect redis", "error", err)
		os.Exit(1)
	}
	objectStore, err := util.OpenObjectStore(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOUseSSL)
	if err != nil {
		slog.Error("configure MinIO", "error", err)
		os.Exit(1)
	}
	if err := util.EnsureObjectStore(dependencyContext, objectStore, cfg.MinIOBucket); err != nil {
		slog.Error("prepare MinIO bucket", "error", err)
		os.Exit(1)
	}
	engine, err := router.Build(db, redisClient, objectStore, cfg)
	if err != nil {
		slog.Error("build application", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr: ":" + cfg.Port, Handler: engine,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	go func() {
		slog.Info("server started", "port", cfg.Port)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			slog.Error("server stopped unexpectedly", "error", serveErr)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
}
