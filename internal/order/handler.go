package order

import (
	"context"
	"net/http"

	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// JobCreator is a local interface to avoid importing the job package (circular dep).
type JobCreator interface {
	CreateJob(ctx context.Context, orderID string) error
}

type Handler struct {
	service    Service
	jobService JobCreator
}

func NewHandler(service Service, jobService JobCreator) *Handler {
	return &Handler{service: service, jobService: jobService}
}

// -------------------------------------------------------------------------------------------------
func (h *Handler) PlaceOrder(c *gin.Context) {
	var req PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

	sub := c.GetString("sub")
	o, err := h.service.PlaceOrder(c.Request.Context(), sub, req.Origin, req.Destination)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	if err := h.jobService.CreateJob(c.Request.Context(), o.ID.String()); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusCreated, OrderResponse{Order: o})
}

// -------------------------------------------------------------------------------------------------
func (h *Handler) WithdrawOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid order id"}})
		return
	}

	sub := c.GetString("sub")
	if err := h.service.WithdrawOrder(c.Request.Context(), id, sub); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order withdrawn"})
}

// -------------------------------------------------------------------------------------------------
func (h *Handler) GetOrderDetails(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid order id"}})
		return
	}

	sub := c.GetString("sub")
	o, err := h.service.GetOrderDetails(c.Request.Context(), id, sub)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, OrderDetailResponse{Order: o})
}

// -------------------------------------------------------------------------------------------------
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/orders", h.PlaceOrder)
	rg.DELETE("/orders/:id", h.WithdrawOrder)
	rg.GET("/orders/:id", h.GetOrderDetails)
}
