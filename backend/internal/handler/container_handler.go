package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/usecase"
)

type ContainerHandler struct{ uc *usecase.ContainerUseCase }

func NewContainerHandler(uc *usecase.ContainerUseCase) *ContainerHandler {
	return &ContainerHandler{uc: uc}
}

type createContainerRequest struct {
	Name         string            `json:"name"          binding:"required"`
	Description  string            `json:"description"`
	Image        string            `json:"image"         binding:"required"`
	CPUMillis    int               `json:"cpu_millis"    binding:"required,min=100"`
	MemoryMB     int               `json:"memory_mb"     binding:"required,min=64"`
	Port         int               `json:"port"          binding:"min=0,max=65535"`
	Protocol     string            `json:"protocol"      binding:"omitempty,oneof=http tcp"`
	Env          map[string]string `json:"env"`
	Command      string            `json:"command"`
	PersistentGB int               `json:"persistent_gb" binding:"min=0"`
	MountPath    string            `json:"mount_path"`
}

// Catalog — GET /api/containers/catalog → 200 [CatalogImage]
func (h *ContainerHandler) Catalog(c *gin.Context) {
	c.JSON(http.StatusOK, entity.Catalog())
}

// Create — POST /api/containers → 201 ContainerView
func (h *ContainerHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req createContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Protocol == "" {
		req.Protocol = entity.ProtocolHTTP
	}
	view, err := h.uc.Create(c.Request.Context(), userID, usecase.CreateContainerRequest{
		Name: req.Name, Description: req.Description, Image: req.Image,
		CPUMillis: req.CPUMillis, MemoryMB: req.MemoryMB, Port: req.Port, Protocol: req.Protocol,
		Env: req.Env, Command: req.Command, PersistentGB: req.PersistentGB, MountPath: req.MountPath,
		Role: getUserRole(c),
	})
	if err != nil {
		handleContainerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

// List — GET /api/containers → 200 [ContainerView]
func (h *ContainerHandler) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	views, err := h.uc.List(c.Request.Context(), userID)
	if err != nil {
		handleContainerError(c, err)
		return
	}
	c.JSON(http.StatusOK, views)
}

// Get — GET /api/containers/:id → 200 ContainerView | 404
func (h *ContainerHandler) Get(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de container inválido"})
		return
	}
	view, err := h.uc.Get(c.Request.Context(), userID, id)
	if err != nil {
		handleContainerError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// Delete — DELETE /api/containers/:id → 204 | 404 | 503
func (h *ContainerHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de container inválido"})
		return
	}
	if err := h.uc.Delete(c.Request.Context(), userID, id); err != nil {
		handleContainerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func handleContainerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usecase.ErrContainerNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrContainerNameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrLimitExceeded), errors.Is(err, usecase.ErrInvalidContainer):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrRuntimeUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
	}
}
