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

type repo struct{}

func NewRepository() Repository {
	return &repo{}
}

func (r *repo) Upsert(ctx context.Context, ext sqlx.ExtContext, d *Drone) error {
	return nil
}

func (r *repo) GetByID(ctx context.Context, ext sqlx.ExtContext, id string) (*Drone, error) {
	return nil, nil
}

func (r *repo) Update(ctx context.Context, ext sqlx.ExtContext, d *Drone) error {
	return nil
}

func (r *repo) ListAll(ctx context.Context, ext sqlx.ExtContext, status *Status, page, limit int) ([]*Drone, int, error) {
	return nil, 0, nil
}
