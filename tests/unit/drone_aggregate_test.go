package unit

import (
	"testing"

	"drone-delivery/internal/drone"
	domainerrors "drone-delivery/internal/errors"

	"github.com/google/uuid"
)

func newIdleDrone() *drone.Drone {
	return drone.New("drone-1")
}

func TestNewDrone_DefaultsIdle(t *testing.T) {
	d := newIdleDrone()

	if d.ID != "drone-1" {
		t.Fatalf("expected drone-1, got %s", d.ID)
	}
	if d.Status != drone.StatusIdle {
		t.Fatalf("expected IDLE, got %s", d.Status)
	}
	if d.CurrentOrderID != nil {
		t.Fatal("expected no current order")
	}
}

// --- Reserve ---

func TestDrone_Reserve_FromIdle(t *testing.T) {
	d := newIdleDrone()
	orderID := uuid.New()

	if err := d.Reserve(orderID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != drone.StatusEnRoutePickup {
		t.Fatalf("expected EN_ROUTE_PICKUP, got %s", d.Status)
	}
	if d.CurrentOrderID == nil || *d.CurrentOrderID != orderID {
		t.Fatal("order ID not set correctly")
	}
}

func TestDrone_Reserve_FromNonIdle_Fails(t *testing.T) {
	d := newIdleDrone()
	_ = d.Reserve(uuid.New())

	err := d.Reserve(uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domainerrors.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if de.Code != domainerrors.ErrInvalidTransition {
		t.Fatalf("expected INVALID_TRANSITION, got %s", de.Code)
	}
}

// --- StartDelivery ---

func TestDrone_StartDelivery_FromEnRoutePickup(t *testing.T) {
	d := newIdleDrone()
	_ = d.Reserve(uuid.New())

	if err := d.StartDelivery(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != drone.StatusEnRouteDelivery {
		t.Fatalf("expected EN_ROUTE_DELIVERY, got %s", d.Status)
	}
}

func TestDrone_StartDelivery_FromIdle_Fails(t *testing.T) {
	d := newIdleDrone()
	if err := d.StartDelivery(); err == nil {
		t.Fatal("expected error")
	}
}

// --- GoIdle ---

func TestDrone_GoIdle(t *testing.T) {
	d := newIdleDrone()
	_ = d.Reserve(uuid.New())

	d.GoIdle()

	if d.Status != drone.StatusIdle {
		t.Fatalf("expected IDLE, got %s", d.Status)
	}
	if d.CurrentOrderID != nil {
		t.Fatal("expected no current order after GoIdle")
	}
}

// --- MarkBroken ---

func TestDrone_MarkBroken_FromIdle(t *testing.T) {
	d := newIdleDrone()
	event, err := d.MarkBroken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != drone.StatusBroken {
		t.Fatalf("expected BROKEN, got %s", d.Status)
	}
	if event.DroneID != "drone-1" {
		t.Fatalf("expected drone-1 in event, got %s", event.DroneID)
	}
	if event.OrderID != nil {
		t.Fatal("expected nil OrderID in event for idle drone")
	}
}

func TestDrone_MarkBroken_WithOrder(t *testing.T) {
	d := newIdleDrone()
	orderID := uuid.New()
	_ = d.Reserve(orderID)

	event, err := d.MarkBroken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.OrderID == nil || *event.OrderID != orderID {
		t.Fatal("expected order ID in broken event")
	}
	if d.CurrentOrderID != nil {
		t.Fatal("expected current order cleared after broken")
	}
}

func TestDrone_MarkBroken_AlreadyBroken_Fails(t *testing.T) {
	d := newIdleDrone()
	_, _ = d.MarkBroken()

	_, err := d.MarkBroken()
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domainerrors.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if de.Code != domainerrors.ErrConflict {
		t.Fatalf("expected CONFLICT, got %s", de.Code)
	}
}

// --- MarkFixed ---

func TestDrone_MarkFixed_FromBroken(t *testing.T) {
	d := newIdleDrone()
	_, _ = d.MarkBroken()

	if err := d.MarkFixed(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Status != drone.StatusIdle {
		t.Fatalf("expected IDLE, got %s", d.Status)
	}
}

func TestDrone_MarkFixed_FromIdle_Fails(t *testing.T) {
	d := newIdleDrone()
	if err := d.MarkFixed(); err == nil {
		t.Fatal("expected error")
	}
}

// --- UpdateLocation ---

func TestDrone_UpdateLocation(t *testing.T) {
	d := newIdleDrone()
	d.UpdateLocation(24.75, 46.65)

	if d.Latitude != 24.75 || d.Longitude != 46.65 {
		t.Fatalf("location mismatch: got (%f, %f)", d.Latitude, d.Longitude)
	}
	if d.LastHeartbeat == nil {
		t.Fatal("expected LastHeartbeat to be set")
	}
}

// --- Location helper ---

func TestDrone_Location(t *testing.T) {
	d := newIdleDrone()
	d.UpdateLocation(24.75, 46.65)

	loc := d.Location()
	if loc.Lat != 24.75 || loc.Lng != 46.65 {
		t.Fatalf("Location() mismatch: got (%f, %f)", loc.Lat, loc.Lng)
	}
}

// --- Full lifecycle ---

func TestDrone_FullDeliveryLifecycle(t *testing.T) {
	d := newIdleDrone()
	orderID := uuid.New()

	if err := d.Reserve(orderID); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := d.StartDelivery(); err != nil {
		t.Fatalf("StartDelivery: %v", err)
	}
	d.GoIdle()

	if d.Status != drone.StatusIdle {
		t.Fatalf("expected IDLE, got %s", d.Status)
	}
	if d.CurrentOrderID != nil {
		t.Fatal("expected no current order")
	}
}

func TestDrone_BrokenAndFixedCycle(t *testing.T) {
	d := newIdleDrone()
	orderID := uuid.New()
	_ = d.Reserve(orderID)

	_, err := d.MarkBroken()
	if err != nil {
		t.Fatalf("MarkBroken: %v", err)
	}

	if err := d.MarkFixed(); err != nil {
		t.Fatalf("MarkFixed: %v", err)
	}

	// Should be able to reserve again
	if err := d.Reserve(uuid.New()); err != nil {
		t.Fatalf("Reserve after fix: %v", err)
	}
}
