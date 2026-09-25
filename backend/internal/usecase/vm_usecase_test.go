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

func (m *mockProxmoxClient) RebootVM(ctx context.Context, vmid int) (string, error) {
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
	return usecase.NewVMUseCase(vmRepo, px, prov, "pve01", 9000, testPools)
}

// Pools distintos por perfil para provar que o backend escolhe (e não o front).
var testPools = map[string]string{"aluno": "pool-alunos", "pesquisador": "researchers"}

func baseCreateReq() usecase.CreateVMRequest {
	return usecase.CreateVMRequest{
		Name:       "minha-vm",
		CPUCores:   1,
		MemoryMB:   1024,
		DiskGB:     4,
		OSTemplate: "debian-11",
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
	req.Role = "pesquisador"

	res, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, 101, res.VM.ProxmoxVMID)
	assert.Nil(t, res.Credentials)
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
	res, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.NoError(t, err)
	assert.Equal(t, provisionedID, res.VM.ID)
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

// Cenário: template Ubuntu → clone do template cloud-init, credenciais geradas, VM nasce running.
func TestCreateVM_UbuntuClonesTemplateWithCredentials(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	var spec repository.VMSpec
	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { spec = args.Get(2).(repository.VMSpec) }).
		Return(104, nil)
	vmRepo.On("Create", mock.Anything, mock.MatchedBy(func(vm *entity.VM) bool {
		return vm.Status == entity.VMStatusRunning
	})).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	req := baseCreateReq()
	req.OSTemplate = "ubuntu-22.04"

	res, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.NoError(t, err)
	assert.Equal(t, 9000, spec.CloneVMID)
	assert.Equal(t, "ubuntu", spec.CloudInitUser)
	assert.NotEmpty(t, spec.CloudInitPassword)
	assert.NotNil(t, res.Credentials)
	assert.Equal(t, "ubuntu", res.Credentials.Username)
	assert.Equal(t, spec.CloudInitPassword, res.Credentials.Password)
	assert.Len(t, res.Credentials.Password, 16)
	vmRepo.AssertExpectations(t)
}

// Cenário: template não-Ubuntu → sem clone, sem credenciais, VM nasce creating.
func TestCreateVM_NonUbuntuDoesNotClone(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	var spec repository.VMSpec
	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { spec = args.Get(2).(repository.VMSpec) }).
		Return(105, nil)
	vmRepo.On("Create", mock.Anything, mock.MatchedBy(func(vm *entity.VM) bool {
		return vm.Status == entity.VMStatusCreating
	})).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	req := baseCreateReq()
	req.OSTemplate = "windows-10"

	res, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.NoError(t, err)
	assert.Equal(t, 0, spec.CloneVMID)
	assert.Empty(t, spec.CloudInitPassword)
	assert.Nil(t, res.Credentials)
}

// Cenário: clone desabilitado (template VMID = 0) → Ubuntu segue o fluxo sem clone.
func TestCreateVM_UbuntuWithCloneDisabled(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	var spec repository.VMSpec
	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { spec = args.Get(2).(repository.VMSpec) }).
		Return(106, nil)
	vmRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	uc := usecase.NewVMUseCase(vmRepo, px, prov, "pve01", 0, testPools)
	req := baseCreateReq()
	req.OSTemplate = "ubuntu-22.04"

	res, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.NoError(t, err)
	assert.Equal(t, 0, spec.CloneVMID)
	assert.Nil(t, res.Credentials)
}

// Cenário: o pool vem do perfil do usuário (config), não do corpo da requisição.
func TestCreateVM_PoolComesFromRole(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	prov.On("Provision", mock.Anything, mock.Anything, mock.MatchedBy(func(s repository.VMSpec) bool {
		return s.Pool == "pool-alunos"
	})).Return(200, nil)
	vmRepo.On("Create", mock.Anything, mock.MatchedBy(func(v *entity.VM) bool { return v.Pool == "pool-alunos" })).Return(nil)

	_, err := newVMUC(vmRepo, px, prov).CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.NoError(t, err)
	prov.AssertExpectations(t)
	vmRepo.AssertExpectations(t)
}
