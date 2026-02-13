package auth

import "drone-delivery/internal/jwt"

type Service interface {
	GenerateToken(name, role string) (string, error)
}

type authService struct {
	jwt *jwt.Service
}

func NewAuthService(jwt *jwt.Service) Service {
	return &authService{jwt: jwt}
}

func (s *authService) GenerateToken(name, role string) (string, error) {
	return s.jwt.GenerateToken(name, role)
}
