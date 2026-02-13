package drone

import (
	"context"

	"github.com/jmoiron/sqlx"

	"drone-delivery/internal/common"
	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/redis"
)

type Service interface {
	EnsureExists(ctx context.Context, droneID string) (*Drone, error)
	GetByID(ctx context.Context, id string) (*Drone, error)
	GetByIDWithTx(ctx context.Context, tx sqlx.ExtContext, id string) (*Drone, error)
	UpdateWithTx(ctx context.Context, tx sqlx.ExtContext, d *Drone) error
	Heartbeat(ctx context.Context, droneID string, lat, lng float64) (*Drone, error)
	GetDroneLocation(ctx context.Context, droneID string) (*common.Location, error)
	ListAll(ctx context.Context, status *Status, page, limit int) ([]*Drone, int, error)
	UpdateStatus(ctx context.Context, droneID string, d *Drone) error
}

type service struct {
	repo       Repository
	db         *sqlx.DB
	cache      *redis.DroneLocationCache
	zoneCenter common.Location
	zoneRadius float64
}

func NewDroneService(repo Repository, db *sqlx.DB, cache *redis.DroneLocationCache, zoneCenter common.Location, zoneRadius float64) Service {
	return &service{
		repo:       repo,
		db:         db,
		cache:      cache,
		zoneCenter: zoneCenter,
		zoneRadius: zoneRadius,
	}
}

func (s *service) EnsureExists(ctx context.Context, droneID string) (*Drone, error) {
	d, err := s.repo.GetByID(ctx, s.db, droneID)
	if err != nil {
		d = New(droneID)
		if err := s.repo.Upsert(ctx, s.db, d); err != nil {
			return nil, domainerrors.NewInternal("failed to create drone", err)
		}
	}
	return d, nil
}

func (s *service) GetByID(ctx context.Context, id string) (*Drone, error) {
	d, err := s.repo.GetByID(ctx, s.db, id)
	if err != nil {
		return nil, domainerrors.DroneNotFound(id)
	}
	return d, nil
}

func (s *service) GetByIDWithTx(ctx context.Context, tx sqlx.ExtContext, id string) (*Drone, error) {
	return s.repo.GetByID(ctx, tx, id)
}

func (s *service) UpdateWithTx(ctx context.Context, tx sqlx.ExtContext, d *Drone) error {
	return s.repo.Update(ctx, tx, d)
}

func (s *service) Heartbeat(ctx context.Context, droneID string, lat, lng float64) (*Drone, error) {
	if err := common.ValidateLatLng(lat, lng); err != nil {
		return nil, domainerrors.NewValidation(err.Error())
	}
	loc := common.NewLocation(lat, lng)
	if err := common.ValidateInZone(loc, s.zoneCenter, s.zoneRadius); err != nil {
		return nil, domainerrors.NewOutOfZone("heartbeat location is outside the delivery zone")
	}

	d, err := s.EnsureExists(ctx, droneID)
	if err != nil {
		return nil, err
	}

	d.UpdateLocation(lat, lng)
	if err := s.repo.Update(ctx, s.db, d); err != nil {
		return nil, domainerrors.NewInternal("failed to update drone location", err)
	}

	_ = s.cache.Set(ctx, droneID, loc)

	return d, nil
}

func (s *service) GetDroneLocation(ctx context.Context, droneID string) (*common.Location, error) {
	cached, err := s.cache.Get(ctx, droneID)
	if err == nil && cached != nil {
		loc := common.NewLocation(cached.Lat, cached.Lng)
		return &loc, nil
	}

	d, err := s.repo.GetByID(ctx, s.db, droneID)
	if err != nil {
		return nil, nil
	}
	loc := d.Location()

	_ = s.cache.Set(ctx, droneID, loc)

	return &loc, nil
}

func (s *service) ListAll(ctx context.Context, status *Status, page, limit int) ([]*Drone, int, error) {
	return s.repo.ListAll(ctx, s.db, status, page, limit)
}

func (s *service) UpdateStatus(ctx context.Context, droneID string, d *Drone) error {
	return s.repo.Update(ctx, s.db, d)
}
