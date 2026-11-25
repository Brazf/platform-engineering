package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
)

var ErrCannotDeleteSelf = errors.New("você não pode excluir a própria conta")

// ResourcesRemainingError: a exclusão do usuário parou porque alguns recursos não saíram.
type ResourcesRemainingError struct {
	VMs        int
	Containers int
	Namespace  bool // VMs e containers saíram, mas o namespace do cluster não
}

func (e *ResourcesRemainingError) Error() string {
	if e.VMs == 0 && e.Containers == 0 && e.Namespace {
		return "não foi possível remover o espaço do usuário no cluster de containers; ele não foi excluído, tente novamente"
	}
	return fmt.Sprintf("não foi possível apagar %d VM(s) e %d container(s) do usuário; ele não foi excluído, tente novamente", e.VMs, e.Containers)
}

// vmDeleter e containerDeleter são satisfeitos por VMUseCase e ContainerUseCase:
// a exclusão de um usuário reaproveita o mesmo caminho de remoção que o próprio dono usa.
type vmDeleter interface {
	DeleteVM(ctx context.Context, userID, vmID uuid.UUID) error
}

type containerDeleter interface {
	Delete(ctx context.Context, userID, id uuid.UUID) error
	DeleteUserNamespace(ctx context.Context, userID uuid.UUID) error
}

// UserSummary é um usuário na lista de "Usuários cadastrados".
type UserSummary struct {
	User           entity.User
	VMCount        int
	ContainerCount int
}

// UserAdminUseCase: ações do pesquisador sobre as contas (listar, trocar e-mail, excluir).
type UserAdminUseCase struct {
	users       repository.UserRepository
	vms         repository.VMRepository
	containers  repository.ContainerRepository
	vmDeleter   vmDeleter
	contDeleter containerDeleter
}

func NewUserAdminUseCase(
	users repository.UserRepository,
	vms repository.VMRepository,
	containers repository.ContainerRepository,
	vmDel vmDeleter,
	contDel containerDeleter,
) *UserAdminUseCase {
	return &UserAdminUseCase{users: users, vms: vms, containers: containers, vmDeleter: vmDel, contDeleter: contDel}
}

func (uc *UserAdminUseCase) List(ctx context.Context) ([]UserSummary, error) {
	users, err := uc.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar usuários: %w", err)
	}
	out := make([]UserSummary, 0, len(users))
	for _, u := range users {
		vms, err := uc.vms.FindByUserID(ctx, u.ID)
		if err != nil {
			return nil, fmt.Errorf("erro ao contar VMs: %w", err)
		}
		cts, err := uc.containers.CountByUserID(ctx, u.ID)
		if err != nil {
			return nil, fmt.Errorf("erro ao contar containers: %w", err)
		}
		out = append(out, UserSummary{User: u, VMCount: len(vms), ContainerCount: int(cts)})
	}
	return out, nil
}

// UpdateEmail troca o e-mail (login) de um usuário; 409 se outra conta já usa.
func (uc *UserAdminUseCase) UpdateEmail(ctx context.Context, id uuid.UUID, email string) (*entity.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar usuário: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if email == user.Email {
		return user, nil
	}
	existing, err := uc.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("erro ao verificar email: %w", err)
	}
	if existing != nil && existing.ID != user.ID {
		return nil, ErrEmailAlreadyInUse
	}
	user.Email = email
	if err := uc.users.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("erro ao salvar email: %w", err)
	}
	return user, nil
}

// Delete apaga todas as VMs e containers do usuário, o namespace dele no cluster e, só se tudo sair, o próprio usuário.
// Tenta todos os recursos mesmo que um falhe, para a nova tentativa ter menos trabalho.
func (uc *UserAdminUseCase) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	if actorID == id {
		return ErrCannotDeleteSelf
	}
	user, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("erro ao buscar usuário: %w", err)
	}
	if user == nil {
		return ErrUserNotFound
	}

	vms, err := uc.vms.FindByUserID(ctx, id)
	if err != nil {
		return fmt.Errorf("erro ao listar VMs do usuário: %w", err)
	}
	cts, err := uc.containers.FindByUserID(ctx, id)
	if err != nil {
		return fmt.Errorf("erro ao listar containers do usuário: %w", err)
	}

	remaining := &ResourcesRemainingError{}
	for _, vm := range vms {
		if err := uc.vmDeleter.DeleteVM(ctx, id, vm.ID); err != nil {
			log.Printf("[usuarios] exclusão de %s: VM %s falhou: %v", id, vm.ID, err)
			remaining.VMs++
		}
	}
	for _, c := range cts {
		if err := uc.contDeleter.Delete(ctx, id, c.ID); err != nil {
			log.Printf("[usuarios] exclusão de %s: container %s falhou: %v", id, c.ID, err)
			remaining.Containers++
		}
	}
	if remaining.VMs > 0 || remaining.Containers > 0 {
		return remaining // namespace fica: apagá-lo levaria junto containers que ainda estão no banco
	}
	if err := uc.contDeleter.DeleteUserNamespace(ctx, id); err != nil {
		remaining.Namespace = true
		return remaining
	}

	if err := uc.users.Delete(ctx, id); err != nil {
		return fmt.Errorf("erro ao remover usuário: %w", err)
	}
	return nil
}
