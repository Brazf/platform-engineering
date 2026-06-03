package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	domainrepo "github.com/proxlab/backend/internal/domain/repository"
	"gorm.io/gorm"
)

type vmRepository struct {
	db *gorm.DB
}

// NewVMRepository retorna uma implementação GORM de VMRepository.
func NewVMRepository(db *gorm.DB) domainrepo.VMRepository {
	return &vmRepository{db: db}
}

func (r *vmRepository) Create(ctx context.Context, vm *entity.VM) error {
	return r.db.WithContext(ctx).Create(vm).Error
}

func (r *vmRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.VM, error) {
	var vm entity.VM
	err := r.db.WithContext(ctx).First(&vm, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &vm, err
}

func (r *vmRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.VM, error) {
	var vms []entity.VM
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&vms).Error
	return vms, err
}

func (r *vmRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.VMStatus) error {
	return r.db.WithContext(ctx).
		Model(&entity.VM{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *vmRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.VM{}, "id = ?", id).Error
}
