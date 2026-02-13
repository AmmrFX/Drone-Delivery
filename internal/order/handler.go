package order

import (
	"context"
	"net/http"

	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// JobManager is a local interface to avoid importing the job package (circular dep).
type JobManager interface {
	CreateOrderAndJob(ctx context.Context, o *Order) error
	CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error
}

type Handler struct {
	service    Service
	jobService JobManager
}

func NewHandler(service Service, jobService JobManager) *Handler {
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
	o := NewOrder(sub, req.Origin, req.Destination)

	if err := h.jobService.CreateOrderAndJob(c.Request.Context(),o); err != nil {
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

	if err := h.jobService.CancelOrderAndJob(c.Request.Context(), id, sub); err != nil {
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
