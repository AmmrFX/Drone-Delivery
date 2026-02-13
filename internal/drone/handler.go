package drone

import (
	"context"
	"net/http"

	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// OrderQuerier avoids importing order package (circular dep prevention).
type OrderQuerier interface {
	GetByDroneID(ctx context.Context, droneID string) (any, error)
	AwaitHandoffWithTx(ctx context.Context, tx sqlx.ExtContext, orderID uuid.UUID) error
}

// JobCreator avoids importing job package.
type JobCreator interface {
	CreateJobWithTx(ctx context.Context, tx sqlx.ExtContext, orderID string) error
}

type Handler struct {
	service     Service
	orderQuery  OrderQuerier
	jobCreator  JobCreator
	db          *sqlx.DB
}

func NewHandler(service Service, orderQuery OrderQuerier, jobCreator JobCreator, db *sqlx.DB) *Handler {
	return &Handler{service: service, orderQuery: orderQuery, jobCreator: jobCreator, db: db}
}

type HeartbeatRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

type HeartbeatResponse struct {
	DroneStatus    Status  `json:"drone_status"`
	CurrentOrderID *string `json:"current_order_id,omitempty"`
}

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

func (h *Handler) GetCurrentOrder(c *gin.Context) {
	droneID := c.GetString("sub")
	o, err := h.orderQuery.GetByDroneID(c.Request.Context(), droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": o})
}

func (h *Handler) ReportBroken(c *gin.Context) {
	droneID := c.GetString("sub")
	ctx := c.Request.Context()

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	defer tx.Rollback()

	// Get drone and mark broken
	d, err := h.service.GetByIDWithTx(ctx, tx, droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	event, err := d.MarkBroken()
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := h.service.UpdateWithTx(ctx, tx, d); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	// If drone had an order, set it to awaiting handoff and create new job
	if event.OrderID != nil {
		if err := h.orderQuery.AwaitHandoffWithTx(ctx, tx, *event.OrderID); err != nil {
			apperrors.ToHTTPError(c, err)
			return
		}
		if err := h.jobCreator.CreateJobWithTx(ctx, tx, event.OrderID.String()); err != nil {
			apperrors.ToHTTPError(c, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "broken report received", "drone_id": droneID})
}
