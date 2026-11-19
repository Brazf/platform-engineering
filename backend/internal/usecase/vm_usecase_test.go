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

func (m *mockVMRepo) FindWithGPU(ctx context.Context) ([]entity.VM, error) {
	args := m.Called(ctx)
	return args.Get(0).([]entity.VM), args.Error(1)
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

func (m *mockProxmoxClient) WaitTask(ctx context.Context, upid string) (string, error) {
	args := m.Called(ctx, upid)
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
	return usecase.NewVMUseCase(vmRepo, px, prov, "pve01", 9000, testPools, testGPUs)
}

var testGPUs = []string{"gpu-01", "gpu-02"}

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

	uc := usecase.NewVMUseCase(vmRepo, px, prov, "pve01", 0, testPools, testGPUs)
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

// ─── Testes: CreateVM — GPU ──────────────────────────────────────────────────

// gpuCreate roda CreateVM com GPU; existing é o que FindWithGPU devolve.
// Devolve o spec enviado ao Provision e a VM gravada.
func gpuCreate(t *testing.T, mappings []string, existing []entity.VM, tweak func(*usecase.CreateVMRequest)) (repository.VMSpec, *entity.VM, *usecase.CreateVMResult, error, *mockVMRepo, *mockProvisioner) {
	t.Helper()
	vmRepo := new(mockVMRepo)
	prov := new(mockProvisioner)
	var spec repository.VMSpec
	var saved *entity.VM
	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { spec = args.Get(2).(repository.VMSpec) }).
		Return(110, nil).Maybe()
	vmRepo.On("FindWithGPU", mock.Anything).Return(existing, nil).Maybe()
	vmRepo.On("Create", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { saved = args.Get(1).(*entity.VM) }).
		Return(nil).Maybe()

	req := baseCreateReq()
	req.RequireGPU = true
	if tweak != nil {
		tweak(&req)
	}
	uc := usecase.NewVMUseCase(vmRepo, new(mockProxmoxClient), prov, "pve01", 9000, testPools, mappings)
	res, err := uc.CreateVM(context.Background(), uuid.New(), req)
	return spec, saved, res, err, vmRepo, prov
}

func vmsOn(mappings ...string) []entity.VM {
	var out []entity.VM
	for _, m := range mappings {
		out = append(out, entity.VM{ID: uuid.New(), GPUMapping: m, RequireGPU: true})
	}
	return out
}

// Cenário: GPU pedida sem nenhum mapping configurado → ErrGPUUnavailable, sem provisionar.
func TestCreateVM_GPUWithoutMappingsIsUnavailable(t *testing.T) {
	_, _, _, err, vmRepo, prov := gpuCreate(t, nil, nil, nil)

	assert.ErrorIs(t, err, usecase.ErrGPUUnavailable)
	prov.AssertNotCalled(t, "Provision")
	vmRepo.AssertNotCalled(t, "Create")
}

// Cenário: primeira VM com GPU → primeiro mapping, gravado na entidade.
func TestCreateVM_FirstGPUVMGetsFirstMapping(t *testing.T) {
	spec, saved, _, err, _, _ := gpuCreate(t, testGPUs, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, "gpu-01", spec.GPUMapping)
	assert.Equal(t, "gpu-01", saved.GPUMapping)
}

// Cenário: balanceia — 1 VM na gpu-01 e 0 na gpu-02 → gpu-02.
func TestCreateVM_GPUBalancesToLeastUsed(t *testing.T) {
	spec, saved, _, err, _, _ := gpuCreate(t, testGPUs, vmsOn("gpu-01"), nil)

	assert.NoError(t, err)
	assert.Equal(t, "gpu-02", spec.GPUMapping)
	assert.Equal(t, "gpu-02", saved.GPUMapping)
}

// Cenário: empate → ordem da config.
func TestCreateVM_GPUTieFollowsConfigOrder(t *testing.T) {
	spec, _, _, err, _, _ := gpuCreate(t, testGPUs, vmsOn("gpu-01", "gpu-02"), nil)

	assert.NoError(t, err)
	assert.Equal(t, "gpu-01", spec.GPUMapping)
}

// Cenário: mapping que saiu da config não entra na contagem.
func TestCreateVM_GPUIgnoresMappingsOutsideConfig(t *testing.T) {
	spec, _, _, err, _, _ := gpuCreate(t, testGPUs, vmsOn("gpu-velha", "gpu-velha", "gpu-01"), nil)

	assert.NoError(t, err)
	assert.Equal(t, "gpu-02", spec.GPUMapping)
}

// Cenário: clone Ubuntu com GPU nasce desligado e ainda devolve credenciais.
func TestCreateVM_UbuntuCloneWithGPUIsStopped(t *testing.T) {
	_, saved, res, err, _, _ := gpuCreate(t, testGPUs, nil, func(r *usecase.CreateVMRequest) { r.OSTemplate = "ubuntu-22.04" })

	assert.NoError(t, err)
	assert.NotNil(t, res.Credentials)
	assert.Equal(t, entity.VMStatusStopped, saved.Status)
}

