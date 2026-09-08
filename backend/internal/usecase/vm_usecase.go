package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
)

var (
	ErrVMNotFound    = errors.New("VM não encontrada")
	ErrAccessDenied  = errors.New("acesso negado")
	ErrLimitExceeded = errors.New("limite de recursos excedido para o seu perfil")
)

// limites máximos por role
var roleLimits = map[string]struct{ MaxCPU, MaxMemMB int }{
	"aluno":       {MaxCPU: 2, MaxMemMB: 4096},
	"pesquisador": {MaxCPU: 8, MaxMemMB: 16384},
}

// CreateVMRequest é o input para criação de VM, incluindo role para validação de limites.
type CreateVMRequest struct {
	Name       string
	CPUCores   int
	MemoryMB   int
	DiskGB     int
	OSTemplate string
	Pool       string   // "researchers" ou "students"
	Tags       []string // ex: ["proxlab","aluno"]
	Role       string   // role do usuário autenticado, para validar limites
	RequireGPU bool
}

// VMCredentials são as credenciais cloud-init geradas para VMs clonadas de template.
// Não são persistidas: só existem na resposta da criação.
type VMCredentials struct {
	Username string
	Password string
}

// CreateVMResult é o retorno de CreateVM. Credentials é nil quando a VM não foi clonada.
type CreateVMResult struct {
	VM          *entity.VM
	Credentials *VMCredentials
}

const cloudInitUser = "ubuntu"

type VMUseCase struct {
	vmRepo             repository.VMRepository
	proxmoxClient      repository.ProxmoxClientInterface
	provisioner        repository.Provisioner
	node               string // nó Proxmox onde as VMs são criadas (PROXMOX_NODE)
	ubuntuTemplateVMID int    // template cloud-init clonado para VMs Ubuntu; 0 desabilita clone
}

func NewVMUseCase(
	vmRepo repository.VMRepository,
	proxmoxClient repository.ProxmoxClientInterface,
	provisioner repository.Provisioner,
	node string,
	ubuntuTemplateVMID int,
) *VMUseCase {
	return &VMUseCase{
		vmRepo:             vmRepo,
		proxmoxClient:      proxmoxClient,
		provisioner:        provisioner,
		node:               node,
		ubuntuTemplateVMID: ubuntuTemplateVMID,
	}
}

// CreateVM valida limites por role, provisiona a VM via Terraform e persiste no banco.
// O UUID da entidade é gerado antes do provisionamento e usado como ID do workspace.
// VMs Ubuntu são clonadas do template cloud-init e já nascem ligadas, com credenciais geradas.
func (uc *VMUseCase) CreateVM(ctx context.Context, userID uuid.UUID, req CreateVMRequest) (*CreateVMResult, error) {
	if err := validateRoleLimits(req.Role, req.CPUCores, req.MemoryMB); err != nil {
		return nil, err
	}

	spec := repository.VMSpec{
		Name:       req.Name,
		CPUCores:   req.CPUCores,
		MemoryMB:   req.MemoryMB,
		DiskGB:     req.DiskGB,
		OSType:     osTypeFromTemplate(req.OSTemplate),
		Pool:       req.Pool,
		Tags:       req.Tags,
		RequireGPU: req.RequireGPU,
	}

	var creds *VMCredentials
	status := entity.VMStatusCreating
	if uc.ubuntuTemplateVMID > 0 && isUbuntu(req.OSTemplate) {
		password, err := generatePassword()
		if err != nil {
			return nil, err
		}
		creds = &VMCredentials{Username: cloudInitUser, Password: password}
		spec.CloneVMID = uc.ubuntuTemplateVMID
		spec.CloudInitUser = creds.Username
		spec.CloudInitPassword = creds.Password
		status = entity.VMStatusRunning
	}

	id := uuid.New()
	vmid, err := uc.provisioner.Provision(ctx, id, spec)
	if err != nil {
		return nil, fmt.Errorf("erro ao provisionar VM: %w", err)
	}

	vm := &entity.VM{
		ID:          id,
		UserID:      userID,
		ProxmoxVMID: vmid,
		Name:        req.Name,
		CPUCores:    req.CPUCores,
		MemoryMB:    req.MemoryMB,
		DiskGB:      req.DiskGB,
		OSTemplate:  req.OSTemplate,
		Node:        uc.node,
		Pool:        req.Pool,
		Tags:        strings.Join(req.Tags, ";"),
		RequireGPU:  req.RequireGPU,
		Status:      status,
	}

	if err := uc.vmRepo.Create(ctx, vm); err != nil {
		// VM já existe no Proxmox; desfaz para não deixar recurso órfão
		if derr := uc.provisioner.Deprovision(ctx, id); derr != nil {
			log.Printf("[vm] falha ao desprovisionar VM %d após erro no banco: %v", vmid, derr)
		}
		return nil, fmt.Errorf("erro ao salvar VM no banco: %w", err)
	}

	return &CreateVMResult{VM: vm, Credentials: creds}, nil
}

func isUbuntu(template string) bool {
	return strings.Contains(strings.ToLower(template), "ubuntu")
}

