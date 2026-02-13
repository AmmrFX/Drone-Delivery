package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	domainerrors "drone-delivery/internal/errors"
)

type Claims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	secret []byte
	expiry time.Duration
}

func NewService(secret string, expiry time.Duration) *Service {
	return &Service{
		secret: []byte(secret),
		expiry: expiry,
	}
}

func (s *Service) GenerateToken(name, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Sub:  name,
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   name,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiry)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domainerrors.NewUnauthorized("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, domainerrors.NewUnauthorized("invalid or expired token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, domainerrors.NewUnauthorized("invalid token claims")
	}

	return claims, nil
}