// Cenário: sem GPU → sem mapping, sem FindWithGPU, Ubuntu continua running.
func TestCreateVM_NoGPUKeepsCurrentBehavior(t *testing.T) {
	spec, saved, _, err, vmRepo, _ := gpuCreate(t, testGPUs, nil, func(r *usecase.CreateVMRequest) {
		r.RequireGPU = false
		r.OSTemplate = "ubuntu-22.04"
	})

	assert.NoError(t, err)
	assert.Empty(t, spec.GPUMapping)
	assert.Empty(t, saved.GPUMapping)
	assert.Equal(t, entity.VMStatusRunning, saved.Status)
	vmRepo.AssertNotCalled(t, "FindWithGPU", mock.Anything)
}

// Cenário: ErrGPUNotPermitted do provisioner sobe embrulhado.
func TestCreateVM_GPUNotPermittedPropagates(t *testing.T) {
	vmRepo := new(mockVMRepo)
	prov := new(mockProvisioner)
	vmRepo.On("FindWithGPU", mock.Anything).Return([]entity.VM(nil), nil)
	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).Return(0, repository.ErrGPUNotPermitted)

	req := baseCreateReq()
	req.RequireGPU = true
	_, err := newVMUC(vmRepo, new(mockProxmoxClient), prov).CreateVM(context.Background(), uuid.New(), req)

	assert.ErrorIs(t, err, repository.ErrGPUNotPermitted)
}

// ─── Testes: StartVM ─────────────────────────────────────────────────────────

type startEnv struct {
	vmRepo *mockVMRepo
	px     *mockProxmoxClient
	uc     *usecase.VMUseCase
	userID uuid.UUID
	vm     *entity.VM
}

func newStartEnv(vm entity.VM) *startEnv {
	e := &startEnv{vmRepo: new(mockVMRepo), px: new(mockProxmoxClient), userID: uuid.New()}
	vm.ID = uuid.New()
	vm.UserID = e.userID
	e.vm = &vm
	e.vmRepo.On("FindByID", mock.Anything, vm.ID).Return(e.vm, nil)
	e.uc = usecase.NewVMUseCase(e.vmRepo, e.px, new(mockProvisioner), "pve01", 9000, testPools, testGPUs)
	return e
}

func (e *startEnv) start() error { return e.uc.StartVM(context.Background(), e.userID, e.vm.ID) }

func (e *startEnv) expectStart(exit string, waitErr error) {
	e.px.On("StartVM", mock.Anything, e.vm.ProxmoxVMID).Return("UPID:1", nil)
	e.px.On("WaitTask", mock.Anything, "UPID:1").Return(exit, waitErr)
}

func (e *startEnv) others(vms ...entity.VM) {
	e.vmRepo.On("FindWithGPU", mock.Anything).Return(vms, nil)
}

func otherVM(mapping string, vmid int) entity.VM {
	return entity.VM{ID: uuid.New(), ProxmoxVMID: vmid, RequireGPU: true, GPUMapping: mapping}
}

// Cenário: VM sem GPU → caminho antigo, sem FindWithGPU nem WaitTask.
func TestStartVM_NoGPUUsesPlainPath(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 300})
	e.px.On("StartVM", mock.Anything, 300).Return("UPID:1", nil)
	e.vmRepo.On("UpdateStatus", mock.Anything, e.vm.ID, entity.VMStatusRunning).Return(nil)

	assert.NoError(t, e.start())
	e.vmRepo.AssertNotCalled(t, "FindWithGPU", mock.Anything)
	e.px.AssertNotCalled(t, "WaitTask", mock.Anything, mock.Anything)
}

// Cenário: GPU livre → liga, espera a tarefa e marca running.
func TestStartVM_GPUFreeStarts(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 301, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others()
	e.expectStart("OK", nil)
	e.vmRepo.On("UpdateStatus", mock.Anything, e.vm.ID, entity.VMStatusRunning).Return(nil)

	assert.NoError(t, e.start())
	e.vmRepo.AssertExpectations(t)
}

