package main

import (
	"drone-delivery/internal/middleware"
)

func (a *AppContext) setupRoutes() {
	r := a.Router

	// ── Global Middleware (outermost → innermost) ──
	r.Use(middleware.Logger())                 // 1. Request logging
	r.Use(middleware.Recovery())               // 2. Panic recovery
	r.Use(middleware.RateLimit(a.RateLimiter)) // 3. Per-IP rate limiting
	r.Use(middleware.Auth(a.JWTService))       // 4. JWT auth (skips /auth/token)

	// ── Health (no auth, no rate limit) ──
	r.GET("/health", a.healthCheck)

	// ── Auth (no role guard, no idempotency) ──
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/token", a.AuthHandler.GenerateToken)
	}

	// ── Enduser Routes (role: enduser) ──
	enduserGroup := r.Group("")
	enduserGroup.Use(middleware.RoleGuard("enduser"))
	enduserGroup.Use(middleware.Bulkhead(a.Config.Bulkhead.MutationPool)) // 5. Bulkhead
	enduserGroup.Use(middleware.Idempotency(a.IdempotencyStore))          // 7. Idempotency
	{
		enduserGroup.POST("/orders", a.OrderHandler.PlaceOrder)
		enduserGroup.DELETE("/orders/:id", a.OrderHandler.WithdrawOrder)
		enduserGroup.GET("/orders/:id", a.OrderHandler.GetOrderDetails)
	}

	// ── Drone Routes (role: drone) ──
	droneGroup := r.Group("/drone")
	droneGroup.Use(middleware.RoleGuard("drone"))
	{
		// Heartbeat gets its own bulkhead pool (high concurrency)
		heartbeat := droneGroup.Group("")
		heartbeat.Use(middleware.Bulkhead(a.Config.Bulkhead.HeartbeatPool))
		{
			heartbeat.POST("/me/heartbeat", a.DroneHandler.Heartbeat)
		}

		// Read-only endpoints
		droneGroup.GET("/jobs", a.JobHandler.ListOpenJobs)
		droneGroup.GET("/me/order", a.DroneHandler.GetCurrentOrder)

		// Mutations get the mutation pool
		mutations := droneGroup.Group("")
		mutations.Use(middleware.Bulkhead(a.Config.Bulkhead.MutationPool))
		mutations.Use(middleware.Idempotency(a.IdempotencyStore))
		{
			mutations.POST("/jobs/reserve", a.JobHandler.ReserveJob)
			mutations.POST("/orders/:id/grab", a.JobHandler.GrabOrder)
			mutations.PATCH("/orders/:id/complete", a.JobHandler.CompleteDelivery)
			mutations.POST("/me/broken", a.DroneHandler.ReportBroken)
		}
	}

	// ── Admin Routes (role: admin) ──
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.RoleGuard("admin"))
	adminGroup.Use(middleware.Bulkhead(a.Config.Bulkhead.AdminPool))
	{
		adminGroup.GET("/orders", a.AdminHandler.ListOrders)
		adminGroup.PATCH("/orders/:id", a.AdminHandler.UpdateOrder)
		adminGroup.GET("/drones", a.AdminHandler.ListDrones)
		adminGroup.PATCH("/drones/:id/status", a.AdminHandler.UpdateDroneStatus)
	}
}
