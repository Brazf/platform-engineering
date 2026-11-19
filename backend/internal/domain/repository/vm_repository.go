package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
)

type VMRepository interface {
	Create(ctx context.Context, vm *entity.VM) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.VM, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.VM, error)
	// FindWithGPU devolve todas as VMs com gpu_mapping preenchido (balanceamento e pré-checagem de start).
	FindWithGPU(ctx context.Context) ([]entity.VM, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status entity.VMStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
}
