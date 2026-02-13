package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"drone-delivery/internal/pkg/apperrors"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()

				slog.ErrorContext(c.Request.Context(), "panic recovered",
					slog.Any("error", r),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.String("ip", c.ClientIP()),
					slog.String("stack", string(stack)),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, apperrors.ErrorResponse{
					Error: apperrors.ErrorBody{
						Code:    "INTERNAL",
						Message: "an unexpected error occurred",
					},
				})
			}
		}()

		c.Next()
	}
}
