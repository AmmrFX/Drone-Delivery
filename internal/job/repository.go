package job

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const columns = `id, order_id, status, reserved_by_drone_id, created_at, updated_at`

type Repository interface {
	Create(ctx context.Context, ext sqlx.ExtContext, j *Job) error
	GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Job, error)
	GetByIDForUpdate(ctx context.Context, ext sqlx.ExtContext, id string) (*Job, error)
	Update(ctx context.Context, ext sqlx.ExtContext, j *Job) error
	ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Job, int, error)
	ListByStatus(ctx context.Context, ext sqlx.ExtContext, status Status) ([]*Job, error)
	GetByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error)
	GetByOrderIDForUpdate(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error)
	CancelByJobID(ctx context.Context, ext sqlx.ExtContext, id string) error
	CancelByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) error
}

type repo struct{}

func NewRepository() Repository {
	return &repo{}
}

// --------------------------------------------------------------
func (r *repo) Create(ctx context.Context, ext sqlx.ExtContext, j *Job) error {
	const query = `INSERT INTO jobs (id, order_id, status, reserved_by_drone_id, created_at, updated_at)
		VALUES (:id, :order_id, :status, :reserved_by_drone_id, :created_at, :updated_at)`
	_, err := sqlx.NamedExecContext(ctx, ext, query, j)
	return err
}

// --------------------------------------------------------------
func (r *repo) GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE id = $1`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, id)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// --------------------------------------------------------------
func (r *repo) GetByIDForUpdate(ctx context.Context, ext sqlx.ExtContext, id string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE id = $1 FOR UPDATE`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, id)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// --------------------------------------------------------------
func (r *repo) Update(ctx context.Context, ext sqlx.ExtContext, j *Job) error {
	const query = `UPDATE jobs SET status = :status, reserved_by_drone_id = :reserved_by_drone_id, updated_at = :updated_at WHERE id = :id`
	_, err := sqlx.NamedExecContext(ctx, ext, query, j)
	return err
}

// --------------------------------------------------------------
func (r *repo) ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Job, int, error) {
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
func (r *repo) ListByStatus(ctx context.Context, ext sqlx.ExtContext, status Status) ([]*Job, error) {
	var jobs []*Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE status = $1 ORDER BY created_at ASC`, columns)
	err := sqlx.SelectContext(ctx, ext, &jobs, query, status)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// --------------------------------------------------------------
func (r *repo) GetByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE order_id = $1 AND status != 'CANCELLED' ORDER BY created_at DESC LIMIT 1`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, orderID)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// --------------------------------------------------------------
func (r *repo) GetByOrderIDForUpdate(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE order_id = $1 AND status != 'CANCELLED' ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, orderID)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

// --------------------------------------------------------------
func (r *repo) CancelByJobID(ctx context.Context, ext sqlx.ExtContext, id string) error {
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
func (r *repo) CancelByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) error {
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
