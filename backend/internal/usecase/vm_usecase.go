package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
)

type CreateVMInput struct {
	Name       string
	CPUCores   int
	MemoryMB   int
	DiskGB     int
	OSTemplate string
	Node       string
}

type CreateVMOutput struct {
	VMID         int
	Status       entity.VMStatus
	WebSocketURL string
}

type VMUseCase struct {
	vmRepo        repository.VMRepository
	proxmoxClient repository.ProxmoxClientInterface
}

func NewVMUseCase(vmRepo repository.VMRepository, proxmoxClient repository.ProxmoxClientInterface) *VMUseCase {
	return &VMUseCase{vmRepo: vmRepo, proxmoxClient: proxmoxClient}
}

func (uc *VMUseCase) Create(ctx context.Context, userID uuid.UUID, input CreateVMInput) (*CreateVMOutput, error) {
	_ = userID
	_ = input
	return nil, errors.New("not implemented")
}

func (uc *VMUseCase) List(ctx context.Context, userID uuid.UUID) ([]entity.VM, error) {
	_ = userID
	return nil, errors.New("not implemented")
}

func (uc *VMUseCase) Get(ctx context.Context, userID, vmID uuid.UUID) (*entity.VM, error) {
	_ = userID
	_ = vmID
	return nil, errors.New("not implemented")
}

func (uc *VMUseCase) Start(ctx context.Context, userID, vmID uuid.UUID) error {
	_ = userID
	_ = vmID
	return errors.New("not implemented")
}

func (uc *VMUseCase) Stop(ctx context.Context, userID, vmID uuid.UUID) error {
	_ = userID
	_ = vmID
	return errors.New("not implemented")
}

func (uc *VMUseCase) Delete(ctx context.Context, userID, vmID uuid.UUID) error {
	_ = userID
	_ = vmID
	return errors.New("not implemented")
}
