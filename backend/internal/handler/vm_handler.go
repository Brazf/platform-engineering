package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/proxlab/backend/internal/usecase"
)

type VMHandler struct {
	vmUC *usecase.VMUseCase
}

func NewVMHandler(vmUC *usecase.VMUseCase) *VMHandler {
	return &VMHandler{vmUC: vmUC}
}

// Create godoc
// POST /api/vms
// Body: { "name": "...", "cpu_cores": 2, "memory_mb": 4096, "disk_gb": 64, "os_template": "ubuntu-22.04", "node": "pve" }
// Response 201: { "vm_id": 100, "status": "creating", "websocket_url": "ws://..." }
func (h *VMHandler) Create(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// List godoc
// GET /api/vms
// Response 200: [{ "id": "...", "name": "...", "status": "running", ... }]
func (h *VMHandler) List(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// Get godoc
// GET /api/vms/:id
// Response 200: { "id": "...", "name": "...", "status": "running", ... }
func (h *VMHandler) Get(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// Start godoc
// POST /api/vms/:id/start
// Response 200: { "status": "running" }
func (h *VMHandler) Start(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// Stop godoc
// POST /api/vms/:id/stop
// Response 200: { "status": "stopped" }
func (h *VMHandler) Stop(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// Delete godoc
// DELETE /api/vms/:id
// Response 204: no content
func (h *VMHandler) Delete(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
