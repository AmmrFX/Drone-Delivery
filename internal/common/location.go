package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"
)

const earthRadiusKM = 6371.0

var (
	ErrOutOfZone      = errors.New("location is outside the allowed delivery zone")
	ErrInvalidLatLng  = errors.New("invalid latitude or longitude")
	ErrMapboxNoRoutes = errors.New("mapbox returned no routes")
	ErrMapboxRequest  = errors.New("mapbox request failed")
)

type Location struct {
	Lat float64 `json:"lat" db:"lat"`
	Lng float64 `json:"lng" db:"lng"`
}

func NewLocation(lat, lng float64) Location {
	return Location{Lat: lat, Lng: lng}
}

type MapboxDirectionsResponse struct {
	Routes []struct {
		Distance float64 `json:"distance"` // meters
		Duration float64 `json:"duration"` // seconds
	} `json:"routes"`
	Code string `json:"code"`
}

type MapboxClient struct {
	BaseURL     string
	AccessToken string
	HTTPClient  *http.Client
}

func NewMapboxClient(baseURL, accessToken string) *MapboxClient {
	return &MapboxClient{
		BaseURL:     baseURL,
		AccessToken: accessToken,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *MapboxClient) GetRouteDistanceAndDuration(ctx context.Context, from, to Location) (distanceKM float64, durationMin float64, err error) {
	url := fmt.Sprintf(
		"%s/directions/v5/mapbox/driving/%f,%f;%f,%f?access_token=%s&overview=false",
		m.BaseURL, from.Lng, from.Lat, to.Lng, to.Lat, m.AccessToken,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrMapboxRequest, err)
	}

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrMapboxRequest, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("%w: status %d", ErrMapboxRequest, resp.StatusCode)
	}

	var result MapboxDirectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrMapboxRequest, err)
	}

	if result.Code != "Ok" || len(result.Routes) == 0 {
		return 0, 0, fmt.Errorf("%w (code: %s)", ErrMapboxNoRoutes, result.Code)
	}

	route := result.Routes[0]
	return route.Distance / 1000.0, route.Duration / 60.0, nil
}

func HaversineDistance(a, b Location) float64 {
	dLat := degreesToRadians(b.Lat - a.Lat)
	dLng := degreesToRadians(b.Lng - a.Lng)

	aLat := degreesToRadians(a.Lat)
	bLat := degreesToRadians(b.Lat)

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(aLat)*math.Cos(bLat)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))

	return earthRadiusKM * c
}

func degreesToRadians(d float64) float64 {
	return d * math.Pi / 180
}

func ValidateInZone(loc Location, center Location, radiusKM float64) error {
	dist := HaversineDistance(loc, center)
	if dist > radiusKM {
		return ErrOutOfZone
	}
	return nil
}

func ValidateLatLng(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return fmt.Errorf("%w: latitude must be between -90 and 90", ErrInvalidLatLng)
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("%w: longitude must be between -180 and 180", ErrInvalidLatLng)
	}
	return nil
}
