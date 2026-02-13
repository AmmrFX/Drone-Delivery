package apperrors

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	domainerrors "drone-delivery/internal/errors"
)

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var codeToStatus = map[string]int{
	domainerrors.ErrNotFound:          http.StatusNotFound,
	domainerrors.ErrInvalidTransition: http.StatusConflict,
	domainerrors.ErrUnauthorized:      http.StatusUnauthorized,
	domainerrors.ErrForbidden:         http.StatusForbidden,
	domainerrors.ErrConflict:          http.StatusConflict,
	domainerrors.ErrValidation:        http.StatusBadRequest,
	domainerrors.ErrOutOfZone:         http.StatusBadRequest,
	domainerrors.ErrInternal:          http.StatusInternalServerError,
}

func ToHTTPError(c *gin.Context, err error) {
	var domainErr *domainerrors.DomainError
	if errors.As(err, &domainErr) {
		status, ok := codeToStatus[domainErr.Code]
		if !ok {
			status = http.StatusInternalServerError
		}
		c.JSON(status, ErrorResponse{
			Error: ErrorBody{
				Code:    domainErr.Code,
				Message: domainErr.Message,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error: ErrorBody{
			Code:    domainerrors.ErrInternal,
			Message: "an unexpected error occurred",
		},
	})
}