// generatePassword gera 16 caracteres URL-safe a partir de 12 bytes aleatórios.
func generatePassword() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("erro ao gerar senha: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ListUserVMs retorna todas as VMs do usuário.
func (uc *VMUseCase) ListUserVMs(ctx context.Context, userID uuid.UUID) ([]entity.VM, error) {
	return uc.vmRepo.FindByUserID(ctx, userID)
}

// GetVM busca uma VM pelo ID, verifica ownership e sincroniza status com o Proxmox.
func (uc *VMUseCase) GetVM(ctx context.Context, userID, vmID uuid.UUID) (*entity.VM, error) {
	vm, err := uc.vmRepo.FindByID(ctx, vmID)
	if err != nil {
		return nil, err
	}
	if vm == nil {
		return nil, ErrVMNotFound
	}
	if vm.UserID != userID {
		return nil, ErrAccessDenied
	}

	// sincroniza status com o Proxmox (best-effort — falha silenciosa)
	if pxStatus, pxErr := uc.proxmoxClient.GetVMStatus(ctx, vm.ProxmoxVMID); pxErr == nil {
		newStatus := entityStatusFromProxmox(pxStatus.Status)
		if newStatus != vm.Status {
			_ = uc.vmRepo.UpdateStatus(ctx, vm.ID, newStatus)
			vm.Status = newStatus
		}
	}

	return vm, nil
}

// StartVM inicia a VM no Proxmox e atualiza o status no banco.
func (uc *VMUseCase) StartVM(ctx context.Context, userID, vmID uuid.UUID) error {
	vm, err := uc.findOwned(ctx, userID, vmID)
	if err != nil {
		return err
	}
	if _, err := uc.proxmoxClient.StartVM(ctx, vm.ProxmoxVMID); err != nil {
		return fmt.Errorf("erro ao iniciar VM no Proxmox: %w", err)
	}
	return uc.vmRepo.UpdateStatus(ctx, vm.ID, entity.VMStatusRunning)
}

// StopVM para a VM no Proxmox e atualiza o status no banco.
func (uc *VMUseCase) StopVM(ctx context.Context, userID, vmID uuid.UUID) error {
	vm, err := uc.findOwned(ctx, userID, vmID)
	if err != nil {
		return err
	}
	if _, err := uc.proxmoxClient.StopVM(ctx, vm.ProxmoxVMID); err != nil {
		return fmt.Errorf("erro ao parar VM no Proxmox: %w", err)
	}
	return uc.vmRepo.UpdateStatus(ctx, vm.ID, entity.VMStatusStopped)
}

// DeleteVM destrói a VM via Terraform e remove do banco.
// VMs criadas antes da migração não têm workspace: o registro é removido e a VM
// fica no Proxmox para remoção manual.
func (uc *VMUseCase) DeleteVM(ctx context.Context, userID, vmID uuid.UUID) error {
	vm, err := uc.findOwned(ctx, userID, vmID)
	if err != nil {
		return err
	}
	if err := uc.provisioner.Deprovision(ctx, vm.ID); err != nil {
		if !errors.Is(err, repository.ErrWorkspaceNotFound) {
			return fmt.Errorf("erro ao desprovisionar VM: %w", err)
		}
		log.Printf("[vm] VM %d (id=%s) sem workspace terraform; removendo só o registro — apagar manualmente no Proxmox", vm.ProxmoxVMID, vm.ID)
	}
	return uc.vmRepo.Delete(ctx, vm.ID)
}

// findOwned busca a VM e verifica que pertence ao usuário.
func (uc *VMUseCase) findOwned(ctx context.Context, userID, vmID uuid.UUID) (*entity.VM, error) {
	vm, err := uc.vmRepo.FindByID(ctx, vmID)
	if err != nil {
		return nil, err
	}
	if vm == nil {
		return nil, ErrVMNotFound
	}
	if vm.UserID != userID {
		return nil, ErrAccessDenied
	}
	return vm, nil
}

func validateRoleLimits(role string, cpuCores, memoryMB int) error {
	limits, ok := roleLimits[role]
	if !ok {
		// role desconhecida → aplica limites de aluno por segurança
		limits = roleLimits["aluno"]
	}
	if cpuCores > limits.MaxCPU {
		return fmt.Errorf("%w: perfil '%s' permite no máximo %d CPU(s)", ErrLimitExceeded, role, limits.MaxCPU)
	}
	if memoryMB > limits.MaxMemMB {
		return fmt.Errorf("%w: perfil '%s' permite no máximo %dMB de RAM", ErrLimitExceeded, role, limits.MaxMemMB)
	}
	return nil
}

func osTypeFromTemplate(template string) string {
	t := strings.ToLower(template)
	switch {
	case strings.Contains(t, "ubuntu") || strings.Contains(t, "debian") || strings.Contains(t, "linux"):
		return "l26"
	case strings.Contains(t, "win11"):
		return "win11"
	case strings.Contains(t, "win10") || strings.Contains(t, "windows"):
		return "win10"
	default:
		return "other"
	}
}

func entityStatusFromProxmox(status string) entity.VMStatus {
	switch status {
	case "running":
		return entity.VMStatusRunning
	case "stopped":
		return entity.VMStatusStopped
	default:
		return entity.VMStatusStopped
	}
}
