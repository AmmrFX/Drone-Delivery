package admin

import (
	"context"

	"drone-delivery/internal/common"
	"drone-delivery/internal/drone"
	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/job"
	"drone-delivery/internal/order"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
	orderService order.Service
	droneService drone.Service
	jobService   job.Service
	db           *sqlx.DB
}

func NewService(orderService order.Service, droneService drone.Service, jobService job.Service, db *sqlx.DB) Service {
	return &service{orderService: orderService, droneService: droneService, jobService: jobService, db: db}
}

func (s *service) HandleDroneBroken(ctx context.Context, droneID string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	d, err := s.droneService.GetByIDWithTx(ctx, tx, droneID)
	if err != nil {
		return domainerrors.DroneNotFound(droneID)
	}

	event, err := d.MarkBroken()
	if err != nil {
		return err
	}
	if err := s.droneService.UpdateWithTx(ctx, tx, d); err != nil {
		return domainerrors.NewInternal("failed to update drone", err)
	}

	if event.OrderID != nil {
		if err := s.orderService.AwaitHandoffWithTx(ctx, tx, *event.OrderID); err != nil {
			return err
		}
		if err := s.jobService.CreateJobWithTx(ctx, tx, event.OrderID.String()); err != nil {
			return domainerrors.NewInternal("failed to create handoff job", err)
		}
	}

	return tx.Commit()
}

func (s *service) MarkDroneFixed(ctx context.Context, droneID string) error {
	d, err := s.droneService.GetByID(ctx, droneID)
	if err != nil {
		return err
	}
	if err := d.MarkFixed(); err != nil {
		return err
	}
	return s.droneService.UpdateStatus(ctx, droneID, d)
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
