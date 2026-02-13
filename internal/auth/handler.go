package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	authService Service
}

func NewHandler(authService Service) *Handler {
	return &Handler{authService: authService}
}

func (h *Handler) GenerateToken(c *gin.Context) {
	name := c.PostForm("name")
	role := c.PostForm("role")
	if name == "" || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and role are required"})
		return
	}

	token, err := h.authService.GenerateToken(name, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}
