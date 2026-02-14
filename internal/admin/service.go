package admin

import (
	"context"

	"drone-delivery/internal/common"
	"drone-delivery/internal/delivery"
	"drone-delivery/internal/drone"
	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/order"

	"github.com/google/uuid"
)

type Service interface {
	HandleDroneBroken(ctx context.Context, droneID string) error
	MarkDroneFixed(ctx context.Context, droneID string) error
	ListOrders(ctx context.Context, status *order.Status, page, limit int) ([]*order.Order, int, error)
	UpdateOrder(ctx context.Context, orderID uuid.UUID, origin, dest *common.Location) (*order.Order, error)
	ListDrones(ctx context.Context, status *drone.Status, page, limit int) ([]*drone.Drone, int, error)
	UpdateDroneStatus(ctx context.Context, droneID, status string) error
}

type service struct {
	orderService    order.Service
	droneService    drone.Service
	deliveryService delivery.Service
}

func NewService(orderService order.Service, droneService drone.Service, deliveryService delivery.Service) Service {
	return &service{orderService: orderService, droneService: droneService, deliveryService: deliveryService}
}

func (s *service) HandleDroneBroken(ctx context.Context, droneID string) error {
	return s.deliveryService.HandleDroneBroken(ctx, droneID)
}

func (s *service) MarkDroneFixed(ctx context.Context, droneID string) error {
	d, err := s.droneService.GetByID(ctx, droneID)
	if err != nil {
		return err
	}
	if err := d.MarkFixed(); err != nil {
		return err
	}
	return s.droneService.UpdateStatus(ctx, d)
}

func (s *service) ListOrders(ctx context.Context, status *order.Status, page, limit int) ([]*order.Order, int, error) {
	return s.orderService.ListAll(ctx, status, page, limit)
}

func (s *service) UpdateOrder(ctx context.Context, orderID uuid.UUID, origin, dest *common.Location) (*order.Order, error) {
	return s.orderService.AdminUpdateOrder(ctx, orderID, origin, dest)
}

func (s *service) ListDrones(ctx context.Context, status *drone.Status, page, limit int) ([]*drone.Drone, int, error) {
	return s.droneService.ListAll(ctx, status, page, limit)
}

func (s *service) UpdateDroneStatus(ctx context.Context, droneID, status string) error {
	switch status {
	case "broken":
		return s.HandleDroneBroken(ctx, droneID)
	case "fixed":
		return s.MarkDroneFixed(ctx, droneID)
	default:
		return domainerrors.NewValidation("status must be 'broken' or 'fixed'")
	}
}
