package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitsFor(t *testing.T) {
	assert.Equal(t, RoleLimits{MaxCPU: 2, MaxMemMB: 4096, MaxContainers: 5, MaxStorageGB: 20}, LimitsFor("aluno"))
	assert.Equal(t, RoleLimits{MaxCPU: 8, MaxMemMB: 16384, MaxContainers: 20, MaxStorageGB: 100}, LimitsFor("pesquisador"))
	assert.Equal(t, LimitsFor("aluno"), LimitsFor("desconhecido"), "role desconhecida cai em aluno")
}

func TestValidateRoleLimits(t *testing.T) {
	assert.NoError(t, validateRoleLimits("aluno", 2, 4096))
	assert.ErrorIs(t, validateRoleLimits("aluno", 3, 1024), ErrLimitExceeded)
	assert.ErrorIs(t, validateRoleLimits("aluno", 1, 8192), ErrLimitExceeded)
}
