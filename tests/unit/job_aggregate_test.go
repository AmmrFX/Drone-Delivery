package unit

import (
	"testing"

	domainerrors "drone-delivery/internal/errors"
	"drone-delivery/internal/job"
)

func newOpenJob() *job.Job {
	return job.NewJob("order-123")
}

func TestNewJob_DefaultsOpen(t *testing.T) {
	j := newOpenJob()

	if j.Status != job.StatusOpen {
		t.Fatalf("expected OPEN, got %s", j.Status)
	}
	if j.OrderID != "order-123" {
		t.Fatalf("expected order-123, got %s", j.OrderID)
	}
	if j.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if j.ReservedByDroneID != nil {
		t.Fatal("expected no reserved drone")
	}
}

// --- Reserve ---

func TestJob_Reserve_FromOpen(t *testing.T) {
	j := newOpenJob()
	if err := j.Reserve("drone-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j.Status != job.StatusReserved {
		t.Fatalf("expected RESERVED, got %s", j.Status)
	}
	if j.ReservedByDroneID == nil || *j.ReservedByDroneID != "drone-1" {
		t.Fatal("drone ID not set correctly")
	}
}

func TestJob_Reserve_AlreadyReserved_Fails(t *testing.T) {
	j := newOpenJob()
	_ = j.Reserve("drone-1")

	err := j.Reserve("drone-2")
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domainerrors.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if de.Code != domainerrors.ErrConflict {
		t.Fatalf("expected CONFLICT, got %s", de.Code)
	}
}

func TestJob_Reserve_FromCompleted_Fails(t *testing.T) {
	j := newOpenJob()
	_ = j.Reserve("drone-1")
	_ = j.Complete()

	if err := j.Reserve("drone-2"); err == nil {
		t.Fatal("expected error")
	}
}

// --- Complete ---

func TestJob_Complete_FromReserved(t *testing.T) {
	j := newOpenJob()
	_ = j.Reserve("drone-1")

	if err := j.Complete(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j.Status != job.StatusCompleted {
		t.Fatalf("expected COMPLETED, got %s", j.Status)
	}
}

func TestJob_Complete_FromOpen_Fails(t *testing.T) {
	j := newOpenJob()
	err := j.Complete()
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domainerrors.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if de.Code != domainerrors.ErrInvalidTransition {
		t.Fatalf("expected INVALID_TRANSITION, got %s", de.Code)
	}
}

// --- Cancel ---

func TestJob_Cancel_FromOpen(t *testing.T) {
	j := newOpenJob()
	if err := j.Cancel(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j.Status != job.StatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", j.Status)
	}
}

func TestJob_Cancel_FromReserved(t *testing.T) {
	j := newOpenJob()
	_ = j.Reserve("drone-1")
	if err := j.Cancel(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j.Status != job.StatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", j.Status)
	}
	if j.ReservedByDroneID != nil {
		t.Fatal("expected reserved drone to be cleared")
	}
}

func TestJob_Cancel_FromCompleted_Fails(t *testing.T) {
	j := newOpenJob()
	_ = j.Reserve("drone-1")
	_ = j.Complete()

	err := j.Cancel()
	if err == nil {
		t.Fatal("expected error")
	}
	de, ok := err.(*domainerrors.DomainError)
	if !ok {
		t.Fatalf("expected DomainError, got %T", err)
	}
	if de.Code != domainerrors.ErrInvalidTransition {
		t.Fatalf("expected INVALID_TRANSITION, got %s", de.Code)
	}
}

func TestJob_Cancel_FromCancelled_Fails(t *testing.T) {
	j := newOpenJob()
	_ = j.Cancel()

	if err := j.Cancel(); err == nil {
		t.Fatal("expected error when cancelling already cancelled job")
	}
}

// --- Full lifecycle ---

func TestJob_FullLifecycle(t *testing.T) {
	j := newOpenJob()

	if err := j.Reserve("drone-1"); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := j.Complete(); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if j.Status != job.StatusCompleted {
		t.Fatalf("expected COMPLETED, got %s", j.Status)
	}
}

func TestJob_ReserveThenCancel(t *testing.T) {
	j := newOpenJob()

	_ = j.Reserve("drone-1")
	if err := j.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if j.Status != job.StatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", j.Status)
	}
}
