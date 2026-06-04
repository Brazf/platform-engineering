package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/middleware"
	"github.com/proxlab/backend/internal/usecase"
)

type VMHandler struct {
	vmUC *usecase.VMUseCase
}

func NewVMHandler(vmUC *usecase.VMUseCase) *VMHandler {
	return &VMHandler{vmUC: vmUC}
}

// createVMRequest é o payload esperado pelo frontend para criação de VM.
// pool e tags são opcionais.
type createVMRequest struct {
	Name       string   `json:"name"        binding:"required"`
	CPUCores   int      `json:"cpu_cores"   binding:"required,min=1,max=16"`
	MemoryMB   int      `json:"memory_mb"   binding:"required,min=512"`
	DiskGB     int      `json:"disk_gb"     binding:"required,min=4"`
	OSTemplate string   `json:"os_template" binding:"required"`
	Node       string   `json:"node"`
	Pool       string   `json:"pool"`  // "researchers" ou "students"
	Tags       []string `json:"tags"`  // ex: ["proxlab","aluno"]
}

// Create godoc
// POST /api/vms
// Body: { "name": "minha-vm", "cpu_cores": 2, "memory_mb": 2048, "disk_gb": 20, "os_template": "ubuntu-22.04", "node": "pve" }
// 201: { "id": "uuid", "proxmox_vm_id": 100, "name": "minha-vm", "status": "creating", "websocket_url": "/api/vms/uuid/terminal" }
// 422: { "error": "limite de recursos excedido..." }
func (h *VMHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	role := getUserRole(c)

	var req createVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vm, err := h.vmUC.CreateVM(c.Request.Context(), userID, usecase.CreateVMRequest{
		Name:       req.Name,
		CPUCores:   req.CPUCores,
		MemoryMB:   req.MemoryMB,
		DiskGB:     req.DiskGB,
		OSTemplate: req.OSTemplate,
		Node:       req.Node,
		Pool:       req.Pool,
		Tags:       req.Tags,
		Role:       role,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrLimitExceeded) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao criar VM"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            vm.ID,
		"proxmox_vm_id": vm.ProxmoxVMID,
		"name":          vm.Name,
		"status":        vm.Status,
		"websocket_url": "/api/vms/" + vm.ID.String() + "/terminal",
	})
}

// List godoc
// GET /api/vms
// 200: [{ "id": "uuid", "name": "minha-vm", "status": "running", ... }]
func (h *VMHandler) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	vms, err := h.vmUC.ListUserVMs(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao listar VMs"})
		return
	}

	c.JSON(http.StatusOK, vms)
}

// Get godoc
// GET /api/vms/:id
// 200: { "id": "uuid", "name": "minha-vm", "status": "running", ... }
// 404: { "error": "VM não encontrada" }
func (h *VMHandler) Get(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	vmID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de VM inválido"})
		return
	}

	vm, err := h.vmUC.GetVM(c.Request.Context(), userID, vmID)
	if err != nil {
		handleVMError(c, err)
		return
	}

	c.JSON(http.StatusOK, vm)
}

// Start godoc
// POST /api/vms/:id/start
// 200: { "status": "running" }
func (h *VMHandler) Start(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	vmID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de VM inválido"})
		return
	}

	if err := h.vmUC.StartVM(c.Request.Context(), userID, vmID); err != nil {
		handleVMError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "running"})
}

// Stop godoc
// POST /api/vms/:id/stop
// 200: { "status": "stopped" }
func (h *VMHandler) Stop(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	vmID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de VM inválido"})
		return
	}

	if err := h.vmUC.StopVM(c.Request.Context(), userID, vmID); err != nil {
		handleVMError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}

// Delete godoc
// DELETE /api/vms/:id
// 204: no content
func (h *VMHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	vmID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de VM inválido"})
		return
	}

	if err := h.vmUC.DeleteVM(c.Request.Context(), userID, vmID); err != nil {
		handleVMError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// getUserID extrai e parseia o userID do contexto Gin (injetado pelo middleware Auth).
func getUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, exists := c.Get(middleware.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuário não autenticado"})
		return uuid.UUID{}, false
	}
	idStr, ok := raw.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token com formato inválido"})
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token com formato inválido"})
		return uuid.UUID{}, false
	}
	return id, true
}

// getUserRole extrai o role do contexto Gin (injetado pelo middleware Auth).
func getUserRole(c *gin.Context) string {
	raw, _ := c.Get(middleware.ContextKeyRole)
	role, _ := raw.(string)
	return role
}

func handleVMError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usecase.ErrVMNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrLimitExceeded):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
	}
}
