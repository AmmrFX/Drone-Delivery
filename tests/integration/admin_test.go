package integration

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAdmin_ListOrders(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	aToken := adminToken(t, app)

	// Place some orders
	placeTestOrder(t, app, userToken)
	placeTestOrder(t, app, userToken)

	// Admin lists all orders
	w := doRequest(app, http.MethodGet, "/admin/orders", nil, aToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	orders := resp["orders"].([]any)
	if len(orders) < 2 {
		t.Fatalf("expected at least 2 orders, got %d", len(orders))
	}
	total := resp["total"].(float64)
	if total < 2 {
		t.Fatalf("expected total >= 2, got %v", total)
	}
}

func TestAdmin_ListOrders_FilterByStatus(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	aToken := adminToken(t, app)

	placeTestOrder(t, app, userToken)

	// Filter by PENDING
	w := doRequest(app, http.MethodGet, "/admin/orders?status=PENDING", nil, aToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	orders := resp["orders"].([]any)
	for _, o := range orders {
		om := o.(map[string]any)
		if om["status"] != "PENDING" {
			t.Fatalf("expected all PENDING, got %s", om["status"])
		}
	}
}

func TestAdmin_UpdateOrder(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	aToken := adminToken(t, app)

	orderID, _ := placeTestOrder(t, app, userToken)

	// Update origin
	newOrigin := map[string]any{
		"origin": map[string]float64{"lat": 24.74, "lng": 46.70},
	}
	w := doRequest(app, http.MethodPatch, fmt.Sprintf("/admin/orders/%s", orderID), newOrigin, aToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	order := resp["order"].(map[string]any)
	if order["origin_lat"].(float64) != 24.74 {
		t.Fatalf("expected updated origin_lat 24.74, got %v", order["origin_lat"])
	}
}

func TestAdmin_UpdateOrder_OutOfZone(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	aToken := adminToken(t, app)

	orderID, _ := placeTestOrder(t, app, userToken)

	// Try to update to out-of-zone location
	badOrigin := map[string]any{
		"origin": map[string]float64{"lat": 0, "lng": 0},
	}
	w := doRequest(app, http.MethodPatch, fmt.Sprintf("/admin/orders/%s", orderID), badOrigin, aToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-zone, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdmin_ListDrones(t *testing.T) {
	app := setupTestApp(t)
	drToken := droneToken(t, app, "drone-1")
	aToken := adminToken(t, app)

	// Register a drone via heartbeat
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)

	// Admin lists drones
	w := doRequest(app, http.MethodGet, "/admin/drones", nil, aToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	drones := resp["drones"].([]any)
	if len(drones) == 0 {
		t.Fatal("expected at least one drone")
	}

	found := false
	for _, d := range drones {
		dm := d.(map[string]any)
		if dm["id"] == "drone-1" {
			found = true
			if dm["status"] != "IDLE" {
				t.Fatalf("expected IDLE, got %s", dm["status"])
			}
		}
	}
	if !found {
		t.Fatal("drone-1 not found in list")
	}
}

func TestAdmin_UpdateDroneStatus_MarkBrokenAndFixed(t *testing.T) {
	app := setupTestApp(t)
	drToken := droneToken(t, app, "drone-1")
	aToken := adminToken(t, app)

	// Register drone
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)

	// Admin marks drone as broken
	w := doRequest(app, http.MethodPatch, "/admin/drones/drone-1/status", map[string]string{"status": "broken"}, aToken)
	if w.Code != http.StatusOK {
		t.Fatalf("mark broken: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Admin marks drone as fixed
	w = doRequest(app, http.MethodPatch, "/admin/drones/drone-1/status", map[string]string{"status": "fixed"}, aToken)
	if w.Code != http.StatusOK {
		t.Fatalf("mark fixed: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify drone is IDLE again
	w = doRequest(app, http.MethodGet, "/admin/drones", nil, aToken)
	resp := parseJSON(t, w)
	drones := resp["drones"].([]any)
	for _, d := range drones {
		dm := d.(map[string]any)
		if dm["id"] == "drone-1" {
			if dm["status"] != "IDLE" {
				t.Fatalf("expected IDLE after fix, got %s", dm["status"])
			}
		}
	}
}

func TestAdmin_UpdateDroneStatus_InvalidStatus(t *testing.T) {
	app := setupTestApp(t)
	drToken := droneToken(t, app, "drone-1")
	aToken := adminToken(t, app)

	// Register drone
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)

	// Invalid status
	w := doRequest(app, http.MethodPatch, "/admin/drones/drone-1/status", map[string]string{"status": "flying"}, aToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdmin_NonAdminCantAccess(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")

	w := doRequest(app, http.MethodGet, "/admin/orders", nil, userToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	drToken := droneToken(t, app, "drone-1")
	w = doRequest(app, http.MethodGet, "/admin/orders", nil, drToken)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
