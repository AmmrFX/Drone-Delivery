package order

import (
	"time"

	"github.com/google/uuid"

	"drone-delivery/internal/common"
	domainerrors "drone-delivery/internal/errors"
)


func (s Status) IsTerminal() bool {
	return s == StatusDelivered || s == StatusFailed || s == StatusWithdrawn
}


func NewOrder(submittedBy string, origin, destination common.Location) *Order {
	now := time.Now()
	return &Order{
		ID:          uuid.New(),
		SubmittedBy: submittedBy,
		OriginLat:   origin.Lat,
		OriginLng:   origin.Lng,
		DestLat:     destination.Lat,
		DestLng:     destination.Lng,
		Status:      StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (o *Order) Origin() common.Location {
	return common.NewLocation(o.OriginLat, o.OriginLng)
}

func (o *Order) Destination() common.Location {
	return common.NewLocation(o.DestLat, o.DestLng)
}

func (o *Order) Withdraw() error {
	if o.Status != StatusPending {
		return domainerrors.OrderInvalidTransition(string(o.Status), string(StatusWithdrawn))
	}
	o.Status = StatusWithdrawn
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) Assign(droneID string) error {
	if o.Status != StatusPending && o.Status != StatusAwaitingHandoff {
		return domainerrors.OrderInvalidTransition(string(o.Status), string(StatusAssigned))
	}
	o.Status = StatusAssigned
	o.AssignedDroneID = &droneID
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) MarkPickedUp() error {
	if o.Status != StatusAssigned {
		return domainerrors.OrderInvalidTransition(string(o.Status), string(StatusPickedUp))
	}
	o.Status = StatusPickedUp
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) MarkDelivered() error {
	if o.Status != StatusPickedUp {
		return domainerrors.OrderInvalidTransition(string(o.Status), string(StatusDelivered))
	}
	o.Status = StatusDelivered
	o.AssignedDroneID = nil
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) MarkFailed() error {
	if o.Status != StatusPickedUp {
		return domainerrors.OrderInvalidTransition(string(o.Status), string(StatusFailed))
	}
	o.Status = StatusFailed
	o.AssignedDroneID = nil
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) AwaitHandoff() error {
	if o.Status != StatusAssigned && o.Status != StatusPickedUp {
		return domainerrors.OrderInvalidTransition(string(o.Status), string(StatusAwaitingHandoff))
	}
	o.Status = StatusAwaitingHandoff
	o.AssignedDroneID = nil
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) UpdateOrigin(loc common.Location) error {
	if o.Status.IsTerminal() {
		return domainerrors.OrderInvalidTransition(string(o.Status), "update_origin")
	}
	o.OriginLat = loc.Lat
	o.OriginLng = loc.Lng
	o.UpdatedAt = time.Now()
	return nil
}

func (o *Order) UpdateDestination(loc common.Location) error {
	if o.Status.IsTerminal() {
		return domainerrors.OrderInvalidTransition(string(o.Status), "update_destination")
	}
	o.DestLat = loc.Lat
	o.DestLng = loc.Lng
	o.UpdatedAt = time.Now()
	return nil
}
