package unit

import (
	"math"
	"testing"

	"drone-delivery/internal/common"
)

func TestHaversineDistance_SamePoint(t *testing.T) {
	a := common.NewLocation(24.7136, 46.6753)
	dist := common.HaversineDistance(a, a)
	if dist != 0 {
		t.Fatalf("expected 0 distance for same point, got %f", dist)
	}
}

func TestHaversineDistance_KnownPair(t *testing.T) {
	// Riyadh to Jeddah: approximately 845 km (haversine / straight-line)
	riyadh := common.NewLocation(24.7136, 46.6753)
	jeddah := common.NewLocation(21.4858, 39.1925)

	dist := common.HaversineDistance(riyadh, jeddah)

	if math.Abs(dist-845) > 20 {
		t.Fatalf("expected ~845 km, got %f km", dist)
	}
}

func TestHaversineDistance_ShortDistance(t *testing.T) {
	// Two points about 1 km apart in Riyadh
	a := common.NewLocation(24.7136, 46.6753)
	b := common.NewLocation(24.7226, 46.6753) // ~1 km north

	dist := common.HaversineDistance(a, b)

	if math.Abs(dist-1.0) > 0.1 {
		t.Fatalf("expected ~1 km, got %f km", dist)
	}
}

func TestHaversineDistance_Symmetric(t *testing.T) {
	a := common.NewLocation(24.7136, 46.6753)
	b := common.NewLocation(25.0, 47.0)

	d1 := common.HaversineDistance(a, b)
	d2 := common.HaversineDistance(b, a)

	if math.Abs(d1-d2) > 1e-10 {
		t.Fatalf("expected symmetric distance, got %f vs %f", d1, d2)
	}
}

func TestHaversineDistance_Antipodal(t *testing.T) {
	// Opposite sides of the earth: approximately 20,015 km (half circumference)
	a := common.NewLocation(0, 0)
	b := common.NewLocation(0, 180)

	dist := common.HaversineDistance(a, b)
	halfCircumference := math.Pi * 6371.0

	if math.Abs(dist-halfCircumference) > 1 {
		t.Fatalf("expected ~%f km, got %f km", halfCircumference, dist)
	}
}

func TestETA_Calculation(t *testing.T) {
	// Simulates the handler's ETA logic: distance / speed
	droneLoc := common.NewLocation(24.7136, 46.6753)
	dest := common.NewLocation(24.75, 46.70)

	distKM := common.HaversineDistance(droneLoc, dest)
	const droneSpeedKMPerMin = 0.5 // 30 km/h as used in handler
	eta := distKM / droneSpeedKMPerMin

	if eta <= 0 {
		t.Fatalf("expected positive ETA, got %f", eta)
	}
	// distance is roughly 4.7 km, so ETA should be about 9.4 minutes
	if math.Abs(eta-9.4) > 2 {
		t.Fatalf("expected ETA ~9.4 min, got %f min", eta)
	}
}

func TestETA_ZeroDistance(t *testing.T) {
	loc := common.NewLocation(24.7136, 46.6753)

	distKM := common.HaversineDistance(loc, loc)
	const droneSpeedKMPerMin = 0.5
	eta := distKM / droneSpeedKMPerMin

	if eta != 0 {
		t.Fatalf("expected 0 ETA for same location, got %f", eta)
	}
}
