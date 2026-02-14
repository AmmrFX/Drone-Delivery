package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"drone-delivery/internal/admin"
	"drone-delivery/internal/auth"
	"drone-delivery/internal/common"
	"drone-delivery/internal/delivery"
	"drone-delivery/internal/drone"
	"drone-delivery/internal/job"
	jwtpkg "drone-delivery/internal/jwt"
	"drone-delivery/internal/middleware"
	"drone-delivery/internal/order"
	"drone-delivery/internal/redis"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	goredis "github.com/redis/go-redis/v9"
)

// testApp holds the wired application for integration tests.
type testApp struct {
	DB     *sqlx.DB
	Redis  *goredis.Client
	Router *gin.Engine
	JWT    *jwtpkg.Service
}

// orderQueryAdapter bridges order.Service to drone.OrderQuerier.
type orderQueryAdapter struct {
	svc order.Service
}

func (a *orderQueryAdapter) GetByDroneID(ctx context.Context, droneID string) (any, error) {
	return a.svc.GetByDroneID(ctx, droneID)
}

const (
	zoneCenter  = 24.7136
	zoneCenterL = 46.6753
	zoneRadius  = 50.0
)

func skipIfNoInfra(t *testing.T) {
	t.Helper()
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set INTEGRATION_TEST=1 and ensure Postgres/Redis are running")
	}
}

func setupTestApp(t *testing.T) *testApp {
	t.Helper()
	skipIfNoInfra(t)

	gin.SetMode(gin.TestMode)

	// Postgres
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=drone_admin password=secure_password dbname=drone_delivery sslmode=disable"
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("postgres connect: %v", err)
	}

	// Redis
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		db.Close()
		t.Fatalf("redis connect: %v", err)
	}

	// Create tables (drop first to ensure clean state)
	createTestSchema(t, db)

	// Infrastructure
	jwtService := jwtpkg.NewService("test-secret", 24*time.Hour)
	droneCache := redis.NewDroneLocationCache(rdb, 60)
	idempotencyStore := redis.NewIdempotencyStore(rdb, 300)
	rateLimiter := redis.NewRateLimiter(rdb, 1000, 60) // generous for tests
	mapboxClient := common.NewMapboxClient("https://api.mapbox.com", "")

	// Repositories
	orderRepo := order.NewRepository()
	droneRepo := drone.NewRepository()
	jobRepo := job.NewRepository()
	deliveryRepo := delivery.NewRepository(orderRepo, jobRepo, droneRepo)

	// Services
	orderService := order.NewOrderService(orderRepo, db, order.ZoneConfig{
		CenterLat: zoneCenter,
		CenterLng: zoneCenterL,
		RadiusKM:  zoneRadius,
	}, mapboxClient)

	center := common.NewLocation(zoneCenter, zoneCenterL)
	droneService := drone.NewDroneService(droneRepo, db, droneCache, center, zoneRadius)
	jobService := job.NewService(jobRepo, db)
	deliveryService := delivery.NewService(db, deliveryRepo)
	adminService := admin.NewService(orderService, droneService, deliveryService)
	authService := auth.NewAuthService(jwtService)

	// Handlers
	authHandler := auth.NewHandler(authService)
	orderHandler := order.NewHandler(orderService, deliveryService, droneService)
	droneHandler := drone.NewHandler(droneService, &orderQueryAdapter{svc: orderService}, deliveryService)
	jobHandler := job.NewHandler(jobService, deliveryService)
	adminHandler := admin.NewHandler(adminService, orderService, droneService)

	// Router
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.RateLimit(rateLimiter))
	r.Use(middleware.Auth(jwtService))

	// Auth
	authGroup := r.Group("/auth")
	authGroup.POST("/token", authHandler.GenerateToken)

	// Enduser
	enduserGroup := r.Group("")
	enduserGroup.Use(middleware.RoleGuard("enduser"))
	enduserGroup.GET("/orders", orderHandler.ListMyOrders)
	enduserGroup.GET("/orders/:id", orderHandler.GetOrderDetails)
	enduserMutations := enduserGroup.Group("")
	enduserMutations.Use(middleware.Bulkhead(50))
	enduserMutations.Use(middleware.Idempotency(idempotencyStore))
	enduserMutations.POST("/orders", orderHandler.PlaceOrder)
	enduserMutations.DELETE("/orders/:id", orderHandler.WithdrawOrder)

	// Drone
	droneGroup := r.Group("/drone")
	droneGroup.Use(middleware.RoleGuard("drone"))
	heartbeat := droneGroup.Group("")
	heartbeat.Use(middleware.Bulkhead(100))
	heartbeat.POST("/me/heartbeat", droneHandler.Heartbeat)
	droneGroup.GET("/jobs", jobHandler.ListOpenJobs)
	droneGroup.GET("/me/order", droneHandler.GetCurrentOrder)
	mutations := droneGroup.Group("")
	mutations.Use(middleware.Bulkhead(50))
	mutations.Use(middleware.Idempotency(idempotencyStore))
	mutations.POST("/jobs/reserve", jobHandler.ReserveJob)
	mutations.POST("/orders/:id/grab", jobHandler.GrabOrder)
	mutations.PATCH("/orders/:id/complete", jobHandler.CompleteDelivery)
	mutations.POST("/me/broken", droneHandler.ReportBroken)

	// Admin
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.RoleGuard("admin"))
	adminGroup.Use(middleware.Bulkhead(20))
	adminGroup.GET("/orders", adminHandler.ListOrders)
	adminGroup.PATCH("/orders/:id", adminHandler.UpdateOrder)
	adminGroup.GET("/drones", adminHandler.ListDrones)
	adminGroup.PATCH("/drones/:id/status", adminHandler.UpdateDroneStatus)

	app := &testApp{DB: db, Redis: rdb, Router: r, JWT: jwtService}

	t.Cleanup(func() {
		cleanTestData(t, db)
		rdb.FlushDB(context.Background())
		db.Close()
		rdb.Close()
	})

	return app
}

