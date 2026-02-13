package job

import (
	"time"

	domainerrors "drone-delivery/internal/errors"

	"github.com/google/uuid"
)

type Status string

const (
	StatusOpen      Status = "OPEN"
	StatusReserved  Status = "RESERVED"
	StatusCompleted Status = "COMPLETED"
	StatusCancelled Status = "CANCELLED"
)

type Job struct {
	ID                string    `db:"id" json:"id"`
	OrderID           string    `db:"order_id" json:"order_id"`
	Status            Status    `db:"status" json:"status"`
	ReservedByDroneID *string   `db:"reserved_by_drone_id" json:"reserved_by_drone_id,omitempty"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
}

func NewJob(orderID string) *Job {
	now := time.Now()
	return &Job{
		ID:        uuid.New().String(),
		OrderID:   orderID,
		Status:    StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (j *Job) Reserve(droneID string) error {
	if j.Status != StatusOpen {
		return domainerrors.JobAlreadyReserved()
	}
	j.Status = StatusReserved
	j.ReservedByDroneID = &droneID
	j.UpdatedAt = time.Now()
	return nil
}

func (j *Job) Complete() error {
	if j.Status != StatusReserved {
		return domainerrors.JobInvalidTransition(string(j.Status), string(StatusCompleted))
	}
	j.Status = StatusCompleted
	j.UpdatedAt = time.Now()
	return nil
}

func (j *Job) Cancel() error {
	if j.Status == StatusCompleted || j.Status == StatusCancelled {
		return domainerrors.JobInvalidTransition(string(j.Status), string(StatusCancelled))
	}
	j.Status = StatusCancelled
	j.ReservedByDroneID = nil
	j.UpdatedAt = time.Now()
	return nil
}
