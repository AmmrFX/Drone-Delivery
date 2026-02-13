package order

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"drone-delivery/internal/common"
	domainerrors "drone-delivery/internal/errors"
)

type Service interface {
	PlaceOrder(ctx context.Context, submittedBy string, origin, destination common.Location) (*Order, error)
	WithdrawOrder(ctx context.Context, orderID uuid.UUID, submittedBy string) error
	GetOrderDetails(ctx context.Context, orderID uuid.UUID, submittedBy string) (*Order, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
	GetByDroneID(ctx context.Context, droneID string) (*Order, error)
	SaveWithTx(ctx context.Context, tx sqlx.ExtContext, o *Order) error
	GetByIDWithTx(ctx context.Context, tx sqlx.ExtContext, id uuid.UUID) (*Order, error)
	AwaitHandoffWithTx(ctx context.Context, tx sqlx.ExtContext, orderID uuid.UUID) error
	ListAll(ctx context.Context, status *Status, page, limit int) ([]*Order, int, error)
	AdminUpdateOrder(ctx context.Context, orderID uuid.UUID, origin, destination *common.Location) (*Order, error)
}

type service struct {
	repo   Repository
	db     *sqlx.DB
	zone   ZoneConfig
	mapbox *common.MapboxClient
}

type ZoneConfig struct {
	CenterLat float64
	CenterLng float64
	RadiusKM  float64
}

func NewOrderService(repo Repository, db *sqlx.DB, zone ZoneConfig, mapbox *common.MapboxClient) Service {
	return &service{repo: repo, db: db, zone: zone, mapbox: mapbox}
}

// -------------------------------------------------------------------------------------------------
func (s *service) validateLocation(loc common.Location, label string) error {
	if err := common.ValidateLatLng(loc.Lat, loc.Lng); err != nil {
		return domainerrors.NewValidation(err.Error())
	}
	center := common.NewLocation(s.zone.CenterLat, s.zone.CenterLng)
	if err := common.ValidateInZone(loc, center, s.zone.RadiusKM); err != nil {
		return domainerrors.NewOutOfZone(label + " is outside the delivery zone")
	}
	return nil
}

// -------------------------------------------------------------------------------------------------
func (s *service) PlaceOrder(ctx context.Context, submittedBy string, origin, destination common.Location) (*Order, error) {
	if err := s.validateLocation(origin, "origin"); err != nil {
		return nil, err
	}
	if err := s.validateLocation(destination, "destination"); err != nil {
		return nil, err
	}

	o := NewOrder(submittedBy, origin, destination)
	if err := s.repo.Create(ctx, s.db, o); err != nil {
		return nil, domainerrors.NewInternal("failed to create order", err)
	}

	return o, nil
}

// -------------------------------------------------------------------------------------------------
func (s *service) WithdrawOrder(ctx context.Context, orderID uuid.UUID, submittedBy string) error {
	o, err := s.repo.GetByID(ctx, s.db, orderID)
	if err != nil {
		return domainerrors.OrderNotFound(orderID.String())
	}
	if o.SubmittedBy != submittedBy {
		return domainerrors.OrderNotOwner()
	}
	if err := o.Withdraw(); err != nil {
		return err
	}
	return s.repo.Update(ctx, s.db, o)
}

// -------------------------------------------------------------------------------------------------
func (s *service) GetOrderDetails(ctx context.Context, orderID uuid.UUID, submittedBy string) (*Order, error) {
	o, err := s.repo.GetByID(ctx, s.db, orderID)
	if err != nil {
		return nil, domainerrors.OrderNotFound(orderID.String())
	}
	if o.SubmittedBy != submittedBy {
		return nil, domainerrors.OrderNotOwner()
	}
	return o, nil
}

// -------------------------------------------------------------------------------------------------
func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	o, err := s.repo.GetByID(ctx, s.db, id)
	if err != nil {
		return nil, domainerrors.OrderNotFound(id.String())
	}
	return o, nil
}

// -------------------------------------------------------------------------------------------------
func (s *service) GetByDroneID(ctx context.Context, droneID string) (*Order, error) {
	o, err := s.repo.GetByDroneID(ctx, s.db, droneID)
	if err != nil {
		return nil, domainerrors.NewNotFound("order", "assigned to drone "+droneID)
	}
	return o, nil
}

// -------------------------------------------------------------------------------------------------
func (s *service) SaveWithTx(ctx context.Context, tx sqlx.ExtContext, o *Order) error {
	return s.repo.Update(ctx, tx, o)
}

// -------------------------------------------------------------------------------------------------
func (s *service) GetByIDWithTx(ctx context.Context, tx sqlx.ExtContext, id uuid.UUID) (*Order, error) {
	return s.repo.GetByID(ctx, tx, id)
}

// -------------------------------------------------------------------------------------------------
func (s *service) ListAll(ctx context.Context, status *Status, page, limit int) ([]*Order, int, error) {
	return s.repo.ListAll(ctx, s.db, status, page, limit)
}

// -------------------------------------------------------------------------------------------------
func (s *service) AdminUpdateOrder(ctx context.Context, orderID uuid.UUID, origin, destination *common.Location) (*Order, error) {
	o, err := s.repo.GetByID(ctx, s.db, orderID)
	if err != nil {
		return nil, domainerrors.OrderNotFound(orderID.String())
	}

	if origin != nil {
		if err := s.validateLocation(*origin, "new origin"); err != nil {
			return nil, err
		}
		if err := o.UpdateOrigin(*origin); err != nil {
			return nil, err
		}
	}
	if destination != nil {
		if err := s.validateLocation(*destination, "new destination"); err != nil {
			return nil, err
		}
		if err := o.UpdateDestination(*destination); err != nil {
			return nil, err
		}
	}

	if err := s.repo.Update(ctx, s.db, o); err != nil {
		return nil, domainerrors.NewInternal("failed to update order", err)
	}
	return o, nil
}

// -------------------------------------------------------------------------------------------------
func (s *service) AwaitHandoffWithTx(ctx context.Context, tx sqlx.ExtContext, orderID uuid.UUID) error {
	o, err := s.repo.GetByID(ctx, tx, orderID)
	if err != nil {
		return domainerrors.OrderNotFound(orderID.String())
	}
	if err := o.AwaitHandoff(); err != nil {
		return err
	}
	return s.repo.Update(ctx, tx, o)
}