func createTestSchema(t *testing.T, db *sqlx.DB) {
	t.Helper()

	// Drop existing tables (in dependency order)
	db.MustExec(`DROP TABLE IF EXISTS jobs CASCADE`)
	db.MustExec(`DROP TABLE IF EXISTS drones CASCADE`)
	db.MustExec(`DROP TABLE IF EXISTS orders CASCADE`)

	// Create tables matching the code's actual columns
	db.MustExec(`CREATE TABLE orders (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		submitted_by VARCHAR(255) NOT NULL,
		origin_lat DOUBLE PRECISION NOT NULL,
		origin_lng DOUBLE PRECISION NOT NULL,
		dest_lat DOUBLE PRECISION NOT NULL,
		dest_lng DOUBLE PRECISION NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
		assigned_drone_id VARCHAR(255),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	db.MustExec(`CREATE TABLE drones (
		id VARCHAR(255) PRIMARY KEY,
		status VARCHAR(50) NOT NULL DEFAULT 'IDLE',
		latitude DOUBLE PRECISION DEFAULT 0,
		longitude DOUBLE PRECISION DEFAULT 0,
		current_order_id UUID REFERENCES orders(id),
		last_heartbeat TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	db.MustExec(`CREATE TABLE jobs (
		id VARCHAR(255) PRIMARY KEY,
		order_id UUID NOT NULL REFERENCES orders(id),
		status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
		reserved_by_drone_id VARCHAR(255) REFERENCES drones(id),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
}

func cleanTestData(t *testing.T, db *sqlx.DB) {
	t.Helper()
	db.Exec(`DELETE FROM jobs`)
	db.Exec(`DELETE FROM drones`)
	db.Exec(`DELETE FROM orders`)
}

// --- Token helpers ---

func enduserToken(t *testing.T, app *testApp, name string) string {
	t.Helper()
	token, err := app.JWT.GenerateToken(name, "enduser")
	if err != nil {
		t.Fatalf("failed to generate enduser token: %v", err)
	}
	return token
}

func droneToken(t *testing.T, app *testApp, droneID string) string {
	t.Helper()
	token, err := app.JWT.GenerateToken(droneID, "drone")
	if err != nil {
		t.Fatalf("failed to generate drone token: %v", err)
	}
	return token
}

func adminToken(t *testing.T, app *testApp) string {
	t.Helper()
	token, err := app.JWT.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}
	return token
}

// --- HTTP request helpers ---

func doRequest(app *testApp, method, path string, body any, token string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Idempotency-Key", fmt.Sprintf("idem-%d", time.Now().UnixNano()))

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

func doFormRequest(app *testApp, method, path string, formData map[string]string) *httptest.ResponseRecorder {
	form := ""
	for k, v := range formData {
		if form != "" {
			form += "&"
		}
		form += k + "=" + v
	}
	req := httptest.NewRequest(method, path, bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON response: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

// --- Location helpers (within Riyadh delivery zone) ---

func validOrigin() map[string]float64 {
	return map[string]float64{"lat": 24.72, "lng": 46.68}
}

func validDestination() map[string]float64 {
	return map[string]float64{"lat": 24.73, "lng": 46.69}
}

func placeTestOrder(t *testing.T, app *testApp, token string) (orderID string, jobID string) {
	t.Helper()

	body := map[string]any{
		"origin":      validOrigin(),
		"destination": validDestination(),
	}
	w := doRequest(app, http.MethodPost, "/orders", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("place order: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	orderData := resp["order"].(map[string]any)
	orderID = orderData["id"].(string)

	// Get job ID from jobs list
	drToken := droneToken(t, app, "helper-drone")
	jw := doRequest(app, http.MethodGet, "/drone/jobs", nil, drToken)
	if jw.Code != http.StatusOK {
		t.Fatalf("list jobs: expected 200, got %d: %s", jw.Code, jw.Body.String())
	}
	jobResp := parseJSON(t, jw)
	jobs := jobResp["jobs"].([]any)
	for _, j := range jobs {
		jm := j.(map[string]any)
		if jm["order_id"] == orderID {
			jobID = jm["id"].(string)
			break
		}
	}
	if jobID == "" {
		t.Fatal("could not find job for the placed order")
	}

	return orderID, jobID
}
