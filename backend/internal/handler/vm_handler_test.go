package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
)

func respond(fn func(*gin.Context, error), err error) (int, string) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	fn(c, err)
	return w.Code, w.Body.String()
}

func TestHandleVMError_GPUAndStart(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		body string
	}{
		{"gpu em uso", fmt.Errorf("x: %w", usecase.ErrGPUInUse), 409,
			"A GPU desta máquina está em uso por outra máquina virtual. Tente ligar novamente quando ela for desligada."},
		{"start falhou", usecase.ErrVMStartFailed, 500, "Não foi possível ligar a máquina virtual."},
		{"genérico", errors.New("proxmox 500: segredo"), 500, "erro interno do servidor"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, body := respond(handleVMError, tc.err)
			assert.Equal(t, tc.code, code)
			assert.Contains(t, body, tc.body)
			assert.NotContains(t, body, "segredo")
		})
	}
}

func TestCreateVMError_GPU(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		body string
	}{
		{"sem mapping", usecase.ErrGPUUnavailable, http.StatusUnprocessableEntity, "GPU não está disponível nesta instalação."},
		{"sem Mapping.Use", fmt.Errorf("terraform: %w: 403 Mapping.Use", repository.ErrGPUNotPermitted), http.StatusServiceUnavailable,
			"GPU temporariamente indisponível. Contate o administrador."},
		{"limite", usecase.ErrLimitExceeded, http.StatusUnprocessableEntity, "limite de recursos"},
		{"genérico", errors.New("terraform: segredo"), 500, "erro ao criar VM"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, body := respond(createVMError, tc.err)
			assert.Equal(t, tc.code, code)
			assert.Contains(t, body, tc.body)
			assert.NotContains(t, body, "Mapping.Use")
			assert.NotContains(t, body, "segredo")
		})
	}
}
