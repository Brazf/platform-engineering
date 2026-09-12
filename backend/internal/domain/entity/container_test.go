package entity_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespaceFor(t *testing.T) {
	id := uuid.MustParse("0f8fad5b-d9cb-469f-a165-70867728950e")
	assert.Equal(t, "u-0f8fad5b", entity.NamespaceFor(id))
}

func TestContainerNameRegexp(t *testing.T) {
	for _, ok := range []string{"a", "meu-pg", "app1", "x-y-z"} {
		assert.True(t, entity.ContainerNameRegexp.MatchString(ok), ok)
	}
	for _, bad := range []string{"", "-a", "a-", "Meu", "a_b", "a b", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		assert.False(t, entity.ContainerNameRegexp.MatchString(bad), bad)
	}
}

func TestEnvMapScanValue(t *testing.T) {
	m := entity.EnvMap{"A": "1"}
	v, err := m.Value()
	require.NoError(t, err)
	var back entity.EnvMap
	require.NoError(t, back.Scan(v))
	assert.Equal(t, m, back)
	require.NoError(t, back.Scan(nil))
	assert.Empty(t, back)
}
