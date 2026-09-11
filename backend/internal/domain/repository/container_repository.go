package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
)

type ContainerRepository interface {
	Create(ctx context.Context, c *entity.Container) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Container, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Container, error)
	FindByUserIDAndName(ctx context.Context, userID uuid.UUID, name string) (*entity.Container, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	SumPersistentGBByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
