package main

import (
	"context"
	"drone-delivery/config"
	"drone-delivery/internal/admin"
	pgmigrate "drone-delivery/internal/repo/postgres"
	"drone-delivery/internal/auth"
	"drone-delivery/internal/common"
	"drone-delivery/internal/delivery"
	"drone-delivery/internal/drone"
	"drone-delivery/internal/job"
	"drone-delivery/internal/jwt"
	"drone-delivery/internal/order"
	"drone-delivery/internal/redis"
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
	goredis "github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
)

type AppContext struct {
	DB     *sqlx.DB
	Config *config.Config
	Redis  *goredis.Client
	Router *gin.Engine

	// Infrastructure
	JWTService       *jwt.Service
	DroneCache       *redis.DroneLocationCache
	IdempotencyStore *redis.IdempotencyStore
	RateLimiter      *redis.RateLimiter
	MapboxClient     *common.MapboxClient

	OrderHandler *order.Handler
	DroneHandler *drone.Handler
	JobHandler   *job.Handler
	AdminHandler *admin.Handler
	AuthHandler  *auth.Handler

	OrderService order.Service
	DroneService drone.Service
	JobService   job.Service
	AdminService admin.Service

	OrderRepo order.Repository
	DroneRepo drone.Repository
	JobRepo   job.Repository
}

// orderQueryAdapter bridges order.Service to drone.OrderQuerier so drone handler
// doesn't import the order package (avoiding circular deps).
type orderQueryAdapter struct {
	svc order.Service
}

func (a *orderQueryAdapter) GetByDroneID(ctx context.Context, droneID string) (any, error) {
	return a.svc.GetByDroneID(ctx, droneID)
}

func wireApp(cfg *config.Config) (*AppContext, error) {
	// ── Postgres ──
	db, err := sqlx.Connect("postgres", cfg.Postgres.DSN())
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	if err := pgmigrate.RunMigrationsUp(db); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	// ── Redis ──
	var rdb *goredis.Client
	if cfg.Redis.URL != "" {
		opts, err := goredis.ParseURL(cfg.Redis.URL)
		if err != nil {
			return nil, fmt.Errorf("redis parse url: %w", err)
		}
		rdb = goredis.NewClient(opts)
	} else {
		rdb = goredis.NewClient(&goredis.Options{
			Addr:     cfg.Redis.Addr(),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
	}
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	// ── Infrastructure ──
	jwtService := jwt.NewService(cfg.JWT.Secret, cfg.JWT.ExpiryHours)
	droneCache := redis.NewDroneLocationCache(rdb, cfg.Drone.LocationCacheTTLSec)
	idempotencyStore := redis.NewIdempotencyStore(rdb, cfg.Drone.IdempotencyTTLSec)
	rateLimiter := redis.NewRateLimiter(rdb, cfg.RateLimiter.MaxRequests, cfg.RateLimiter.WindowSeconds)
	mapboxClient := common.NewMapboxClient(cfg.Mapbox.BaseURL, cfg.Mapbox.AccessToken)

	// ── Repositories ──
	orderRepo := order.NewRepository()
	droneRepo := drone.NewRepository()
	jobRepo := job.NewRepository()
	deliveryRepo := delivery.NewRepository(orderRepo, jobRepo, droneRepo)

	// ── Services ──
	orderService := order.NewOrderService(orderRepo, db, order.ZoneConfig{
		CenterLat: cfg.Zone.CenterLat,
		CenterLng: cfg.Zone.CenterLng,
		RadiusKM:  cfg.Zone.RadiusKM,
	}, mapboxClient)

	zoneCenter := common.NewLocation(cfg.Zone.CenterLat, cfg.Zone.CenterLng)
	droneService := drone.NewDroneService(droneRepo, db, droneCache, zoneCenter, cfg.Zone.RadiusKM)
	jobService := job.NewService(jobRepo, db)
	deliveryService := delivery.NewService(db, deliveryRepo)
	adminService := admin.NewService(orderService, droneService, deliveryService)
	authService := auth.NewAuthService(jwtService)

	// ── Handlers ──

	authHandler := auth.NewHandler(authService)
	orderHandler := order.NewHandler(orderService, deliveryService, droneService)
	droneHandler := drone.NewHandler(droneService, &orderQueryAdapter{svc: orderService}, deliveryService)
	jobHandler := job.NewHandler(jobService, deliveryService)
	adminHandler := admin.NewHandler(adminService, orderService, droneService)

	return &AppContext{
		Config: cfg,
		DB:     db,
		Redis:  rdb,
		Router: gin.Default(),

		JWTService:       jwtService,
		DroneCache:       droneCache,
		IdempotencyStore: idempotencyStore,
		RateLimiter:      rateLimiter,
		MapboxClient:     mapboxClient,

		OrderRepo: orderRepo,
		DroneRepo: droneRepo,
		JobRepo:   jobRepo,

		OrderService: orderService,
		DroneService: droneService,
		JobService:   jobService,
		AdminService: adminService,

		AuthHandler:  authHandler,
		OrderHandler: orderHandler,
		DroneHandler: droneHandler,
		JobHandler:   jobHandler,
		AdminHandler: adminHandler,
	}, nil
}
func (a *AppContext) Close() {
	a.DB.Close()
	a.Redis.Close()
}

func (a *AppContext) healthCheck(c *gin.Context) {
	ctx := c.Request.Context()
	checks := map[string]string{}
	healthy := true

	if err := a.DB.PingContext(ctx); err != nil {
		checks["postgres"] = err.Error()
		healthy = false
	} else {
		checks["postgres"] = "ok"
	}

	if err := a.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = err.Error()
		healthy = false
	} else {
		checks["redis"] = "ok"
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, gin.H{
		"status": checks,
	})
}
