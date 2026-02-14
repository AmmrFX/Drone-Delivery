package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server         ServerConfig
	JWT            JWTConfig
	Postgres       PostgresConfig
	Redis          RedisConfig
	RateLimiter    RateLimiterConfig
	CircuitBreaker CircuitBreakerConfig
	Bulkhead       BulkheadConfig
	Zone           ZoneConfig
	Drone          DroneConfig
	Mapbox         MapboxConfig
}

type ServerConfig struct {
	Port            int
	ShutdownTimeout time.Duration
}

type JWTConfig struct {
	Secret      string
	ExpiryHours time.Duration
}

type PostgresConfig struct {
	URL      string // DATABASE_URL takes precedence if set
	Host     string
	Port     int
	User     string
	Password string
	DB       string
	SSLMode  string
}

type RedisConfig struct {
	URL      string // REDIS_URL takes precedence if set
	Host     string
	Port     int
	Password string
	DB       int
}

type RateLimiterConfig struct {
	MaxRequests   int
	WindowSeconds int
}

type CircuitBreakerConfig struct {
	FailureThreshold int
	CooldownSeconds  int
}

type BulkheadConfig struct {
	HeartbeatPool int
	MutationPool  int
	AdminPool     int
}

type ZoneConfig struct {
	CenterLat float64
	CenterLng float64
	RadiusKM  float64
}

type DroneConfig struct {
	SpeedKMH            float64
	LocationCacheTTLSec int
	IdempotencyTTLSec   int
}

type MapboxConfig struct {
	BaseURL     string
	AccessToken string
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

func getenvFloat(key string, fallback float64) float64 {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	return v
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Port:            getenvInt("PORT", getenvInt("SERVER_PORT", 8080)),
			ShutdownTimeout: time.Duration(getenvInt("SHUTDOWN_TIMEOUT_SECONDS", 5)) * time.Second,
		},
		JWT: JWTConfig{
			Secret:      getenv("JWT_SECRET", "default-secret-change-me"),
			ExpiryHours: time.Duration(getenvInt("JWT_EXPIRY_HOURS", 24)) * time.Hour,
		},
		Postgres: PostgresConfig{
			URL:      getenv("DATABASE_URL", ""),
			Host:     getenv("POSTGRES_HOST", "localhost"),
			Port:     getenvInt("POSTGRES_PORT", 5432),
			User:     getenv("POSTGRES_USER", "drone_admin"),
			Password: getenv("POSTGRES_PASSWORD", "secure_password"),
			DB:       getenv("POSTGRES_DB", "drone_delivery"),
			SSLMode:  getenv("POSTGRES_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			URL:      getenv("REDIS_URL", ""),
			Host:     getenv("REDIS_HOST", "localhost"),
			Port:     getenvInt("REDIS_PORT", 6379),
			Password: getenv("REDIS_PASSWORD", ""),
			DB:       getenvInt("REDIS_DB", 0),
		},
		RateLimiter: RateLimiterConfig{
			MaxRequests:   getenvInt("RATE_LIMIT_MAX_REQUESTS", 100),
			WindowSeconds: getenvInt("RATE_LIMIT_WINDOW_SECONDS", 60),
		},
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: getenvInt("CB_FAILURE_THRESHOLD", 5),
			CooldownSeconds:  getenvInt("CB_COOLDOWN_SECONDS", 30),
		},
		Bulkhead: BulkheadConfig{
			HeartbeatPool: getenvInt("BULKHEAD_HEARTBEAT_POOL", 100),
			MutationPool:  getenvInt("BULKHEAD_MUTATION_POOL", 50),
			AdminPool:     getenvInt("BULKHEAD_ADMIN_POOL", 20),
		},
		Zone: ZoneConfig{
			CenterLat: getenvFloat("ZONE_CENTER_LAT", 24.7136),
			CenterLng: getenvFloat("ZONE_CENTER_LNG", 46.6753),
			RadiusKM:  getenvFloat("ZONE_RADIUS_KM", 50),
		},
		Drone: DroneConfig{
			SpeedKMH:            getenvFloat("DRONE_SPEED_KMH", 50),
			LocationCacheTTLSec: getenvInt("DRONE_LOCATION_CACHE_TTL_SECONDS", 60),
			IdempotencyTTLSec:   getenvInt("IDEMPOTENCY_TTL_SECONDS", 300),
		},
		Mapbox: MapboxConfig{
			BaseURL:     getenv("MAPBOX_BASE_URL", "https://api.mapbox.com"),
			AccessToken: getenv("MAPBOX_ACCESS_TOKEN", ""),
		},
	}

	return cfg, nil
}

func (p PostgresConfig) DSN() string {
	if p.URL != "" {
		return p.URL
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DB, p.SSLMode)
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}
