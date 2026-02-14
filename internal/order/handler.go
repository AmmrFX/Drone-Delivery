package order

import (
	"context"
	"net/http"

	"drone-delivery/internal/common"
	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DeliveryManager avoids importing the delivery package (circular dep prevention).
type DeliveryManager interface {
	CreateOrderAndJob(ctx context.Context, o *Order) error
	CancelOrderAndJob(ctx context.Context, orderID uuid.UUID, submittedBy string) error
}

// DroneLocator avoids importing the drone package (circular dep prevention).
type DroneLocator interface {
	GetDroneLocation(ctx context.Context, droneID string) (*common.Location, error)
}

type Handler struct {
	service         Service
	deliveryService DeliveryManager
	droneLocator    DroneLocator
}

func NewHandler(service Service, deliveryService DeliveryManager, droneLocator DroneLocator) *Handler {
	return &Handler{service: service, deliveryService: deliveryService, droneLocator: droneLocator}
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

	if err := h.service.ValidateLocation(req.Origin, "origin"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}
	if err := h.service.ValidateLocation(req.Destination, "destination"); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

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
	ctx := c.Request.Context()
	o, err := h.service.GetOrderDetails(ctx, id, sub)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	resp := OrderDetailResponse{Order: o}

	if o.AssignedDroneID != nil {
		loc, err := h.droneLocator.GetDroneLocation(ctx, *o.AssignedDroneID)
		if err == nil && loc != nil {
			resp.DroneLocation = loc
			dest := o.Destination()
			distKM := common.HaversineDistance(*loc, dest)
			const droneSpeedKMPerMin = 0.5 // ~30 km/h
			eta := distKM / droneSpeedKMPerMin
			resp.ETAMinutes = &eta
		}
	}

	c.JSON(http.StatusOK, resp)
}

// -------------------------------------------------------------------------------------------------
func (h *Handler) ListMyOrders(c *gin.Context) {
	sub := c.GetString("sub")
	orders, err := h.service.ListMyOrders(c.Request.Context(), sub)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}
