package delivery

import (
	"context"
	"fmt"

	"drone-delivery/internal/drone"
	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/job"
	"drone-delivery/internal/order"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	CreateOrderAndJob(ctx context.Context, db *sqlx.DB, o *order.Order) error
	CancelOrderAndJob(ctx context.Context, db *sqlx.DB, orderID uuid.UUID, submittedBy string) error
	ReserveJobAndAssign(ctx context.Context, db *sqlx.DB, jobID, droneID string) (*job.Job, error)
	GrabOrder(ctx context.Context, db *sqlx.DB, orderID uuid.UUID, droneID string) error
	CompleteDelivery(ctx context.Context, db *sqlx.DB, orderID uuid.UUID, droneID string, delivered bool) error
}

type repo struct {
	orderRepo order.Repository
	jobRepo   job.Repository
	droneRepo drone.Repository
}

func NewRepository(orderRepo order.Repository, jobRepo job.Repository, droneRepo drone.Repository) Repository {
	return &repo{orderRepo: orderRepo, jobRepo: jobRepo, droneRepo: droneRepo}
}

// --------------------------------------------------------------
// CreateOrderAndJob persists the order and creates its job in one transaction.
func (r *repo) CreateOrderAndJob(ctx context.Context, db *sqlx.DB, o *order.Order) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	if err := r.orderRepo.Create(ctx, tx, o); err != nil {
		return domainerrors.NewInternal("failed to create order", err)
	}

	j := job.NewJob(o.ID.String())
	if err := r.jobRepo.Create(ctx, tx, j); err != nil {
		return domainerrors.NewInternal("failed to create job", err)
	}

	return tx.Commit()
}

// --------------------------------------------------------------
// CancelOrderAndJob cancels the order and its job in one transaction.
func (r *repo) CancelOrderAndJob(ctx context.Context, db *sqlx.DB, orderID uuid.UUID, submittedBy string) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	if err := r.orderRepo.Cancel(ctx, tx, orderID, submittedBy); err != nil {
		return domainerrors.NewInternal("failed to cancel order", err)
	}

	if err := r.jobRepo.CancelByOrderID(ctx, tx, orderID.String()); err != nil {
		return domainerrors.NewInternal("failed to cancel job", err)
	}

	return tx.Commit()
}

// --------------------------------------------------------------
// ReserveJobAndAssign reserves the job, assigns the order to the drone,
// and reserves the drone — all in one transaction.
func (r *repo) ReserveJobAndAssign(ctx context.Context, db *sqlx.DB, jobID, droneID string) (*job.Job, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// 1. Reserve job
	j, err := r.jobRepo.GetByID(ctx, tx, jobID)
	if err != nil {
		return nil, domainerrors.JobNotFound(jobID)
	}
	if err := j.Reserve(droneID); err != nil {
		return nil, err
	}
	if err := r.jobRepo.Update(ctx, tx, j); err != nil {
		return nil, domainerrors.NewInternal("failed to reserve job", err)
	}

	// 2. Assign order to drone
	orderID, err := uuid.Parse(j.OrderID)
	if err != nil {
		return nil, domainerrors.NewValidation("invalid order id in job")
	}
	o, err := r.orderRepo.GetByID(ctx, tx, orderID)
	if err != nil {
		return nil, domainerrors.NewNotFound("order", orderID.String())
	}
	if err := o.Assign(droneID); err != nil {
		return nil, err
	}
	if err := r.orderRepo.Update(ctx, tx, o); err != nil {
		return nil, domainerrors.NewInternal("failed to assign order", err)
	}

	// 3. Reserve drone
	d, err := r.droneRepo.GetByID(ctx, tx, droneID)
	if err != nil {
		return nil, domainerrors.NewNotFound("drone", droneID)
	}
	if err := d.Reserve(orderID); err != nil {
		return nil, err
	}
	if err := r.droneRepo.Update(ctx, tx, d); err != nil {
		return nil, domainerrors.NewInternal("failed to reserve drone", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, domainerrors.NewInternal("failed to commit transaction", err)
	}
	return j, nil
}

// --------------------------------------------------------------
// GrabOrder marks the order as picked up and transitions the drone
// to delivering — all in one transaction.
func (r *repo) GrabOrder(ctx context.Context, db *sqlx.DB, orderID uuid.UUID, droneID string) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// 1. Mark order picked up
	o, err := r.orderRepo.GetByID(ctx, tx, orderID)
	if err != nil {
		return domainerrors.NewNotFound("order", orderID.String())
	}
	if o.AssignedDroneID == nil || *o.AssignedDroneID != droneID {
		return domainerrors.NewForbidden("drone is not assigned to this order")
	}
	if err := o.MarkPickedUp(); err != nil {
		return err
	}
	if err := r.orderRepo.Update(ctx, tx, o); err != nil {
		return domainerrors.NewInternal("failed to update order", err)
	}

	// 2. Drone starts delivery
	d, err := r.droneRepo.GetByID(ctx, tx, droneID)
	if err != nil {
		return domainerrors.NewNotFound("drone", droneID)
	}
	if err := d.StartDelivery(); err != nil {
		return err
	}
	if err := r.droneRepo.Update(ctx, tx, d); err != nil {
		return domainerrors.NewInternal("failed to update drone", err)
	}

	return tx.Commit()
}

// --------------------------------------------------------------
// CompleteDelivery marks the order as delivered/failed, idles the drone,
// and completes the job — all in one transaction.
func (r *repo) CompleteDelivery(ctx context.Context, db *sqlx.DB, orderID uuid.UUID, droneID string, delivered bool) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// 1. Update order status
	o, err := r.orderRepo.GetByID(ctx, tx, orderID)
	if err != nil {
		return domainerrors.NewNotFound("order", orderID.String())
	}
	if o.AssignedDroneID == nil || *o.AssignedDroneID != droneID {
		return domainerrors.NewForbidden("drone is not assigned to this order")
	}
	if delivered {
		if err := o.MarkDelivered(); err != nil {
			return err
		}
	} else {
		if err := o.MarkFailed(); err != nil {
			return err
		}
	}
	if err := r.orderRepo.Update(ctx, tx, o); err != nil {
		return domainerrors.NewInternal("failed to update order", err)
	}

	// 2. Drone goes idle
	d, err := r.droneRepo.GetByID(ctx, tx, droneID)
	if err != nil {
		return domainerrors.NewNotFound("drone", droneID)
	}
	d.GoIdle()
	if err := r.droneRepo.Update(ctx, tx, d); err != nil {
		return domainerrors.NewInternal("failed to update drone", err)
	}

	// 3. Complete the job
	j, err := r.jobRepo.GetByOrderID(ctx, tx, orderID.String())
	if err == nil && j != nil {
		if err := j.Complete(); err == nil {
			if updateErr := r.jobRepo.Update(ctx, tx, j); updateErr != nil {
				return domainerrors.NewInternal(fmt.Sprintf("failed to complete job for order %s", orderID), updateErr)
			}
		}
	}

	return tx.Commit()
}
