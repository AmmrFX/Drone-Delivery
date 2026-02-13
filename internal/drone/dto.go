package drone

import (
	"drone-delivery/internal/common"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusIdle            Status = "IDLE"
	StatusEnRoutePickup   Status = "EN_ROUTE_PICKUP"
	StatusEnRouteDelivery Status = "EN_ROUTE_DELIVERY"
	StatusBroken          Status = "BROKEN"
)

type Drone struct {
	ID             string     `db:"id" json:"id"`
	Status         Status     `db:"status" json:"status"`
	Latitude       float64    `db:"latitude" json:"latitude"`
	Longitude      float64    `db:"longitude" json:"longitude"`
	CurrentOrderID *uuid.UUID `db:"current_order_id" json:"current_order_id,omitempty"`
	LastHeartbeat  *time.Time `db:"last_heartbeat" json:"last_heartbeat,omitempty"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

type DroneBrokenEvent struct {
	DroneID  string
	Location common.Location
	OrderID  *uuid.UUID
}
