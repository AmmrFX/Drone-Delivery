package unit

import (
	"testing"

	"drone-delivery/internal/common"
	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/order"

	"github.com/google/uuid"
)

func newPendingOrder() *order.Order {
	return order.NewOrder("user-1", common.NewLocation(24.7, 46.7), common.NewLocation(24.8, 46.8))
}

func TestNewOrder_DefaultsPending(t *testing.T) {
	o := newPendingOrder()

	if o.Status != order.StatusPending {
		t.Fatalf("expected PENDING, got %s", o.Status)
	}
	if o.SubmittedBy != "user-1" {
		t.Fatalf("expected user-1, got %s", o.SubmittedBy)
	}
	if o.ID == uuid.Nil {
		t.Fatal("expected non-nil UUID")
	}
	if o.AssignedDroneID != nil {
		t.Fatal("expected no assigned drone")
	}
}

func TestNewOrder_SetsCoordinates(t *testing.T) {
	o := newPendingOrder()

	if o.OriginLat != 24.7 || o.OriginLng != 46.7 {
		t.Fatalf("origin mismatch: got (%f, %f)", o.OriginLat, o.OriginLng)
	}
	if o.DestLat != 24.8 || o.DestLng != 46.8 {
		t.Fatalf("destination mismatch: got (%f, %f)", o.DestLat, o.DestLng)
	}
}

func TestOrder_Origin_Destination_Helpers(t *testing.T) {
	o := newPendingOrder()

	orig := o.Origin()
	if orig.Lat != 24.7 || orig.Lng != 46.7 {
		t.Fatalf("Origin() mismatch")
	}
	dest := o.Destination()
	if dest.Lat != 24.8 || dest.Lng != 46.8 {
		t.Fatalf("Destination() mismatch")
	}
}

// --- Withdraw ---

