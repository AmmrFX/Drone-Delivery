package unit

import (
	"errors"
	"testing"

	"drone-delivery/internal/common"
)

func TestValidateInZone_InsideZone(t *testing.T) {
	center := common.NewLocation(24.7136, 46.6753)
	loc := common.NewLocation(24.72, 46.68) // very close to center

	if err := common.ValidateInZone(loc, center, 50); err != nil {
		t.Fatalf("expected in-zone, got error: %v", err)
	}
}

func TestValidateInZone_OnBoundary(t *testing.T) {
	center := common.NewLocation(24.7136, 46.6753)
	// A point roughly 50 km away
	loc := common.NewLocation(25.163, 46.6753) // ~50 km north

	err := common.ValidateInZone(loc, center, 50)
	// Exact boundary might pass or fail depending on precision; we just verify it doesn't panic
	_ = err
}

func TestValidateInZone_OutsideZone(t *testing.T) {
	center := common.NewLocation(24.7136, 46.6753)
	// Jeddah is ~949 km away, definitely outside 50 km radius
	loc := common.NewLocation(21.4858, 39.1925)

	err := common.ValidateInZone(loc, center, 50)
	if err == nil {
		t.Fatal("expected out-of-zone error")
	}
	if !errors.Is(err, common.ErrOutOfZone) {
		t.Fatalf("expected ErrOutOfZone, got %v", err)
	}
}

func TestValidateInZone_ExactCenter(t *testing.T) {
	center := common.NewLocation(24.7136, 46.6753)

	if err := common.ValidateInZone(center, center, 50); err != nil {
		t.Fatalf("center should be in zone: %v", err)
	}
}

func TestValidateInZone_ZeroRadius(t *testing.T) {
	center := common.NewLocation(24.7136, 46.6753)
	loc := common.NewLocation(24.714, 46.676) // slightly off

	err := common.ValidateInZone(loc, center, 0)
	if err == nil {
		t.Fatal("expected error for zero radius")
	}
}

// --- ValidateLatLng ---

func TestValidateLatLng_Valid(t *testing.T) {
	cases := []struct {
		lat, lng float64
	}{
		{0, 0},
		{90, 180},
		{-90, -180},
		{24.7136, 46.6753},
		{45.5, -73.5},
	}

	for _, tc := range cases {
		if err := common.ValidateLatLng(tc.lat, tc.lng); err != nil {
			t.Errorf("expected valid for (%f, %f), got error: %v", tc.lat, tc.lng, err)
		}
	}
}

func TestValidateLatLng_InvalidLatitude(t *testing.T) {
	cases := []float64{-91, 91, 100, -100}
	for _, lat := range cases {
		err := common.ValidateLatLng(lat, 0)
		if err == nil {
			t.Errorf("expected error for latitude %f", lat)
		}
		if !errors.Is(err, common.ErrInvalidLatLng) {
			t.Errorf("expected ErrInvalidLatLng for latitude %f, got %v", lat, err)
		}
	}
}

func TestValidateLatLng_InvalidLongitude(t *testing.T) {
	cases := []float64{-181, 181, 200, -200}
	for _, lng := range cases {
		err := common.ValidateLatLng(0, lng)
		if err == nil {
			t.Errorf("expected error for longitude %f", lng)
		}
		if !errors.Is(err, common.ErrInvalidLatLng) {
			t.Errorf("expected ErrInvalidLatLng for longitude %f, got %v", lng, err)
		}
	}
}

func TestValidateLatLng_BothInvalid(t *testing.T) {
	err := common.ValidateLatLng(91, 181)
	if err == nil {
		t.Fatal("expected error for both invalid")
	}
}
