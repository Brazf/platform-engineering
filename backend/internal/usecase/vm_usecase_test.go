package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
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

func (m *mockProxmoxClient) GetTermProxy(ctx context.Context, vmid int) (proxmox.TermProxyResponse, error) {
	args := m.Called(ctx, vmid)
	return args.Get(0).(proxmox.TermProxyResponse), args.Error(1)
}

// ─── Mock: Provisioner ───────────────────────────────────────────────────────

type mockProvisioner struct{ mock.Mock }

func (m *mockProvisioner) Provision(ctx context.Context, workspaceID uuid.UUID, spec repository.VMSpec) (int, error) {
	args := m.Called(ctx, workspaceID, spec)
	return args.Int(0), args.Error(1)
}

func (m *mockProvisioner) Deprovision(ctx context.Context, workspaceID uuid.UUID) error {
	return m.Called(ctx, workspaceID).Error(0)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newVMUC(vmRepo *mockVMRepo, px *mockProxmoxClient, prov *mockProvisioner) *usecase.VMUseCase {
	return usecase.NewVMUseCase(vmRepo, px, prov, "pve01")
}

func baseCreateReq() usecase.CreateVMRequest {
	return usecase.CreateVMRequest{
		Name:       "minha-vm",
		CPUCores:   1,
		MemoryMB:   1024,
		DiskGB:     4,
		OSTemplate: "ubuntu-22.04",
		Pool:       "students",
		Tags:       []string{"proxlab", "aluno"},
		Role:       "aluno",
	}
}

// ─── Testes: CreateVM — limites de role ──────────────────────────────────────

// Cenário: aluno solicita 8 CPUs (limite é 2) → ErrLimitExceeded, sem provisionar.
func TestCreateVM_AlunoExceedsCPULimit(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	uc := newVMUC(vmRepo, px, prov)
	req := baseCreateReq()
	req.CPUCores = 8

	_, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)
	prov.AssertNotCalled(t, "Provision")
	vmRepo.AssertNotCalled(t, "Create")
}

// Cenário: aluno solicita 8 GB de RAM (limite é 4096 MB) → ErrLimitExceeded.
func TestCreateVM_AlunoExceedsRAMLimit(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	uc := newVMUC(vmRepo, px, prov)
	req := baseCreateReq()
	req.MemoryMB = 8192

	_, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)
	prov.AssertNotCalled(t, "Provision")
}

// Cenário: pesquisador com 8 CPUs — dentro do limite → provisiona e persiste com o VMID retornado.
func TestCreateVM_PesquisadorWithinLimit(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	prov.On("Provision", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.MatchedBy(func(s repository.VMSpec) bool {
		return s.Name == "minha-vm" && s.CPUCores == 8 && s.MemoryMB == 16384 && s.OSType == "l26" && s.Pool == "researchers"
	})).Return(101, nil)
	vmRepo.On("Create", mock.Anything, mock.MatchedBy(func(vm *entity.VM) bool {
		return vm.ProxmoxVMID == 101 && vm.Node == "pve01" && vm.Status == entity.VMStatusCreating && vm.Tags == "proxlab;aluno"
	})).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	req := baseCreateReq()
	req.CPUCores = 8
	req.MemoryMB = 16384
	req.Pool = "researchers"
	req.Role = "pesquisador"

	vm, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.NoError(t, err)
	assert.NotNil(t, vm)
	assert.Equal(t, 101, vm.ProxmoxVMID)
	prov.AssertExpectations(t)
	vmRepo.AssertExpectations(t)
}

// Cenário: o UUID passado ao Provision é o mesmo ID da entidade persistida.
func TestCreateVM_WorkspaceIDMatchesEntityID(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	var provisionedID uuid.UUID
	prov.On("Provision", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.Anything).
		Run(func(args mock.Arguments) { provisionedID = args.Get(1).(uuid.UUID) }).
		Return(102, nil)
	vmRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.VM")).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	vm, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.NoError(t, err)
	assert.Equal(t, provisionedID, vm.ID)
}

// Cenário: role desconhecido → aplica limites de aluno por segurança.
func TestCreateVM_UnknownRoleFallsBackToAlunoLimits(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	uc := newVMUC(vmRepo, px, prov)
	req := baseCreateReq()
	req.CPUCores = 4
	req.Role = "convidado" // não existe

	_, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)
}

// Cenário: Provision falha → erro propagado, nada persistido.
func TestCreateVM_ProvisionError(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).Return(0, errors.New("terraform apply falhou"))

	uc := newVMUC(vmRepo, px, prov)
	_, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.Error(t, err)
	vmRepo.AssertNotCalled(t, "Create")
}

// Cenário: Provision ok mas banco falha → Deprovision best-effort para não deixar VM órfã.
func TestCreateVM_RepoErrorTriggersDeprovision(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).Return(103, nil)
	vmRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db down"))
	prov.On("Deprovision", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	_, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.Error(t, err)
	prov.AssertCalled(t, "Deprovision", mock.Anything, mock.AnythingOfType("uuid.UUID"))
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
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)

	uc := newVMUC(vmRepo, px, prov)
	_, err := uc.GetVM(context.Background(), attackerID, vmID)

	assert.ErrorIs(t, err, usecase.ErrAccessDenied)
	// status sync não deve ser chamado antes da checagem de ownership
	px.AssertNotCalled(t, "GetVMStatus")
}

// Cenário: VM não existe no banco → ErrVMNotFound.
func TestGetVM_NotFound(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil, nil)

	uc := newVMUC(vmRepo, px, prov)
	_, err := uc.GetVM(context.Background(), uuid.New(), uuid.New())

	assert.ErrorIs(t, err, usecase.ErrVMNotFound)
}

// ─── Testes: DeleteVM ─────────────────────────────────────────────────────────

// Cenário: dono da VM chama Delete → Deprovision e remove do banco.
func TestDeleteVM_Success(t *testing.T) {
	userID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: userID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)
	prov.On("Deprovision", mock.Anything, vmID).Return(nil)
	vmRepo.On("Delete", mock.Anything, vmID).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), userID, vmID)

	assert.NoError(t, err)
	prov.AssertExpectations(t)
	vmRepo.AssertExpectations(t)
}

// Cenário: VM pré-migração (sem workspace) → aviso em log, ainda remove do banco.
func TestDeleteVM_MissingWorkspaceStillDeletesRecord(t *testing.T) {
	userID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: userID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)
	prov.On("Deprovision", mock.Anything, vmID).Return(repository.ErrWorkspaceNotFound)
	vmRepo.On("Delete", mock.Anything, vmID).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), userID, vmID)

	assert.NoError(t, err)
	vmRepo.AssertCalled(t, "Delete", mock.Anything, vmID)
}

// Cenário: Deprovision falha por outro motivo → não remove do banco.
func TestDeleteVM_DeprovisionErrorKeepsRecord(t *testing.T) {
	userID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: userID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)
	prov.On("Deprovision", mock.Anything, vmID).Return(errors.New("terraform destroy falhou"))

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), userID, vmID)

	assert.Error(t, err)
	vmRepo.AssertNotCalled(t, "Delete")
}

// Cenário: usuário tenta deletar VM de outro → ErrAccessDenied, sem Deprovision.
func TestDeleteVM_AnotherUsersVM(t *testing.T) {
	ownerID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: ownerID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), uuid.New(), vmID)

	assert.ErrorIs(t, err, usecase.ErrAccessDenied)
	prov.AssertNotCalled(t, "Deprovision")
}
