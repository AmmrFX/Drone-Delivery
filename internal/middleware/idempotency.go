package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"drone-delivery/internal/pkg/apperrors"
)

type idempotencyStore interface {
	Check(ctx context.Context, userID, key string) ([]byte, bool, error)
	Set(ctx context.Context, userID, key string, response []byte) error
}

// responseRecorder captures the response body so we can store it.
type responseRecorder struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func Idempotency(store idempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}

		userID := c.GetString("sub")
		ctx := c.Request.Context()

		// Check for a cached response.
		cached, found, err := store.Check(ctx, userID, key)
		if err != nil {
			slog.ErrorContext(ctx, "idempotency check failed",
				slog.String("error", err.Error()),
			)
			// fail open
			c.Next()
			return
		}

		if found {
			c.Data(http.StatusOK, "application/json", cached)
			c.Abort()
			return
		}

		// Record the response body.
		rec := &responseRecorder{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = rec

		c.Next()

		// Only cache successful responses.
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			if err := store.Set(ctx, userID, key, rec.body.Bytes()); err != nil {
				slog.ErrorContext(ctx, "idempotency store failed",
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

// RequireIdempotencyKey rejects requests without an Idempotency-Key header.
// Use on mutation endpoints where idempotency is mandatory.
func RequireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}

		if c.GetHeader("Idempotency-Key") == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, apperrors.ErrorResponse{
				Error: apperrors.ErrorBody{
					Code:    "VALIDATION",
					Message: "Idempotency-Key header is required",
				},
			})
			return
		}

		c.Next()
	}
}
