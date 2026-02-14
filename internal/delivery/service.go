package delivery

import (
	"context"

	"drone-delivery/internal/job"
	"drone-delivery/internal/order"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreateOrderAndJob(ctx context.Context, o *order.Order) error
	CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error
	ReserveJobAndAssign(ctx context.Context, jobID, droneID string) (*job.Job, error)
	GrabOrder(ctx context.Context, orderID uuid.UUID, droneID string) error
	CompleteDelivery(ctx context.Context, orderID uuid.UUID, droneID string, delivered bool) error
}

type service struct {
	db   *sqlx.DB
	repo Repository
}

func NewService(db *sqlx.DB, repo Repository) Service {
	return &service{db: db, repo: repo}
}

func (s *service) CreateOrderAndJob(ctx context.Context, o *order.Order) error {
	return s.repo.CreateOrderAndJob(ctx, s.db, o)
}

func (s *service) CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error {
	return s.repo.CancelOrderAndJob(ctx, s.db, orderID, submittedBy)
}

func (s *service) ReserveJobAndAssign(ctx context.Context, jobID, droneID string) (*job.Job, error) {
	return s.repo.ReserveJobAndAssign(ctx, s.db, jobID, droneID)
}

func (s *service) GrabOrder(ctx context.Context, orderID uuid.UUID, droneID string) error {
	return s.repo.GrabOrder(ctx, s.db, orderID, droneID)
}

func (s *service) CompleteDelivery(ctx context.Context, orderID uuid.UUID, droneID string, delivered bool) error {
	return s.repo.CompleteDelivery(ctx, s.db, orderID, droneID, delivered)
}
