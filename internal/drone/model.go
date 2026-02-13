package drone

import (
	"time"

	"github.com/google/uuid"

	"drone-delivery/internal/common"
	domainerrors "drone-delivery/internal/errors"
)


func New(id string) *Drone {
	now := time.Now()
	return &Drone{
		ID:        id,
		Status:    StatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (d *Drone) Location() common.Location {
	return common.NewLocation(d.Latitude, d.Longitude)
}

func (d *Drone) Reserve(orderID uuid.UUID) error {
	if d.Status != StatusIdle {
		return domainerrors.DroneInvalidTransition(string(d.Status), string(StatusEnRoutePickup))
	}
	d.Status = StatusEnRoutePickup
	d.CurrentOrderID = &orderID
	d.UpdatedAt = time.Now()
	return nil
}

func (d *Drone) StartDelivery() error {
	if d.Status != StatusEnRoutePickup {
		return domainerrors.DroneInvalidTransition(string(d.Status), string(StatusEnRouteDelivery))
	}
	d.Status = StatusEnRouteDelivery
	d.UpdatedAt = time.Now()
	return nil
}

func (d *Drone) GoIdle() {
	d.Status = StatusIdle
	d.CurrentOrderID = nil
	d.UpdatedAt = time.Now()
}

func (d *Drone) MarkBroken() (*DroneBrokenEvent, error) {
	if d.Status == StatusBroken {
		return nil, domainerrors.DroneAlreadyBroken()
	}
	event := &DroneBrokenEvent{
		DroneID:  d.ID,
		Location: d.Location(),
		OrderID:  d.CurrentOrderID,
	}
	d.Status = StatusBroken
	d.CurrentOrderID = nil
	d.UpdatedAt = time.Now()
	return event, nil
}

func (d *Drone) MarkFixed() error {
	if d.Status != StatusBroken {
		return domainerrors.DroneInvalidTransition(string(d.Status), string(StatusIdle))
	}
	d.Status = StatusIdle
	d.CurrentOrderID = nil
	d.UpdatedAt = time.Now()
	return nil
}

func (d *Drone) UpdateLocation(lat, lng float64) {
	d.Latitude = lat
	d.Longitude = lng
	now := time.Now()
	d.LastHeartbeat = &now
	d.UpdatedAt = now
}
