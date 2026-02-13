package drone

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Upsert(ctx context.Context, ext sqlx.ExtContext, d *Drone) error
	GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Drone, error)
	Update(ctx context.Context, ext sqlx.ExtContext, d *Drone) error
	ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Drone, int, error)
}

type droneRepository struct{}

func NewDroneRepository() Repository {
	return &droneRepository{}
}

func (r *droneRepository) Upsert(ctx context.Context, ext sqlx.ExtContext, d *Drone) error {
	return nil
}

func (r *droneRepository) GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Drone, error) {
	return nil, nil
}

func (r *droneRepository) Update(ctx context.Context, ext sqlx.ExtContext, d *Drone) error {
	return nil
}

func (r *droneRepository) ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Drone, int, error) {
	return nil, 0, nil
}
