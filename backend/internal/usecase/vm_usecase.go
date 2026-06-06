package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/proxmox"
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
	Node       string
	Pool       string   // "researchers" ou "students"
	Tags       []string // ex: ["proxlab","aluno"]
	Role       string   // role do usuário autenticado, para validar limites
	RequireGPU bool
}

type VMUseCase struct {
	vmRepo        repository.VMRepository
	proxmoxClient repository.ProxmoxClientInterface
}

func NewVMUseCase(vmRepo repository.VMRepository, proxmoxClient repository.ProxmoxClientInterface) *VMUseCase {
	return &VMUseCase{vmRepo: vmRepo, proxmoxClient: proxmoxClient}
}

// CreateVM valida limites por role, cria a VM no Proxmox e persiste no banco.
func (uc *VMUseCase) CreateVM(ctx context.Context, userID uuid.UUID, req CreateVMRequest) (*entity.VM, error) {
	if err := validateRoleLimits(req.Role, req.CPUCores, req.MemoryMB); err != nil {
		return nil, err
	}

	node := req.Node
	if node == "" {
		node = "pve"
	}

	vmid, _, err := uc.proxmoxClient.CreateVM(ctx, proxmox.VMCreateRequest{
		Name:       req.Name,
		CPUCores:   req.CPUCores,
		MemoryMB:   req.MemoryMB,
		DiskGB:     req.DiskGB,
		OSType:     osTypeFromTemplate(req.OSTemplate),
		Pool:       req.Pool,
		Tags:       req.Tags,
		RequireGPU: req.RequireGPU,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao criar VM no Proxmox: %w", err)
	}

	vm := &entity.VM{
		ID:          uuid.New(),
		UserID:      userID,
		ProxmoxVMID: vmid,
		Name:        req.Name,
		CPUCores:    req.CPUCores,
		MemoryMB:    req.MemoryMB,
		DiskGB:      req.DiskGB,
		OSTemplate:  req.OSTemplate,
		Node:        node,
		Pool:        req.Pool,
		Tags:        strings.Join(req.Tags, ";"),
		RequireGPU:  req.RequireGPU,
		Status:      entity.VMStatusCreating,
	}

	if err := uc.vmRepo.Create(ctx, vm); err != nil {
		return nil, fmt.Errorf("erro ao salvar VM no banco: %w", err)
	}

	return vm, nil
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

// DeleteVM destrói a VM no Proxmox e remove do banco.
func (uc *VMUseCase) DeleteVM(ctx context.Context, userID, vmID uuid.UUID) error {
	vm, err := uc.findOwned(ctx, userID, vmID)
	if err != nil {
		return err
	}
	if err := uc.proxmoxClient.DeleteVM(ctx, vm.ProxmoxVMID); err != nil {
		return fmt.Errorf("erro ao deletar VM no Proxmox: %w", err)
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
