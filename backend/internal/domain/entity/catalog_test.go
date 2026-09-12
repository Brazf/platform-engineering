package entity_test

import (
	"testing"

	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestCatalog(t *testing.T) {
	pg, ok := entity.CatalogByKey("postgres")
	assert.True(t, ok)
	assert.Equal(t, "postgres:16", pg.Image)
	assert.Equal(t, 5432, pg.Port)
	assert.Equal(t, "tcp", pg.Protocol)
	assert.Equal(t, "/var/lib/postgresql/data", pg.MountPath)
	assert.Equal(t, []string{"POSTGRES_PASSWORD"}, pg.RequiredEnv)

	ub, ok := entity.CatalogByKey("ubuntu")
	assert.True(t, ok)
	assert.Equal(t, 0, ub.Port)
	assert.Equal(t, "sleep infinity", ub.Command)

	_, ok = entity.CatalogByKey("nao-existe")
	assert.False(t, ok)
	assert.Len(t, entity.Catalog(), 6)
}
