package entity

import (
	"time"

	"github.com/google/uuid"
)

type VMStatus string

const (
	VMStatusCreating VMStatus = "creating"
	VMStatusRunning  VMStatus = "running"
	VMStatusStopped  VMStatus = "stopped"
	VMStatusError    VMStatus = "error"
)

type VM struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"          json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"       json:"user_id"`
	ProxmoxVMID int       `gorm:"not null"                       json:"proxmox_vm_id"`
	Name        string    `gorm:"not null"                       json:"name"`
	CPUCores    int       `gorm:"column:cpu_cores;not null"      json:"cpu_cores"`
	MemoryMB    int       `gorm:"column:memory_mb;not null"      json:"memory_mb"`
	DiskGB      int       `gorm:"column:disk_gb;not null"        json:"disk_gb"`
	OSTemplate  string    `gorm:"not null"                       json:"os_template"`
	Node        string    `gorm:"not null"                       json:"node"`
	Pool        string    `gorm:"default:''"                     json:"pool"`
	Tags        string    `gorm:"default:''"                     json:"tags"` // separado por ";" igual ao Proxmox
	RequireGPU  bool      `gorm:"default:false"                  json:"require_gpu"`
	GPUMapping  string    `gorm:"column:gpu_mapping;type:varchar(64);not null;default:''" json:"gpu_mapping"` // resource mapping da GPU; vazio = sem GPU (ou VM legada)
	Status      VMStatus  `gorm:"type:varchar(20);not null;default:'creating'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
