package job

import (
	"context"

	domainerrors "drone-delivery/internal/errors"

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
}

type service struct {
	db   *sqlx.DB
	repo Repository
}

func NewService(repo Repository, db *sqlx.DB) Service {
	return &service{db: db, repo: repo}
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
