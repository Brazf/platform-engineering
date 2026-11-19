package terraform

import (
	"errors"
	"testing"

	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/stretchr/testify/assert"
)

func TestApplyError_MappingUseBecomesErrGPUNotPermitted(t *testing.T) {
	err := applyError(errors.New("403 Permission check failed (/mapping/pci/gpu-rtx4060-01, Mapping.Use)"))
	assert.ErrorIs(t, err, repository.ErrGPUNotPermitted)
}

func TestApplyError_OtherErrorsKeepGenericWrap(t *testing.T) {
	err := applyError(errors.New("boom"))
	assert.NotErrorIs(t, err, repository.ErrGPUNotPermitted)
	assert.ErrorContains(t, err, "terraform apply falhou")
}
