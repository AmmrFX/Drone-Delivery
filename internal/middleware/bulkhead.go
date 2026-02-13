package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"drone-delivery/internal/pkg/apperrors"
)

func Bulkhead(maxConcurrent int) gin.HandlerFunc {
	sem := make(chan struct{}, maxConcurrent)

	return func(c *gin.Context) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			c.Next()
		default:
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, apperrors.ErrorResponse{
				Error: apperrors.ErrorBody{
					Code:    "SERVICE_UNAVAILABLE",
					Message: "server is at capacity, please try again later",
				},
			})
		}
	}
}
