package admin

import (
	"net/http"
	"strconv"

	"drone-delivery/internal/common"
	"drone-delivery/internal/drone"
	"drone-delivery/internal/order"
	"drone-delivery/internal/pkg/apperrors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	adminService Service
	orderService order.Service
	droneService drone.Service
}

func NewHandler(adminService Service, orderService order.Service, droneService drone.Service) *Handler {
	return &Handler{adminService: adminService, orderService: orderService, droneService: droneService}
}

func (h *Handler) ListOrders(c *gin.Context) {
	page, limit := parsePagination(c)

	var statusPtr *order.Status
	if s := c.Query("status"); s != "" {
		st := order.Status(s)
		statusPtr = &st
	}

	orders, total, err := h.adminService.ListOrders(c.Request.Context(), statusPtr, page, limit)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"orders": orders, "total": total, "page": page, "limit": limit})
}

func (h *Handler) UpdateOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": "invalid order id"}})
		return
	}

	var req struct {
		Origin      *common.Location `json:"origin"`
		Destination *common.Location `json:"destination"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

	o, err := h.adminService.UpdateOrder(c.Request.Context(), id, req.Origin, req.Destination)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"order": o})
}

func (h *Handler) ListDrones(c *gin.Context) {
	page, limit := parsePagination(c)

	var statusPtr *drone.Status
	if s := c.Query("status"); s != "" {
		st := drone.Status(s)
		statusPtr = &st
	}

	drones, total, err := h.adminService.ListDrones(c.Request.Context(), statusPtr, page, limit)
	if err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"drones": drones, "total": total, "page": page, "limit": limit})
}

func (h *Handler) UpdateDroneStatus(c *gin.Context) {
	droneID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION", "message": err.Error()}})
		return
	}

	if err := h.adminService.UpdateDroneStatus(c.Request.Context(), droneID, req.Status); err != nil {
		apperrors.ToHTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "drone status updated", "status": req.Status})
}

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	limit := 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return page, limit
}
