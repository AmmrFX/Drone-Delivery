package drone

import (
	"context"
	"net/http"

	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
)

// OrderQuerier avoids importing order package (circular dep prevention).
type OrderQuerier interface {
	GetByDroneID(ctx context.Context, droneID string) (any, error)
}

// BrokenHandler avoids importing delivery package (circular dep prevention).
type BrokenHandler interface {
	HandleDroneBroken(ctx context.Context, droneID string) error
}

type Handler struct {
	service       Service
	orderQuery    OrderQuerier
	brokenHandler BrokenHandler
}

func NewHandler(service Service, orderQuery OrderQuerier, brokenHandler BrokenHandler) *Handler {
	return &Handler{service: service, orderQuery: orderQuery, brokenHandler: brokenHandler}
}

// --------------------------------------------------------------
func (h *Handler) Heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

	droneID := c.GetString("sub")
	d, err := h.service.Heartbeat(c.Request.Context(), droneID, req.Latitude, req.Longitude)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	resp := HeartbeatResponse{DroneStatus: d.Status}
	if d.CurrentOrderID != nil {
		s := d.CurrentOrderID.String()
		resp.CurrentOrderID = &s
	}
	c.JSON(http.StatusOK, resp)
}

// --------------------------------------------------------------
func (h *Handler) GetCurrentOrder(c *gin.Context) {
	droneID := c.GetString("sub")
	o, err := h.orderQuery.GetByDroneID(c.Request.Context(), droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": o})
}

// --------------------------------------------------------------
func (h *Handler) ReportBroken(c *gin.Context) {
	droneID := c.GetString("sub")

	if err := h.brokenHandler.HandleDroneBroken(c.Request.Context(), droneID); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "broken report received", "drone_id": droneID})
}
