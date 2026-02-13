package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"drone-delivery/internal/pkg/apperrors"
)

type rateLimiter interface {
	Allow(ctx context.Context, ip string) (bool, error)
}

func RateLimit(limiter rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		allowed, err := limiter.Allow(c.Request.Context(), ip)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "rate limiter error",
				slog.String("ip", ip),
				slog.String("error", err.Error()),
			)
			// fail open — don't block requests if Redis is down
			c.Next()
			return
		}

		if !allowed {
			slog.WarnContext(c.Request.Context(), "rate limit exceeded",
				slog.String("ip", ip),
			)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, apperrors.ErrorResponse{
				Error: apperrors.ErrorBody{
					Code:    "RATE_LIMITED",
					Message: "too many requests, please try again later",
				},
			})
			return
		}

		c.Next()
	}
}
