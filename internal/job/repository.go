package job

import (
	"context"
	"fmt"

	"drone-delivery/internal/drone"
	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/order"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const columns = `id, order_id, status, reserved_by_drone_id, created_at, updated_at`

type Repository interface {
	Create(ctx context.Context, ext sqlx.ExtContext, j *Job) error
	GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Job, error)
	Update(ctx context.Context, ext sqlx.ExtContext, j *Job) error
	ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Job, int, error)
	ListByStatus(ctx context.Context, ext sqlx.ExtContext, status Status) ([]*Job, error)
	GetByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error)
	CancelByJobID(ctx context.Context, ext sqlx.ExtContext, id string) error
	CancelByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) error

	// Cross-domain transactional operations
	CreateOrderAndJob(ctx context.Context, o *order.Order) error
	ReserveJobAndAssign(ctx context.Context, jobID, droneID string) (*Job, error)
	GrabOrder(ctx context.Context, orderID uuid.UUID, droneID string) error
	CompleteDelivery(ctx context.Context, orderID uuid.UUID, droneID string, delivered bool) error
	CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error
}

type jobRepository struct {
	db        *sqlx.DB
	orderRepo order.Repository
	droneRepo drone.Repository
}

func NewJobRepository(db *sqlx.DB, orderRepo order.Repository, droneRepo drone.Repository) Repository {
	return &jobRepository{db: db, orderRepo: orderRepo, droneRepo: droneRepo}
}

// --------------------------------------------------------------
func (r *jobRepository) Create(ctx context.Context, ext sqlx.ExtContext, j *Job) error {
	const query = `INSERT INTO jobs (id, order_id, status, reserved_by_drone_id, created_at, updated_at)
		VALUES (:id, :order_id, :status, :reserved_by_drone_id, :created_at, :updated_at)`
	_, err := sqlx.NamedExecContext(ctx, ext, query, j)
	return err
}

// --------------------------------------------------------------
func (r *jobRepository) GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE id = $1`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, id)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// --------------------------------------------------------------
func (r *jobRepository) Update(ctx context.Context, ext sqlx.ExtContext, j *Job) error {
	const query = `UPDATE jobs SET status = :status, reserved_by_drone_id = :reserved_by_drone_id, updated_at = :updated_at WHERE id = :id`
	_, err := sqlx.NamedExecContext(ctx, ext, query, j)
	return err
}

// --------------------------------------------------------------
func (r *jobRepository) ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Job, int, error) {
	offset := (page - 1) * limit
	args := []any{}
	argIdx := 1

	where := ""
	if status != nil {
		where = fmt.Sprintf(" WHERE status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM jobs%s`, where)
	if err := sqlx.GetContext(ctx, ext, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf(`SELECT %s FROM jobs%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, columns, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var jobs []*Job
	if err := sqlx.SelectContext(ctx, ext, &jobs, dataQuery, args...); err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}

// --------------------------------------------------------------
func (r *jobRepository) ListByStatus(ctx context.Context, ext sqlx.ExtContext, status Status) ([]*Job, error) {
	var jobs []*Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE status = $1 ORDER BY created_at ASC`, columns)
	err := sqlx.SelectContext(ctx, ext, &jobs, query, status)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// --------------------------------------------------------------
func (r *jobRepository) GetByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE order_id = $1 AND status != 'CANCELLED' ORDER BY created_at DESC LIMIT 1`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, orderID)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// --------------------------------------------------------------
func (r *jobRepository) CancelByJobID(ctx context.Context, ext sqlx.ExtContext, id string) error {
	const query = `UPDATE jobs SET status = 'CANCELLED', reserved_by_drone_id = NULL, updated_at = NOW()
		WHERE id = $1 AND status NOT IN ('COMPLETED', 'CANCELLED')`
	res, err := ext.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("job %s not found or already completed/cancelled", id)
	}
	return nil
}

// --------------------------------------------------------------
func (r *jobRepository) CancelByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) error {
	const query = `UPDATE jobs SET status = 'CANCELLED', reserved_by_drone_id = NULL, updated_at = NOW()
		WHERE order_id = $1 AND status NOT IN ('COMPLETED', 'CANCELLED')`
	res, err := ext.ExecContext(ctx, query, orderID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("job for order %s not found or already completed/cancelled", orderID)
	}
	return nil
}

func (r *jobRepository) CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domainerrors.NewInternal("failed to begin transaction", err)

	}
	defer tx.Rollback()

	if err := r.orderRepo.Cancel(ctx, tx, orderID, submittedBy); err != nil {
		return domainerrors.NewInternal("failed to cancel order", err)
	}

	if err := r.CancelByOrderID(ctx, tx, orderID.String()); err != nil {
		return domainerrors.NewInternal("failed to cancel job", err)
	}

	return tx.Commit()
}

// --------------------------------------------------------------
// CreateOrderAndJob persists the order and creates its job — all in one transaction.
func (r *jobRepository) CreateOrderAndJob(ctx context.Context, o *order.Order) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	if err := r.orderRepo.Create(ctx, tx, o); err != nil {
		return domainerrors.NewInternal("failed to create order", err)
	}

	j := NewJob(o.ID.String())
	if err := r.Create(ctx, tx, j); err != nil {
		return domainerrors.NewInternal("failed to create job", err)
	}

	return tx.Commit()
}

// --------------------------------------------------------------
// ReserveJobAndAssign reserves the job, assigns the order to the drone, and reserves the drone — all in one transaction.
func (r *jobRepository) ReserveJobAndAssign(ctx context.Context, jobID, droneID string) (*Job, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, domainerrors.NewInternal("failed to begin transaction", err)
	}
	defer tx.Rollback()

	// 1. Reserve job
	j, err := r.GetByID(ctx, tx, jobID)
	if err != nil {
		return nil, domainerrors.JobNotFound(jobID)
	}
	if err := j.Reserve(droneID); err != nil {
		return nil, err
	}
	if err := r.Update(ctx, tx, j); err != nil {
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

// GrabOrder marks the order as picked up and transitions the drone to delivering — all in one transaction.
func (r *jobRepository) GrabOrder(ctx context.Context, orderID uuid.UUID, droneID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
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

// CompleteDelivery marks the order as delivered/failed, idles the drone, and completes the job — all in one transaction.
func (r *jobRepository) CompleteDelivery(ctx context.Context, orderID uuid.UUID, droneID string, delivered bool) error {
	tx, err := r.db.BeginTxx(ctx, nil)
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
	j, err := r.GetByOrderID(ctx, tx, orderID.String())
	if err == nil && j != nil {
		if err := j.Complete(); err == nil {
			_ = r.Update(ctx, tx, j)
		}
	}

	return tx.Commit()
}
