package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const columns = `id, submitted_by, origin_lat, origin_lng, dest_lat, dest_lng, status, assigned_drone_id, created_at, updated_at`

type Repository interface {
	Create(ctx context.Context, ext sqlx.ExtContext, o *Order) error
	GetByID(ctx context.Context, ext sqlx.ExtContext, id uuid.UUID) (*Order, error)
	Update(ctx context.Context, ext sqlx.ExtContext, o *Order) error
	ListBySubmitter(ctx context.Context, ext sqlx.ExtContext, submittedBy string) ([]*Order, error)
	ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Order, int, error)
	GetByDroneID(ctx context.Context, ext sqlx.ExtContext, droneID string) (*Order, error)
	Cancel(ctx context.Context, ext sqlx.ExtContext, orderID uuid.UUID, submittedBy string) error
}

type orderRepository struct{}

func NewOrderRepository() Repository {
	return &orderRepository{}
}

func (r *orderRepository) Create(ctx context.Context, ext sqlx.ExtContext, o *Order) error {
	const query = `INSERT INTO orders (id, submitted_by, origin_lat, origin_lng, dest_lat, dest_lng, status, assigned_drone_id, created_at, updated_at)
		VALUES (:id, :submitted_by, :origin_lat, :origin_lng, :dest_lat, :dest_lng, :status, :assigned_drone_id, :created_at, :updated_at)`

	_, err := sqlx.NamedExecContext(ctx, ext, query, o)
	return err
}

func (r *orderRepository) GetByID(ctx context.Context, ext sqlx.ExtContext, id uuid.UUID) (*Order, error) {
	var o Order
	query := fmt.Sprintf(`SELECT %s FROM orders WHERE id = $1`, columns)
	err := sqlx.GetContext(ctx, ext, &o, query, id)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepository) Update(ctx context.Context, ext sqlx.ExtContext, o *Order) error {
	const query = `UPDATE orders SET status = :status, assigned_drone_id = :assigned_drone_id, origin_lat = :origin_lat, origin_lng = :origin_lng, dest_lat = :dest_lat, dest_lng = :dest_lng, updated_at = :updated_at WHERE id = :id`
	_, err := sqlx.NamedExecContext(ctx, ext, query, o)
	return err
}

func (r *orderRepository) ListBySubmitter(ctx context.Context, ext sqlx.ExtContext, submittedBy string) ([]*Order, error) {
	var orders []*Order
	query := fmt.Sprintf(`SELECT %s FROM orders WHERE submitted_by = $1 ORDER BY created_at DESC`, columns)
	err := sqlx.SelectContext(ctx, ext, &orders, query, submittedBy)
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *orderRepository) ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Order, int, error) {
	offset := (page - 1) * limit
	args := []any{}
	argIdx := 1

	where := ""
	if status != nil {
		where = fmt.Sprintf(" WHERE status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}

	// Total count.
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM orders%s`, where)
	if err := sqlx.GetContext(ctx, ext, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	// Page results.
	dataQuery := fmt.Sprintf(`SELECT %s FROM orders%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, columns, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var orders []*Order
	if err := sqlx.SelectContext(ctx, ext, &orders, dataQuery, args...); err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (r *orderRepository) Cancel(ctx context.Context, ext sqlx.ExtContext, orderID uuid.UUID, submittedBy string) error {
	const query = `UPDATE orders SET status = 'CANCELLED', submitted_by = $2, updated_at = NOW()
		WHERE id = $1 AND status NOT IN ('COMPLETED', 'CANCELLED')`
	res, err := ext.ExecContext(ctx, query, orderID, submittedBy)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("order %s not found or already completed/cancelled", orderID)
	}
	return nil
}

func (r *orderRepository) GetByDroneID(ctx context.Context, ext sqlx.ExtContext, droneID string) (*Order, error) {
	var o Order
	query := fmt.Sprintf(`SELECT %s FROM orders WHERE assigned_drone_id = $1`, columns)
	err := sqlx.GetContext(ctx, ext, &o, query, droneID)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
