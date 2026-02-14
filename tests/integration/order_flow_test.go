package integration

import (
	"fmt"
	"net/http"
	"testing"
)

func TestOrderFlow_PlaceOrder(t *testing.T) {
	app := setupTestApp(t)
	token := enduserToken(t, app, "user-1")

	body := map[string]any{
		"origin":      validOrigin(),
		"destination": validDestination(),
	}
	w := doRequest(app, http.MethodPost, "/orders", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	order := resp["order"].(map[string]any)

	if order["id"] == nil || order["id"] == "" {
		t.Fatal("expected order ID")
	}
	if order["status"] != "PENDING" {
		t.Fatalf("expected PENDING, got %s", order["status"])
	}
	if order["submitted_by"] != "user-1" {
		t.Fatalf("expected user-1, got %s", order["submitted_by"])
	}
}

func TestOrderFlow_PlaceOrder_OutOfZone(t *testing.T) {
	app := setupTestApp(t)
	token := enduserToken(t, app, "user-1")

	body := map[string]any{
		"origin":      map[string]float64{"lat": 0, "lng": 0}, // far from Riyadh
		"destination": validDestination(),
	}
	w := doRequest(app, http.MethodPost, "/orders", body, token)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-zone origin, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrderFlow_ListMyOrders(t *testing.T) {
	app := setupTestApp(t)
	token := enduserToken(t, app, "user-1")

	// Place an order first
	body := map[string]any{
		"origin":      validOrigin(),
		"destination": validDestination(),
	}
	doRequest(app, http.MethodPost, "/orders", body, token)

	// List orders
	w := doRequest(app, http.MethodGet, "/orders", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	orders := resp["orders"].([]any)
	if len(orders) == 0 {
		t.Fatal("expected at least one order")
	}
}

func TestOrderFlow_ListMyOrders_IsolatedByUser(t *testing.T) {
	app := setupTestApp(t)
	tokenA := enduserToken(t, app, "user-a")
	tokenB := enduserToken(t, app, "user-b")

	// User A places an order
	body := map[string]any{
		"origin":      validOrigin(),
		"destination": validDestination(),
	}
	doRequest(app, http.MethodPost, "/orders", body, tokenA)

	// User B should see no orders
	w := doRequest(app, http.MethodGet, "/orders", nil, tokenB)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	orders := resp["orders"].([]any)
	if len(orders) != 0 {
		t.Fatalf("expected 0 orders for user-b, got %d", len(orders))
	}
}

func TestOrderFlow_GetOrderDetails(t *testing.T) {
	app := setupTestApp(t)
	token := enduserToken(t, app, "user-1")

	orderID, _ := placeTestOrder(t, app, token)

	w := doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	order := resp["order"].(map[string]any)
	if order["id"] != orderID {
		t.Fatalf("expected order ID %s, got %s", orderID, order["id"])
	}
}

func TestOrderFlow_GetOrderDetails_WrongUser(t *testing.T) {
	app := setupTestApp(t)
	tokenA := enduserToken(t, app, "user-a")
	tokenB := enduserToken(t, app, "user-b")

	orderID, _ := placeTestOrder(t, app, tokenA)

	// User B should not see user A's order
	w := doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, tokenB)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrderFlow_FullDeliveryLifecycle(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	drToken := droneToken(t, app, "drone-1")

	// 1. Send heartbeat (registers drone in zone)
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	w := doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("heartbeat: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Place order
	orderID, jobID := placeTestOrder(t, app, userToken)

	// 3. Drone lists open jobs
	w = doRequest(app, http.MethodGet, "/drone/jobs", nil, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("list jobs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	jobResp := parseJSON(t, w)
	jobs := jobResp["jobs"].([]any)
	if len(jobs) == 0 {
		t.Fatal("expected at least one open job")
	}

	// 4. Drone reserves the job
	reserveBody := map[string]string{"job_id": jobID}
	w = doRequest(app, http.MethodPost, "/drone/jobs/reserve", reserveBody, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("reserve job: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Verify order is now ASSIGNED
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	if w.Code != http.StatusOK {
		t.Fatalf("get order: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	orderResp := parseJSON(t, w)
	orderData := orderResp["order"].(map[string]any)
	if orderData["status"] != "ASSIGNED" {
		t.Fatalf("expected ASSIGNED, got %s", orderData["status"])
	}

	// 6. Drone grabs (picks up) the order
	w = doRequest(app, http.MethodPost, fmt.Sprintf("/drone/orders/%s/grab", orderID), nil, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("grab order: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 7. Verify order is PICKED_UP
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	orderResp = parseJSON(t, w)
	orderData = orderResp["order"].(map[string]any)
	if orderData["status"] != "PICKED_UP" {
		t.Fatalf("expected PICKED_UP, got %s", orderData["status"])
	}

	// 8. Drone completes the delivery
	completeBody := map[string]string{"status": "delivered"}
	w = doRequest(app, http.MethodPatch, fmt.Sprintf("/drone/orders/%s/complete", orderID), completeBody, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("complete delivery: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 9. Verify order is DELIVERED
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	orderResp = parseJSON(t, w)
	orderData = orderResp["order"].(map[string]any)
	if orderData["status"] != "DELIVERED" {
		t.Fatalf("expected DELIVERED, got %s", orderData["status"])
	}
}

func TestOrderFlow_FailedDelivery(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	drToken := droneToken(t, app, "drone-1")

	// Setup drone
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)

	// Place and assign order
	orderID, jobID := placeTestOrder(t, app, userToken)
	doRequest(app, http.MethodPost, "/drone/jobs/reserve", map[string]string{"job_id": jobID}, drToken)
	doRequest(app, http.MethodPost, fmt.Sprintf("/drone/orders/%s/grab", orderID), nil, drToken)

	// Mark as failed
	w := doRequest(app, http.MethodPatch, fmt.Sprintf("/drone/orders/%s/complete", orderID), map[string]string{"status": "failed"}, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("fail delivery: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify order is FAILED
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	resp := parseJSON(t, w)
	order := resp["order"].(map[string]any)
	if order["status"] != "FAILED" {
		t.Fatalf("expected FAILED, got %s", order["status"])
	}
}

func TestOrderFlow_DroneHeartbeat(t *testing.T) {
	app := setupTestApp(t)
	drToken := droneToken(t, app, "drone-1")

	body := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	w := doRequest(app, http.MethodPost, "/drone/me/heartbeat", body, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	if resp["drone_status"] != "IDLE" {
		t.Fatalf("expected IDLE, got %s", resp["drone_status"])
	}
}

func TestOrderFlow_DroneHeartbeat_OutOfZone(t *testing.T) {
	app := setupTestApp(t)
	drToken := droneToken(t, app, "drone-1")

	body := map[string]float64{"latitude": 0, "longitude": 0}
	w := doRequest(app, http.MethodPost, "/drone/me/heartbeat", body, drToken)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-zone heartbeat, got %d: %s", w.Code, w.Body.String())
	}
}
