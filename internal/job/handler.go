package job

import (
	"context"
	"net/http"

	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service         Service
	deliveryManager DeliveryManager
}
type DeliveryManager interface {
	ReserveJobAndAssign(ctx context.Context, jobID, droneID string) (*Job, error)
	GrabOrder(ctx context.Context, orderID uuid.UUID, droneID string) error
	CompleteDelivery(ctx context.Context, orderID uuid.UUID, droneID string, success bool) error
}

func NewHandler(service Service, deliveryManager DeliveryManager) *Handler {
	return &Handler{service: service, deliveryManager: deliveryManager}
}

// --------------------------------------------------------------
func (h *Handler) ListOpenJobs(c *gin.Context) {
	jobs, err := h.service.ListOpenJobs(c.Request.Context())
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

// --------------------------------------------------------------
func (h *Handler) ReserveJob(c *gin.Context) {
	var req struct {
		JobID string `json:"job_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

	droneID := c.GetString("sub")

	j, err := h.deliveryManager.ReserveJobAndAssign(c.Request.Context(), req.JobID, droneID)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"job": j, "message": "job reserved"})
}

// --------------------------------------------------------------
func (h *Handler) GrabOrder(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid order id"}})
		return
	}

	droneID := c.GetString("sub")

	if err := h.deliveryManager.GrabOrder(c.Request.Context(), orderID, droneID); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order picked up"})
}

// --------------------------------------------------------------
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

	if err := h.deliveryManager.CompleteDelivery(c.Request.Context(), orderID, droneID, req.Status == "delivered"); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "delivery completed", "status": req.Status})
}
