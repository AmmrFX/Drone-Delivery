package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"drone-delivery/internal/jwt"
	"drone-delivery/internal/pkg/apperrors"
)

var skipAuth = map[string]bool{
	"/auth/token": true,
	"/health":     true,
}

func Auth(jwtService *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if skipAuth[c.Request.URL.Path] {
			c.Next()
			return
		}

		header := c.GetHeader("Authorization")
		if header == "" {
			unauthorized(c, "missing authorization header")
			return
		}

		token, found := strings.CutPrefix(header, "Bearer ")
		if !found {
			unauthorized(c, "invalid authorization format")
			return
		}

		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			slog.WarnContext(c.Request.Context(), "auth failed",
				slog.String("path", c.Request.URL.Path),
				slog.String("ip", c.ClientIP()),
				slog.String("error", err.Error()),
			)
			unauthorized(c, "invalid or expired token")
			return
		}

		c.Set("sub", claims.Sub)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func unauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, apperrors.ErrorResponse{
		Error: apperrors.ErrorBody{
			Code:    "UNAUTHORIZED",
			Message: msg,
		},
	})
}
