package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/proxmox"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─── Mock: VMRepository ──────────────────────────────────────────────────────

type mockVMRepo struct{ mock.Mock }

func (m *mockVMRepo) Create(ctx context.Context, vm *entity.VM) error {
	return m.Called(ctx, vm).Error(0)
}

func (m *mockVMRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.VM, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]entity.VM), args.Error(1)
}

func (m *mockVMRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.VM, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.VM), args.Error(1)
}

func (m *mockVMRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.VMStatus) error {
	return m.Called(ctx, id, status).Error(0)
}

func (m *mockVMRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ─── Mock: ProxmoxClientInterface ────────────────────────────────────────────

type mockProxmoxClient struct{ mock.Mock }

func (m *mockProxmoxClient) CreateVM(ctx context.Context, req proxmox.VMCreateRequest) (int, string, error) {
	args := m.Called(ctx, req)
	return args.Int(0), args.String(1), args.Error(2)
}

func (m *mockProxmoxClient) GetVMStatus(ctx context.Context, vmid int) (proxmox.VMStatus, error) {
	args := m.Called(ctx, vmid)
	return args.Get(0).(proxmox.VMStatus), args.Error(1)
}

func (m *mockProxmoxClient) StartVM(ctx context.Context, vmid int) (string, error) {
	args := m.Called(ctx, vmid)
	return args.String(0), args.Error(1)
}

func (m *mockProxmoxClient) StopVM(ctx context.Context, vmid int) (string, error) {
	args := m.Called(ctx, vmid)
	return args.String(0), args.Error(1)
}

func (m *mockProxmoxClient) DeleteVM(ctx context.Context, vmid int) error {
	return m.Called(ctx, vmid).Error(0)
}

func (m *mockProxmoxClient) GetVNCProxy(ctx context.Context, vmid int) (proxmox.VNCProxyResponse, error) {
	args := m.Called(ctx, vmid)
	return args.Get(0).(proxmox.VNCProxyResponse), args.Error(1)
}

func (m *mockProxmoxClient) ListVMs(ctx context.Context) ([]proxmox.VMStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).([]proxmox.VMStatus), args.Error(1)
}

func (m *mockProxmoxClient) GetTermProxy(ctx context.Context, vmid int) (proxmox.TermProxyResponse, error) {
	args := m.Called(ctx, vmid)
	return args.Get(0).(proxmox.TermProxyResponse), args.Error(1)
}

func (m *mockProxmoxClient) GetTaskStatus(ctx context.Context, upid string) (proxmox.TaskStatus, error) {
	args := m.Called(ctx, upid)
	return args.Get(0).(proxmox.TaskStatus), args.Error(1)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newVMUC(vmRepo *mockVMRepo, px *mockProxmoxClient) *usecase.VMUseCase {
	return usecase.NewVMUseCase(vmRepo, px)
}

func baseCreateReq() usecase.CreateVMRequest {
	return usecase.CreateVMRequest{
		Name:       "minha-vm",
		CPUCores:   1,
		MemoryMB:   1024,
		DiskGB:     4,
		OSTemplate: "ubuntu-22.04",
		Node:       "pve01",
		Pool:       "students",
		Role:       "aluno",
	}
}

// ─── Testes: CreateVM — limites de role ──────────────────────────────────────

// Cenário: aluno solicita 8 CPUs (limite é 2) → ErrLimitExceeded, sem chamar Proxmox.
func TestCreateVM_AlunoExceedsCPULimit(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)

	uc := newVMUC(vmRepo, px)
	req := baseCreateReq()
	req.CPUCores = 8

	_, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)
	px.AssertNotCalled(t, "CreateVM")
	vmRepo.AssertNotCalled(t, "Create")
}

// Cenário: aluno solicita 8 GB de RAM (limite é 4096 MB) → ErrLimitExceeded.
func TestCreateVM_AlunoExceedsRAMLimit(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)

	uc := newVMUC(vmRepo, px)
	req := baseCreateReq()
	req.MemoryMB = 8192

	_, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)
	px.AssertNotCalled(t, "CreateVM")
}

