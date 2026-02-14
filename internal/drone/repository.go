package drone

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const columns = `id, status, latitude, longitude, current_order_id, last_heartbeat, created_at, updated_at`

type Repository interface {
	Upsert(ctx context.Context, ext sqlx.ExtContext, d *Drone) error
	GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Drone, error)
	GetByIDForUpdate(ctx context.Context, ext sqlx.ExtContext, id string) (*Drone, error)
	Update(ctx context.Context, ext sqlx.ExtContext, d *Drone) error
	ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Drone, int, error)
}

type repo struct{}

func NewRepository() Repository {
	return &repo{}
}

func (r *repo) Upsert(ctx context.Context, ext sqlx.ExtContext, d *Drone) error {
	const query = `INSERT INTO drones (id, status, latitude, longitude, current_order_id, last_heartbeat, created_at, updated_at)
		VALUES (:id, :status, :latitude, :longitude, :current_order_id, :last_heartbeat, :created_at, :updated_at)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			current_order_id = EXCLUDED.current_order_id,
			last_heartbeat = EXCLUDED.last_heartbeat,
			updated_at = EXCLUDED.updated_at`
	_, err := sqlx.NamedExecContext(ctx, ext, query, d)
	return err
}

func (r *repo) GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Drone, error) {
	var d Drone
	query := fmt.Sprintf(`SELECT %s FROM drones WHERE id = $1`, columns)
	err := sqlx.GetContext(ctx, ext, &d, query, id)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *repo) GetByIDForUpdate(ctx context.Context, ext sqlx.ExtContext, id string) (*Drone, error) {
	var d Drone
	query := fmt.Sprintf(`SELECT %s FROM drones WHERE id = $1 FOR UPDATE`, columns)
	err := sqlx.GetContext(ctx, ext, &d, query, id)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *repo) Update(ctx context.Context, ext sqlx.ExtContext, d *Drone) error {
	const query = `UPDATE drones SET status = :status, latitude = :latitude, longitude = :longitude,
		current_order_id = :current_order_id, last_heartbeat = :last_heartbeat, updated_at = :updated_at
		WHERE id = :id`
	_, err := sqlx.NamedExecContext(ctx, ext, query, d)
	return err
}

func (r *repo) ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Drone, int, error) {
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
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM drones%s`, where)
	if err := sqlx.GetContext(ctx, ext, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf(`SELECT %s FROM drones%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, columns, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var drones []*Drone
	if err := sqlx.SelectContext(ctx, ext, &drones, dataQuery, args...); err != nil {
		return nil, 0, err
	}

	return drones, total, nil
}
