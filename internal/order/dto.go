package order

import (
	"drone-delivery/internal/common"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending         Status = "PENDING"
	StatusAssigned        Status = "ASSIGNED"
	StatusPickedUp        Status = "PICKED_UP"
	StatusDelivered       Status = "DELIVERED"
	StatusFailed          Status = "FAILED"
	StatusWithdrawn       Status = "WITHDRAWN"
	StatusAwaitingHandoff Status = "AWAITING_HANDOFF"
)

type Order struct {
	ID              uuid.UUID `db:"id" json:"id"`
	SubmittedBy     string    `db:"submitted_by" json:"submitted_by"`
	OriginLat       float64   `db:"origin_lat" json:"origin_lat"`
	OriginLng       float64   `db:"origin_lng" json:"origin_lng"`
	DestLat         float64   `db:"dest_lat" json:"dest_lat"`
	DestLng         float64   `db:"dest_lng" json:"dest_lng"`
	Status          Status    `db:"status" json:"status"`
	AssignedDroneID *string   `db:"assigned_drone_id" json:"assigned_drone_id,omitempty"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}
type PlaceOrderRequest struct {
	Origin      common.Location `json:"origin" binding:"required"`
	Destination common.Location `json:"destination" binding:"required"`
}

type OrderResponse struct {
	Order *Order `json:"order"`
}

type OrderDetailResponse struct {
	Order         *Order           `json:"order"`
	DroneLocation *common.Location `json:"drone_location,omitempty"`
	ETAMinutes    *float64         `json:"eta_minutes,omitempty"`
}
