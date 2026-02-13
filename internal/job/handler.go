package job

import (
	"net/http"

	"drone-delivery/internal/drone"
	"drone-delivery/internal/order"
	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Handler struct {
	service      Service
	orderService order.Service
	droneService drone.Service
	db           *sqlx.DB
}

func NewHandler(service Service, orderService order.Service, droneService drone.Service, db *sqlx.DB) *Handler {
	return &Handler{service: service, orderService: orderService, droneService: droneService, db: db}
}

func (h *Handler) ListOpenJobs(c *gin.Context) {
	jobs, err := h.service.ListOpenJobs(c.Request.Context())
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func (h *Handler) ReserveJob(c *gin.Context) {
	var req struct {
		JobID string `json:"job_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

	droneID := c.GetString("sub")
	ctx := c.Request.Context()

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	defer tx.Rollback()

	// Reserve the job
	j, err := h.service.ReserveJob(ctx, req.JobID, droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	// Assign order to drone
	orderID, err := uuid.Parse(j.OrderID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	o, err := h.orderService.GetByIDWithTx(ctx, tx, orderID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := o.Assign(droneID); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := h.orderService.SaveWithTx(ctx, tx, o); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	// Reserve drone
	d, err := h.droneService.GetByIDWithTx(ctx, tx, droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := d.Reserve(orderID); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := h.droneService.UpdateWithTx(ctx, tx, d); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	if err := tx.Commit(); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"job": j, "message": "job reserved"})
}

func (h *Handler) GrabOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid order id"}})
		return
	}

	droneID := c.GetString("sub")
	ctx := c.Request.Context()

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	defer tx.Rollback()

	// Mark order picked up
	o, err := h.orderService.GetByIDWithTx(ctx, tx, orderID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if o.AssignedDroneID == nil || *o.AssignedDroneID != droneID {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "drone is not assigned to this order"}})
		return
	}
	if err := o.MarkPickedUp(); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := h.orderService.SaveWithTx(ctx, tx, o); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	// Drone starts delivery
	d, err := h.droneService.GetByIDWithTx(ctx, tx, droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := d.StartDelivery(); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := h.droneService.UpdateWithTx(ctx, tx, d); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	if err := tx.Commit(); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order picked up"})
}

func (h *Handler) CompleteDelivery(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid order id"}})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}
	if req.Status != "delivered" && req.Status != "failed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "status must be 'delivered' or 'failed'"}})
		return
	}

	droneID := c.GetString("sub")
	ctx := c.Request.Context()

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	defer tx.Rollback()

	// Update order status
	o, err := h.orderService.GetByIDWithTx(ctx, tx, orderID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if o.AssignedDroneID == nil || *o.AssignedDroneID != droneID {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "drone is not assigned to this order"}})
		return
	}

	if req.Status == "delivered" {
		if err := o.MarkDelivered(); err != nil {
			apperrors.ToHTTPError(c, err)
			return
		}
	} else {
		if err := o.MarkFailed(); err != nil {
			apperrors.ToHTTPError(c, err)
			return
		}
	}
	if err := h.orderService.SaveWithTx(ctx, tx, o); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	// Drone goes idle
	d, err := h.droneService.GetByIDWithTx(ctx, tx, droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	d.GoIdle()
	if err := h.droneService.UpdateWithTx(ctx, tx, d); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	// Complete the job
	j, err := h.service.GetByOrderID(ctx, orderID.String())
	if err == nil && j != nil {
		_ = h.service.CompleteJob(ctx, j.ID)
	}

	if err := tx.Commit(); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "delivery completed", "status": req.Status})
}