func TestOrder_Withdraw_FromPending(t *testing.T) {
	o := newPendingOrder()
	if err := o.Withdraw(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusWithdrawn {
		t.Fatalf("expected WITHDRAWN, got %s", o.Status)
	}
}

func TestOrder_Withdraw_FromNonPending_Fails(t *testing.T) {
	statuses := []struct {
		name  string
		setup func() *order.Order
	}{
		{"ASSIGNED", func() *order.Order {
			o := newPendingOrder()
			_ = o.Assign("drone-1")
			return o
		}},
		{"PICKED_UP", func() *order.Order {
			o := newPendingOrder()
			_ = o.Assign("drone-1")
			_ = o.MarkPickedUp()
			return o
		}},
		{"DELIVERED", func() *order.Order {
			o := newPendingOrder()
			_ = o.Assign("drone-1")
			_ = o.MarkPickedUp()
			_ = o.MarkDelivered()
			return o
		}},
	}

	for _, tc := range statuses {
		t.Run(tc.name, func(t *testing.T) {
			o := tc.setup()
			err := o.Withdraw()
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
		})
	}
}

// --- Assign ---

func TestOrder_Assign_FromPending(t *testing.T) {
	o := newPendingOrder()
	if err := o.Assign("drone-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusAssigned {
		t.Fatalf("expected ASSIGNED, got %s", o.Status)
	}
	if o.AssignedDroneID == nil || *o.AssignedDroneID != "drone-1" {
		t.Fatal("drone ID not set correctly")
	}
}

func TestOrder_Assign_FromAwaitingHandoff(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	_ = o.AwaitHandoff()

	if err := o.Assign("drone-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusAssigned {
		t.Fatalf("expected ASSIGNED, got %s", o.Status)
	}
	if *o.AssignedDroneID != "drone-2" {
		t.Fatal("expected drone-2")
	}
}

func TestOrder_Assign_FromDelivered_Fails(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	_ = o.MarkPickedUp()
	_ = o.MarkDelivered()

	err := o.Assign("drone-2")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- MarkPickedUp ---

func TestOrder_MarkPickedUp_FromAssigned(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	if err := o.MarkPickedUp(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusPickedUp {
		t.Fatalf("expected PICKED_UP, got %s", o.Status)
	}
}

func TestOrder_MarkPickedUp_FromPending_Fails(t *testing.T) {
	o := newPendingOrder()
	if err := o.MarkPickedUp(); err == nil {
		t.Fatal("expected error")
	}
}

// --- MarkDelivered ---

func TestOrder_MarkDelivered_FromPickedUp(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	_ = o.MarkPickedUp()
	if err := o.MarkDelivered(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusDelivered {
		t.Fatalf("expected DELIVERED, got %s", o.Status)
	}
	if o.AssignedDroneID != nil {
		t.Fatal("expected drone to be unassigned after delivery")
	}
}

func TestOrder_MarkDelivered_FromAssigned_Fails(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	if err := o.MarkDelivered(); err == nil {
		t.Fatal("expected error")
	}
}

// --- MarkFailed ---

func TestOrder_MarkFailed_FromPickedUp(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	_ = o.MarkPickedUp()
	if err := o.MarkFailed(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusFailed {
		t.Fatalf("expected FAILED, got %s", o.Status)
	}
	if o.AssignedDroneID != nil {
		t.Fatal("expected drone to be unassigned after failure")
	}
}

func TestOrder_MarkFailed_FromPending_Fails(t *testing.T) {
	o := newPendingOrder()
	if err := o.MarkFailed(); err == nil {
		t.Fatal("expected error")
	}
}

// --- AwaitHandoff ---

func TestOrder_AwaitHandoff_FromAssigned(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	if err := o.AwaitHandoff(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusAwaitingHandoff {
		t.Fatalf("expected AWAITING_HANDOFF, got %s", o.Status)
	}
	if o.AssignedDroneID != nil {
		t.Fatal("expected drone to be unassigned")
	}
}

func TestOrder_AwaitHandoff_FromPickedUp(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	_ = o.MarkPickedUp()
	if err := o.AwaitHandoff(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.Status != order.StatusAwaitingHandoff {
		t.Fatalf("expected AWAITING_HANDOFF, got %s", o.Status)
	}
}

func TestOrder_AwaitHandoff_FromPending_Fails(t *testing.T) {
	o := newPendingOrder()
	if err := o.AwaitHandoff(); err == nil {
		t.Fatal("expected error")
	}
}

// --- UpdateOrigin / UpdateDestination ---

func TestOrder_UpdateOrigin_NonTerminal(t *testing.T) {
	o := newPendingOrder()
	loc := common.NewLocation(25.0, 47.0)
	if err := o.UpdateOrigin(loc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.OriginLat != 25.0 || o.OriginLng != 47.0 {
		t.Fatal("origin not updated")
	}
}

func TestOrder_UpdateOrigin_Terminal_Fails(t *testing.T) {
	o := newPendingOrder()
	_ = o.Withdraw()

	if err := o.UpdateOrigin(common.NewLocation(25.0, 47.0)); err == nil {
		t.Fatal("expected error for terminal state")
	}
}

func TestOrder_UpdateDestination_NonTerminal(t *testing.T) {
	o := newPendingOrder()
	loc := common.NewLocation(25.0, 47.0)
	if err := o.UpdateDestination(loc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.DestLat != 25.0 || o.DestLng != 47.0 {
		t.Fatal("destination not updated")
	}
}

func TestOrder_UpdateDestination_Terminal_Fails(t *testing.T) {
	o := newPendingOrder()
	_ = o.Assign("drone-1")
	_ = o.MarkPickedUp()
	_ = o.MarkDelivered()

	if err := o.UpdateDestination(common.NewLocation(25.0, 47.0)); err == nil {
		t.Fatal("expected error for terminal state")
	}
}

// --- IsTerminal ---

func TestStatus_IsTerminal(t *testing.T) {
	terminals := []order.Status{order.StatusDelivered, order.StatusFailed, order.StatusWithdrawn}
	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}

	nonTerminals := []order.Status{order.StatusPending, order.StatusAssigned, order.StatusPickedUp, order.StatusAwaitingHandoff}
	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("expected %s to NOT be terminal", s)
		}
	}
}

// --- Full lifecycle ---

func TestOrder_FullDeliveryLifecycle(t *testing.T) {
	o := newPendingOrder()

	if err := o.Assign("drone-1"); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if err := o.MarkPickedUp(); err != nil {
		t.Fatalf("MarkPickedUp: %v", err)
	}
	if err := o.MarkDelivered(); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}
	if o.Status != order.StatusDelivered {
		t.Fatalf("expected DELIVERED, got %s", o.Status)
	}
}

func TestOrder_HandoffReassignLifecycle(t *testing.T) {
	o := newPendingOrder()

	_ = o.Assign("drone-1")
	_ = o.AwaitHandoff()
	_ = o.Assign("drone-2")
	_ = o.MarkPickedUp()
	_ = o.MarkDelivered()

	if o.Status != order.StatusDelivered {
		t.Fatalf("expected DELIVERED, got %s", o.Status)
	}
}