// Cenário: pesquisador com 8 CPUs — dentro do limite → chama Proxmox e persiste.
func TestCreateVM_PesquisadorWithinLimit(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)

	px.On("CreateVM", mock.Anything, mock.AnythingOfType("proxmox.VMCreateRequest")).
		Return(101, "UPID:pve:abc", nil)
	vmRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.VM")).Return(nil)

	uc := newVMUC(vmRepo, px)
	req := baseCreateReq()
	req.CPUCores = 8
	req.MemoryMB = 16384
	req.Pool = "researchers"
	req.Role = "pesquisador"

	vm, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.NoError(t, err)
	assert.NotNil(t, vm)
	assert.Equal(t, 101, vm.ProxmoxVMID)
	px.AssertExpectations(t)
	vmRepo.AssertExpectations(t)
}

// Cenário: role desconhecido → aplica limites de aluno por segurança.
func TestCreateVM_UnknownRoleFallsBackToAlunoLimits(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)

	uc := newVMUC(vmRepo, px)
	req := baseCreateReq()
	req.CPUCores = 4
	req.Role = "convidado" // não existe

	_, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)
}

// Cenário: Proxmox retorna erro → CreateVM propaga erro sem salvar no banco.
func TestCreateVM_ProxmoxError(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)

	px.On("CreateVM", mock.Anything, mock.AnythingOfType("proxmox.VMCreateRequest")).
		Return(0, "", errors.New("proxmox: quota exceeded"))

	uc := newVMUC(vmRepo, px)

	_, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.Error(t, err)
	vmRepo.AssertNotCalled(t, "Create")
}

// ─── Testes: GetVM — controle de acesso ──────────────────────────────────────

// Cenário: usuário tenta acessar VM de outro usuário → ErrAccessDenied.
func TestGetVM_AnotherUsersVM(t *testing.T) {
	ownerID := uuid.New()
	attackerID := uuid.New()
	vmID := uuid.New()

	storedVM := &entity.VM{
		ID:          vmID,
		UserID:      ownerID, // pertence ao owner, não ao attacker
		ProxmoxVMID: 200,
		Status:      entity.VMStatusRunning,
	}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)

	uc := newVMUC(vmRepo, px)
	_, err := uc.GetVM(context.Background(), attackerID, vmID)

	assert.ErrorIs(t, err, usecase.ErrAccessDenied)
	// status sync não deve ser chamado antes da checagem de ownership
	px.AssertNotCalled(t, "GetVMStatus")
}

// Cenário: VM não existe no banco → ErrVMNotFound.
func TestGetVM_NotFound(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	vmRepo.On("FindByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil, nil)

	uc := newVMUC(vmRepo, px)
	_, err := uc.GetVM(context.Background(), uuid.New(), uuid.New())

	assert.ErrorIs(t, err, usecase.ErrVMNotFound)
}

// ─── Testes: DeleteVM ─────────────────────────────────────────────────────────

// Cenário: dono da VM chama Delete → chama Proxmox e remove do banco.
func TestDeleteVM_Success(t *testing.T) {
	userID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: userID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)
	px.On("DeleteVM", mock.Anything, 300).Return(nil)
	vmRepo.On("Delete", mock.Anything, vmID).Return(nil)

	uc := newVMUC(vmRepo, px)
	err := uc.DeleteVM(context.Background(), userID, vmID)

	assert.NoError(t, err)
	px.AssertExpectations(t)
	vmRepo.AssertExpectations(t)
}

// Cenário: usuário tenta deletar VM de outro → ErrAccessDenied, sem chamar Proxmox.
func TestDeleteVM_AnotherUsersVM(t *testing.T) {
	ownerID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: ownerID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)

	uc := newVMUC(vmRepo, px)
	err := uc.DeleteVM(context.Background(), uuid.New(), vmID)

	assert.ErrorIs(t, err, usecase.ErrAccessDenied)
	px.AssertNotCalled(t, "DeleteVM")
}
