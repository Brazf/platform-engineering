package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	domainrepo "github.com/proxlab/backend/internal/domain/repository"
	"gorm.io/gorm"
)

type containerRepository struct{ db *gorm.DB }

// NewContainerRepository retorna uma implementação GORM de ContainerRepository.
func NewContainerRepository(db *gorm.DB) domainrepo.ContainerRepository {
	return &containerRepository{db: db}
}

func (r *containerRepository) Create(ctx context.Context, c *entity.Container) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *containerRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Container, error) {
	var out []entity.Container
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *containerRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Container, error) {
	var c entity.Container
	err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

func (r *containerRepository) FindByUserIDAndName(ctx context.Context, userID uuid.UUID, name string) (*entity.Container, error) {
	var c entity.Container
	err := r.db.WithContext(ctx).First(&c, "user_id = ? AND name = ?", userID, name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

func (r *containerRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.Container{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *containerRepository) SumPersistentGBByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var sum int
	err := r.db.WithContext(ctx).Model(&entity.Container{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(persistent_gb), 0)").Scan(&sum).Error
	return sum, err
}

func (r *containerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Container{}, "id = ?", id).Error
}