// Cenário: outra VM ligada no mesmo mapping → ErrGPUInUse, sem StartVM.
func TestStartVM_GPUBusyByOtherRunningVM(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 302, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others(otherVM("gpu-01", 400))
	e.px.On("GetVMStatus", mock.Anything, 400).Return(proxmox.VMStatus{Status: "running"}, nil)

	assert.ErrorIs(t, e.start(), usecase.ErrGPUInUse)
	e.px.AssertNotCalled(t, "StartVM", mock.Anything, mock.Anything)
	e.vmRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

// Cenário: VM pausada ainda segura a placa → conta como ocupada.
func TestStartVM_PausedOtherVMCountsAsBusy(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 303, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others(otherVM("gpu-01", 401))
	e.px.On("GetVMStatus", mock.Anything, 401).Return(proxmox.VMStatus{Status: "paused"}, nil)

	assert.ErrorIs(t, e.start(), usecase.ErrGPUInUse)
}

// Cenário: outra VM desligada no mesmo mapping não bloqueia.
func TestStartVM_StoppedOtherVMDoesNotBlock(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 304, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others(otherVM("gpu-01", 402))
	e.px.On("GetVMStatus", mock.Anything, 402).Return(proxmox.VMStatus{Status: "stopped"}, nil)
	e.expectStart("OK", nil)
	e.vmRepo.On("UpdateStatus", mock.Anything, e.vm.ID, entity.VMStatusRunning).Return(nil)

	assert.NoError(t, e.start())
}

// Cenário: VM ligada em outro mapping não bloqueia (e nem é consultada).
func TestStartVM_OtherMappingDoesNotBlock(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 305, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others(otherVM("gpu-02", 403))
	e.expectStart("OK", nil)
	e.vmRepo.On("UpdateStatus", mock.Anything, e.vm.ID, entity.VMStatusRunning).Return(nil)

	assert.NoError(t, e.start())
	e.px.AssertNotCalled(t, "GetVMStatus", mock.Anything, 403)
}

// Cenário: a própria VM (já com gpu_mapping) não conta contra si mesma.
func TestStartVM_OwnVMIsNotConsidered(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 306, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others(*e.vm)
	e.expectStart("OK", nil)
	e.vmRepo.On("UpdateStatus", mock.Anything, e.vm.ID, entity.VMStatusRunning).Return(nil)

	assert.NoError(t, e.start())
	e.px.AssertNotCalled(t, "GetVMStatus", mock.Anything, mock.Anything)
}

// Cenário: erro ao consultar a outra VM (ex.: apagada no Proxmox) → não bloqueia.
func TestStartVM_StatusErrorOnOtherVMDoesNotBlock(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 307, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others(otherVM("gpu-01", 404))
	e.px.On("GetVMStatus", mock.Anything, 404).Return(proxmox.VMStatus{}, errors.New("500 vm não existe"))
	e.expectStart("OK", nil)
	e.vmRepo.On("UpdateStatus", mock.Anything, e.vm.ID, entity.VMStatusRunning).Return(nil)

	assert.NoError(t, e.start())
}

// Cenário: Proxmox recusa com "already in use" (VM fora do ProxLab) → ErrGPUInUse, status não muda.
func TestStartVM_TaskAlreadyInUseIsConflict(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 308, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others()
	e.expectStart("device 0000:01:00.0 already in use by VMID 77", nil)

	assert.ErrorIs(t, e.start(), usecase.ErrGPUInUse)
	e.vmRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

// Cenário: tarefa falha por outro motivo → ErrVMStartFailed, status não muda.
func TestStartVM_TaskOtherFailureIsStartFailed(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 309, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others()
	e.expectStart("start failed: QEMU exited with code 1", nil)

	err := e.start()
	assert.ErrorIs(t, err, usecase.ErrVMStartFailed)
	assert.NotErrorIs(t, err, usecase.ErrGPUInUse)
	e.vmRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

// Cenário: tarefa falha e a repetição da pré-checagem acha a VM ocupando (corrida) → ErrGPUInUse.
func TestStartVM_TaskFailureWithBusyOtherIsConflict(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 310, RequireGPU: true, GPUMapping: "gpu-01"})
	other := otherVM("gpu-01", 405)
	e.vmRepo.On("FindWithGPU", mock.Anything).Return([]entity.VM{other}, nil)
	e.px.On("GetVMStatus", mock.Anything, 405).Return(proxmox.VMStatus{Status: "stopped"}, nil).Once()
	e.px.On("GetVMStatus", mock.Anything, 405).Return(proxmox.VMStatus{Status: "running"}, nil)
	e.expectStart("start failed: algo opaco", nil)

	assert.ErrorIs(t, e.start(), usecase.ErrGPUInUse)
}

// Cenário: erro/timeout em WaitTask → erro genérico, status do banco não muda.
func TestStartVM_WaitTaskErrorKeepsStatus(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 311, RequireGPU: true, GPUMapping: "gpu-01"})
	e.others()
	e.expectStart("", context.DeadlineExceeded)

	err := e.start()
	assert.Error(t, err)
	assert.NotErrorIs(t, err, usecase.ErrGPUInUse)
	e.vmRepo.AssertNotCalled(t, "UpdateStatus", mock.Anything, mock.Anything, mock.Anything)
}

// Cenário: VM legada (require_gpu sem mapping) → pula pré-checagem, espera a tarefa, traduz o erro.
func TestStartVM_LegacyGPUVMSkipsPrecheckButTranslatesError(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 312, RequireGPU: true})
	e.expectStart("device already in use", nil)

	assert.ErrorIs(t, e.start(), usecase.ErrGPUInUse)
	e.vmRepo.AssertNotCalled(t, "FindWithGPU", mock.Anything)
}

// Cenário: legada com tarefa OK → running.
func TestStartVM_LegacyGPUVMStarts(t *testing.T) {
	e := newStartEnv(entity.VM{ProxmoxVMID: 313, RequireGPU: true})
	e.expectStart("OK", nil)
	e.vmRepo.On("UpdateStatus", mock.Anything, e.vm.ID, entity.VMStatusRunning).Return(nil)

	assert.NoError(t, e.start())
}
