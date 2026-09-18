package router

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"biosample-cold-custody-tracking/backend/internal/config"
	"biosample-cold-custody-tracking/backend/internal/handler"
	"biosample-cold-custody-tracking/backend/internal/middleware"
	"biosample-cold-custody-tracking/backend/internal/repository"
	"biosample-cold-custody-tracking/backend/internal/service"
	"biosample-cold-custody-tracking/backend/internal/util"
)

func Build(db *gorm.DB, redisClient *redis.Client, objectStore *minio.Client, cfg config.Config) (*gin.Engine, error) {
	auditRepo := repository.NewAuditRepository(db)
	userRepo := repository.NewUserRepository(db)
	storageRepo := repository.NewStorageRepository(db)
	specimenRepo := repository.NewSpecimenRepository(db)
	transferRepo := repository.NewTransferRepository(db)
	protocolRepo := repository.NewProtocolRepository(db)

	auditService := service.NewAuditService(auditRepo)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.TokenTTL)
	storageService := service.NewStorageService(storageRepo, auditService)
	specimenService := service.NewSpecimenService(specimenRepo, auditService)
	transferService := service.NewTransferService(transferRepo, specimenRepo, auditService)
	protocolService := service.NewProtocolService(protocolRepo, specimenRepo, transferRepo, auditService, objectStore, cfg.MinIOBucket)

	if err := authService.Seed(context.Background()); err != nil {
		return nil, err
	}

	authHandler := handler.NewAuthHandler(authService)
	storageHandler := handler.NewStorageHandler(storageService)
	specimenHandler := handler.NewSpecimenHandler(specimenService)
	transferHandler := handler.NewTransferHandler(transferService)
	protocolHandler := handler.NewProtocolHandler(protocolService)
	auditHandler := handler.NewAuditHandler(auditService)

	engine := gin.New()
	engine.Use(middleware.RequestContext())
	engine.Use(middleware.ErrorHandler())
	engine.Use(middleware.NewRateLimiter(redisClient, cfg.RateLimit, cfg.RateWindow).Handler())

	engine.GET("/healthz", healthHandler(db, redisClient, objectStore, cfg.MinIOBucket))

	api := engine.Group("/api")
	api.POST("/auth/login", authHandler.Login)
	secured := api.Group("")
	secured.Use(middleware.Auth(cfg.JWTSecret))
	secured.GET("/auth/me", authHandler.Me)
	secured.POST("/auth/logout", authHandler.Logout)

	secured.GET("/storage-containers", storageHandler.List)
	secured.GET("/storage-containers/:id", storageHandler.Get)
	secured.POST("/storage-containers", middleware.RequirePermission("storage:write"), storageHandler.Create)
	secured.PATCH("/storage-containers/:id", middleware.RequirePermission("storage:write"), storageHandler.Update)

	secured.GET("/specimens", specimenHandler.List)
	secured.GET("/specimens/overview", specimenHandler.Overview)
	secured.GET("/specimens/:id", specimenHandler.Get)
	secured.POST("/specimens", middleware.RequirePermission("specimen:create"), specimenHandler.Create)
	secured.PATCH("/specimens/:id", middleware.RequirePermission("specimen:update"), specimenHandler.Update)
	secured.POST("/specimens/:id/transition", middleware.RequirePermission("specimen:transition"), specimenHandler.Transition)

	secured.GET("/custody-transfers", transferHandler.List)
	secured.GET("/custody-transfers/:id", transferHandler.Get)
	secured.POST("/custody-transfers", middleware.RequirePermission("transfer:prepare"), transferHandler.Create)
	secured.POST("/custody-transfers/:id/resolve", middleware.RequirePermission("transfer:resolve"), transferHandler.Resolve)

	secured.GET("/protocol-reviews", protocolHandler.List)
	secured.GET("/protocol-reviews/:id", protocolHandler.Get)
	secured.POST("/protocol-reviews", middleware.RequirePermission("protocol:review"), protocolHandler.Review)
	secured.GET("/audit-logs", middleware.RequirePermission("audit:read"), auditHandler.List)

	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     gin.H{"code": "ROUTE_NOT_FOUND", "message": "接口不存在"},
			"requestId": c.GetString("requestId"),
		})
	})
	return engine, nil
}

func healthHandler(db *gorm.DB, redisClient *redis.Client, objectStore *minio.Client, bucket string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		components := gin.H{"postgres": "ok", "redis": "ok", "minio": "ok"}
		healthy := true
		if err := util.Ready(ctx, db); err != nil {
			components["postgres"] = "unavailable"
			healthy = false
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			components["redis"] = "unavailable"
			healthy = false
		}
		if err := util.ObjectStoreReady(ctx, objectStore, bucket); err != nil {
			components["minio"] = "unavailable"
			healthy = false
		}
		status := http.StatusOK
		label := "ok"
		if !healthy {
			status = http.StatusServiceUnavailable
			label = "degraded"
		}
		c.JSON(status, gin.H{
			"status":     label,
			"service":    "biosample-cold-custody-tracking",
			"components": components,
			"checkedAt":  time.Now().UTC(),
		})
	}
}
