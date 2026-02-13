package job

import (
	"context"

	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/order"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Service interface {
	CreateJob(ctx context.Context, orderID string) error
	GetJob(ctx context.Context, jobID string) (*Job, error)
	GetByOrderID(ctx context.Context, orderID string) (*Job, error)
	ListOpenJobs(ctx context.Context) ([]*Job, error)
	ReserveJob(ctx context.Context, jobID, droneID string) (*Job, error)
	CompleteJob(ctx context.Context, jobID string) error
	CancelJob(ctx context.Context, jobID string) error
	CancelJobByOrderID(ctx context.Context, orderID string) error
	ListJobs(ctx context.Context, status *Status, page, limit int) ([]*Job, int, error)
	CreateJobWithTx(ctx context.Context, tx sqlx.ExtContext, orderID string) error
	CreateOrderAndJob(ctx context.Context, o *order.Order) error
	ReserveJobAndAssign(ctx context.Context, jobID, droneID string) (*Job, error)
	GrabOrder(ctx context.Context, orderID uuid.UUID, droneID string) error
	CompleteDelivery(ctx context.Context, orderID uuid.UUID, droneID string, delivered bool) error
	CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error
}

type service struct {
	repo Repository
	db   *sqlx.DB
}

func NewService(repo Repository, db *sqlx.DB) Service {
	return &service{repo: repo, db: db}
}

// --------------------------------------------------------------
func (s *service) CreateJob(ctx context.Context, orderID string) error {
	j := NewJob(orderID)
	return s.repo.Create(ctx, s.db, j)
}

// --------------------------------------------------------------
func (s *service) CreateJobWithTx(ctx context.Context, tx sqlx.ExtContext, orderID string) error {
	j := NewJob(orderID)
	return s.repo.Create(ctx, tx, j)
}

// --------------------------------------------------------------
func (s *service) GetJob(ctx context.Context, jobID string) (*Job, error) {
	j, err := s.repo.GetByID(ctx, s.db, jobID)
	if err != nil {
		return nil, domainerrors.JobNotFound(jobID)
	}
	return j, nil
}

// --------------------------------------------------------------
func (s *service) ListOpenJobs(ctx context.Context) ([]*Job, error) {
	return s.repo.ListByStatus(ctx, s.db, StatusOpen)
}

// --------------------------------------------------------------
func (s *service) ReserveJob(ctx context.Context, jobID, droneID string) (*Job, error) {
	j, err := s.repo.GetByID(ctx, s.db, jobID)
	if err != nil {
		return nil, domainerrors.JobNotFound(jobID)
	}
	if err := j.Reserve(droneID); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, s.db, j); err != nil {
		return nil, domainerrors.NewInternal("failed to reserve job", err)
	}
	return j, nil
}

// --------------------------------------------------------------
func (s *service) CompleteJob(ctx context.Context, jobID string) error {
	j, err := s.repo.GetByID(ctx, s.db, jobID)
	if err != nil {
		return domainerrors.JobNotFound(jobID)
	}
	if err := j.Complete(); err != nil {
		return err
	}
	return s.repo.Update(ctx, s.db, j)
}

// --------------------------------------------------------------
func (s *service) CancelJob(ctx context.Context, jobID string) error {
	return s.repo.CancelByJobID(ctx, s.db, jobID)
}

// --------------------------------------------------------------
func (s *service) CancelJobByOrderID(ctx context.Context, orderID string) error {
	return s.repo.CancelByOrderID(ctx, s.db, orderID)
}

func (s *service) CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error {
	return s.repo.CancelOrderAndJob(ctx, orderID, submittedBy)
}

// --------------------------------------------------------------
func (s *service) GetByOrderID(ctx context.Context, orderID string) (*Job, error) {
	j, err := s.repo.GetByOrderID(ctx, s.db, orderID)
	if err != nil {
		return nil, domainerrors.NewNotFound("job", "order "+orderID)
	}
	return j, nil
}

// --------------------------------------------------------------
func (s *service) ListJobs(ctx context.Context, status *Status, page, limit int) ([]*Job, int, error) {
	return s.repo.ListAll(ctx, s.db, status, page, limit)
}

// --------------------------------------------------------------
func (s *service) CreateOrderAndJob(ctx context.Context, o *order.Order) error {
	return s.repo.CreateOrderAndJob(ctx, o)
}

// --------------------------------------------------------------
func (s *service) ReserveJobAndAssign(ctx context.Context, jobID, droneID string) (*Job, error) {
	return s.repo.ReserveJobAndAssign(ctx, jobID, droneID)
}

// --------------------------------------------------------------
func (s *service) GrabOrder(ctx context.Context, orderID uuid.UUID, droneID string) error {
	return s.repo.GrabOrder(ctx, orderID, droneID)
}

// --------------------------------------------------------------
func (s *service) CompleteDelivery(ctx context.Context, orderID uuid.UUID, droneID string, delivered bool) error {
	return s.repo.CompleteDelivery(ctx, orderID, droneID, delivered)
}
