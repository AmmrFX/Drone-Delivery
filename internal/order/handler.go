package order

import (
	"context"
	"net/http"

	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DeliveryManager avoids importing the delivery package (circular dep prevention).
type DeliveryManager interface {
	CreateOrderAndJob(ctx context.Context, o *Order) error
	CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error
}

type Handler struct {
	service         Service
	deliveryService DeliveryManager
}

func NewHandler(service Service, deliveryService DeliveryManager) *Handler {
	return &Handler{service: service, deliveryService: deliveryService}
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

	if err := h.deliveryService.CreateOrderAndJob(c.Request.Context(), o); err != nil {
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

	if err := h.deliveryService.CancelOrderAndJob(c.Request.Context(), id, sub); err != nil {
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
