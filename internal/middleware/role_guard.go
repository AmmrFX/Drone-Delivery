package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"drone-delivery/internal/pkg/apperrors"
)

func RoleGuard(required string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != required {
			c.AbortWithStatusJSON(http.StatusForbidden, apperrors.ErrorResponse{
				Error: apperrors.ErrorBody{
					Code:    "FORBIDDEN",
					Message: "insufficient permissions",
				},
			})
			return
		}

		c.Next()
	}
}
