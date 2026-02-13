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
	Update(ctx context.Context, ext sqlx.ExtContext, j *Job) error
	ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Job, int, error)
	ListByStatus(ctx context.Context, ext sqlx.ExtContext, status Status) ([]*Job, error)
	GetByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error)
}

type jobRepository struct{}

func NewJobRepository() Repository {
	return &jobRepository{}
}

func (r *jobRepository) Create(ctx context.Context, ext sqlx.ExtContext, j *Job) error {
	const query = `INSERT INTO jobs (id, order_id, status, reserved_by_drone_id, created_at, updated_at)
		VALUES (:id, :order_id, :status, :reserved_by_drone_id, :created_at, :updated_at)`
	_, err := sqlx.NamedExecContext(ctx, ext, query, j)
	return err
}

func (r *jobRepository) GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE id = $1`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, id)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *jobRepository) Update(ctx context.Context, ext sqlx.ExtContext, j *Job) error {
	const query = `UPDATE jobs SET status = :status, reserved_by_drone_id = :reserved_by_drone_id, updated_at = :updated_at WHERE id = :id`
	_, err := sqlx.NamedExecContext(ctx, ext, query, j)
	return err
}

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

func (r *jobRepository) ListByStatus(ctx context.Context, ext sqlx.ExtContext, status Status) ([]*Job, error) {
	var jobs []*Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE status = $1 ORDER BY created_at ASC`, columns)
	err := sqlx.SelectContext(ctx, ext, &jobs, query, status)
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *jobRepository) GetByOrderID(ctx context.Context, ext sqlx.ExtContext, orderID string) (*Job, error) {
	var j Job
	query := fmt.Sprintf(`SELECT %s FROM jobs WHERE order_id = $1 AND status != 'CANCELLED' ORDER BY created_at DESC LIMIT 1`, columns)
	err := sqlx.GetContext(ctx, ext, &j, query, orderID)
	if err != nil {
		return nil, err
	}
	return &j, nil
}
