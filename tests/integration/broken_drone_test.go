package integration

import (
	"fmt"
	"net/http"
	"testing"
)

func TestBrokenDrone_IdleDrone(t *testing.T) {
	app := setupTestApp(t)
	drToken := droneToken(t, app, "drone-1")

	// Register drone via heartbeat
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)

	// Report broken
	w := doRequest(app, http.MethodPost, "/drone/me/broken", nil, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseJSON(t, w)
	if resp["message"] != "broken report received" {
		t.Fatalf("unexpected message: %v", resp["message"])
	}
}

func TestBrokenDrone_WithActiveOrder_HandoffCreated(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	drToken := droneToken(t, app, "drone-1")
	dr2Token := droneToken(t, app, "drone-2")

	// Setup drone
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)

	// Place order and have drone-1 reserve it
	orderID, jobID := placeTestOrder(t, app, userToken)
	w := doRequest(app, http.MethodPost, "/drone/jobs/reserve", map[string]string{"job_id": jobID}, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("reserve: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify order is ASSIGNED
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	resp := parseJSON(t, w)
	order := resp["order"].(map[string]any)
	if order["status"] != "ASSIGNED" {
		t.Fatalf("expected ASSIGNED, got %s", order["status"])
	}

	// Drone-1 reports broken
	w = doRequest(app, http.MethodPost, "/drone/me/broken", nil, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("broken report: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify order is now AWAITING_HANDOFF
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	resp = parseJSON(t, w)
	order = resp["order"].(map[string]any)
	if order["status"] != "AWAITING_HANDOFF" {
		t.Fatalf("expected AWAITING_HANDOFF, got %s", order["status"])
	}

	// Verify a new OPEN job was created for handoff
	// Setup drone-2
	heartbeat2 := map[string]float64{"latitude": 24.73, "longitude": 46.69}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat2, dr2Token)

	w = doRequest(app, http.MethodGet, "/drone/jobs", nil, dr2Token)
	if w.Code != http.StatusOK {
		t.Fatalf("list jobs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	jobResp := parseJSON(t, w)
	jobs := jobResp["jobs"].([]any)

	var handoffJobID string
	for _, j := range jobs {
		jm := j.(map[string]any)
		if jm["order_id"] == orderID && jm["status"] == "OPEN" {
			handoffJobID = jm["id"].(string)
			break
		}
	}
	if handoffJobID == "" {
		t.Fatal("expected a new OPEN job for handoff")
	}

	// Drone-2 picks up the handoff
	w = doRequest(app, http.MethodPost, "/drone/jobs/reserve", map[string]string{"job_id": handoffJobID}, dr2Token)
	if w.Code != http.StatusOK {
		t.Fatalf("reserve handoff: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify order is now ASSIGNED to drone-2
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	resp = parseJSON(t, w)
	order = resp["order"].(map[string]any)
	if order["status"] != "ASSIGNED" {
		t.Fatalf("expected ASSIGNED after handoff, got %s", order["status"])
	}
	if order["assigned_drone_id"] != "drone-2" {
		t.Fatalf("expected assigned to drone-2, got %v", order["assigned_drone_id"])
	}
}

func TestBrokenDrone_AlreadyBroken(t *testing.T) {
	app := setupTestApp(t)
	drToken := droneToken(t, app, "drone-1")

	// Register and break
	heartbeat := map[string]float64{"latitude": 24.72, "longitude": 46.68}
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", heartbeat, drToken)
	w := doRequest(app, http.MethodPost, "/drone/me/broken", nil, drToken)
	if w.Code != http.StatusOK {
		t.Fatalf("first broken: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Try to break again — should fail with conflict
	w = doRequest(app, http.MethodPost, "/drone/me/broken", nil, drToken)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for already broken, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBrokenDrone_FullHandoffDelivery(t *testing.T) {
	app := setupTestApp(t)
	userToken := enduserToken(t, app, "user-1")
	dr1Token := droneToken(t, app, "drone-1")
	dr2Token := droneToken(t, app, "drone-2")

	// Setup drones
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", map[string]float64{"latitude": 24.72, "longitude": 46.68}, dr1Token)
	doRequest(app, http.MethodPost, "/drone/me/heartbeat", map[string]float64{"latitude": 24.73, "longitude": 46.69}, dr2Token)

	// Place order, drone-1 reserves and grabs
	orderID, jobID := placeTestOrder(t, app, userToken)
	doRequest(app, http.MethodPost, "/drone/jobs/reserve", map[string]string{"job_id": jobID}, dr1Token)
	doRequest(app, http.MethodPost, fmt.Sprintf("/drone/orders/%s/grab", orderID), nil, dr1Token)

	// Drone-1 breaks while delivering
	w := doRequest(app, http.MethodPost, "/drone/me/broken", nil, dr1Token)
	if w.Code != http.StatusOK {
		t.Fatalf("broken: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Find handoff job
	w = doRequest(app, http.MethodGet, "/drone/jobs", nil, dr2Token)
	jobResp := parseJSON(t, w)
	jobs := jobResp["jobs"].([]any)
	var handoffJobID string
	for _, j := range jobs {
		jm := j.(map[string]any)
		if jm["order_id"] == orderID {
			handoffJobID = jm["id"].(string)
			break
		}
	}
	if handoffJobID == "" {
		t.Fatal("expected handoff job")
	}

	// Drone-2 picks up handoff, grabs, and delivers
	doRequest(app, http.MethodPost, "/drone/jobs/reserve", map[string]string{"job_id": handoffJobID}, dr2Token)
	doRequest(app, http.MethodPost, fmt.Sprintf("/drone/orders/%s/grab", orderID), nil, dr2Token)
	w = doRequest(app, http.MethodPatch, fmt.Sprintf("/drone/orders/%s/complete", orderID), map[string]string{"status": "delivered"}, dr2Token)
	if w.Code != http.StatusOK {
		t.Fatalf("complete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify final status
	w = doRequest(app, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), nil, userToken)
	resp := parseJSON(t, w)
	order := resp["order"].(map[string]any)
	if order["status"] != "DELIVERED" {
		t.Fatalf("expected DELIVERED, got %s", order["status"])
	}
}
