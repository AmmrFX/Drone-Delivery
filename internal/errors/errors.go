package errors

import "fmt"

const (
	ErrNotFound          = "NOT_FOUND"
	ErrInvalidTransition = "INVALID_TRANSITION"
	ErrUnauthorized      = "UNAUTHORIZED"
	ErrForbidden         = "FORBIDDEN"
	ErrConflict          = "CONFLICT"
	ErrValidation        = "VALIDATION"
	ErrOutOfZone         = "OUT_OF_ZONE"
	ErrInternal          = "INTERNAL"
)

type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

func Wrap(code, msg string, err error) *DomainError {
	return &DomainError{Code: code, Message: msg, Err: err}
}

// --- Generic ---

func NewNotFound(entity, id string) *DomainError {
	return &DomainError{Code: ErrNotFound, Message: fmt.Sprintf("%s with id %s not found", entity, id)}
}

func NewInvalidTransition(from, to string) *DomainError {
	return &DomainError{Code: ErrInvalidTransition, Message: fmt.Sprintf("cannot transition from %s to %s", from, to)}
}

func NewUnauthorized(msg string) *DomainError {
	return &DomainError{Code: ErrUnauthorized, Message: msg}
}

func NewForbidden(msg string) *DomainError {
	return &DomainError{Code: ErrForbidden, Message: msg}
}

func NewConflict(msg string) *DomainError {
	return &DomainError{Code: ErrConflict, Message: msg}
}

func NewValidation(msg string) *DomainError {
	return &DomainError{Code: ErrValidation, Message: msg}
}

func NewOutOfZone(msg string) *DomainError {
	return &DomainError{Code: ErrOutOfZone, Message: msg}
}

func NewInternal(msg string, err error) *DomainError {
	return &DomainError{Code: ErrInternal, Message: msg, Err: err}
}

// --- Order ---

func OrderNotFound(id string) *DomainError {
	return NewNotFound("order", id)
}

func OrderInvalidTransition(from, to string) *DomainError {
	return NewInvalidTransition(from, to)
}

func OrderNotOwner() *DomainError {
	return NewForbidden("you do not own this order")
}

// --- Drone ---

func DroneNotFound(id string) *DomainError {
	return NewNotFound("drone", id)
}

func DroneInvalidTransition(from, to string) *DomainError {
	return NewInvalidTransition(from, to)
}

func DroneNotAssigned() *DomainError {
	return NewForbidden("drone is not assigned to this order")
}

func DroneAlreadyBroken() *DomainError {
	return NewConflict("drone is already broken")
}

// --- Job ---

func JobNotFound(id string) *DomainError {
	return NewNotFound("job", id)
}

func JobAlreadyReserved() *DomainError {
	return NewConflict("job is already reserved by another drone")
}

func JobInvalidTransition(from, to string) *DomainError {
	return NewInvalidTransition(from, to)
}
