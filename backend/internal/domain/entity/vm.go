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
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	ProxmoxVMID int       `gorm:"not null"`
	Name        string    `gorm:"not null"`
	CPUCores    int       `gorm:"column:cpu_cores;not null"`
	MemoryMB    int       `gorm:"column:memory_mb;not null"`
	DiskGB      int       `gorm:"column:disk_gb;not null"`
	OSTemplate  string    `gorm:"not null"`
	Node        string    `gorm:"not null"`
	Status      VMStatus  `gorm:"type:varchar(20);not null;default:'creating'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
