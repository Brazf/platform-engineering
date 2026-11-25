package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─── Mocks: quem apaga VMs e containers ──────────────────────────────────────

type mockVMDeleter struct{ mock.Mock }

func (m *mockVMDeleter) DeleteVM(ctx context.Context, userID, vmID uuid.UUID) error {
	return m.Called(ctx, userID, vmID).Error(0)
}

type mockContainerDeleter struct{ mock.Mock }

func (m *mockContainerDeleter) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return m.Called(ctx, userID, id).Error(0)
}

func (m *mockContainerDeleter) DeleteUserNamespace(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}

type adminDeps struct {
	users      *mockUserRepo
	vms        *mockVMRepo
	containers *mockContainerRepo
	vmDel      *mockVMDeleter
	ctDel      *mockContainerDeleter
	uc         *usecase.UserAdminUseCase
}

func newAdmin() adminDeps {
	d := adminDeps{
		users: new(mockUserRepo), vms: new(mockVMRepo), containers: new(mockContainerRepo),
		vmDel: new(mockVMDeleter), ctDel: new(mockContainerDeleter),
	}
	d.uc = usecase.NewUserAdminUseCase(d.users, d.vms, d.containers, d.vmDel, d.ctDel)
	return d
}

// ─── List ────────────────────────────────────────────────────────────────────

// Cenário: lista traz cada usuário com a contagem de VMs e containers.
func TestAdminList_Counts(t *testing.T) {
	d := newAdmin()
	u := entity.User{ID: uuid.New(), Email: "a@b.com", Role: entity.RoleAluno}
	d.users.On("List", mock.Anything).Return([]entity.User{u}, nil)
	d.vms.On("FindByUserID", mock.Anything, u.ID).Return([]entity.VM{{ID: uuid.New()}, {ID: uuid.New()}}, nil)
	d.containers.On("CountByUserID", mock.Anything, u.ID).Return(3, nil)

	got, err := d.uc.List(context.Background())

	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, 2, got[0].VMCount)
	assert.Equal(t, 3, got[0].ContainerCount)
}

// ─── UpdateEmail ─────────────────────────────────────────────────────────────

// Cenário: e-mail já usado por outra pessoa — ErrEmailAlreadyInUse.
func TestAdminUpdateEmail_Conflict(t *testing.T) {
	d := newAdmin()
	u := &entity.User{ID: uuid.New(), Email: "a@b.com"}
	other := &entity.User{ID: uuid.New(), Email: "c@d.com"}
	d.users.On("FindByID", mock.Anything, u.ID).Return(u, nil)
	d.users.On("FindByEmail", mock.Anything, "c@d.com").Return(other, nil)

	_, err := d.uc.UpdateEmail(context.Background(), u.ID, " C@D.com ")

	assert.ErrorIs(t, err, usecase.ErrEmailAlreadyInUse)
	d.users.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// Cenário: e-mail livre — salvo normalizado (minúsculo, sem espaços).
func TestAdminUpdateEmail_OK(t *testing.T) {
	d := newAdmin()
	u := &entity.User{ID: uuid.New(), Email: "a@b.com"}
	d.users.On("FindByID", mock.Anything, u.ID).Return(u, nil)
	d.users.On("FindByEmail", mock.Anything, "novo@b.com").Return(nil, nil)
	d.users.On("Update", mock.Anything, mock.MatchedBy(func(x *entity.User) bool { return x.Email == "novo@b.com" })).Return(nil)

	got, err := d.uc.UpdateEmail(context.Background(), u.ID, " Novo@B.com")

	assert.NoError(t, err)
	assert.Equal(t, "novo@b.com", got.Email)
}

// Cenário: usuário inexistente — ErrUserNotFound.
func TestAdminUpdateEmail_NotFound(t *testing.T) {
	d := newAdmin()
	id := uuid.New()
	d.users.On("FindByID", mock.Anything, id).Return(nil, nil)

	_, err := d.uc.UpdateEmail(context.Background(), id, "x@y.com")

	assert.ErrorIs(t, err, usecase.ErrUserNotFound)
}

// ─── Delete ──────────────────────────────────────────────────────────────────

// Cenário: pesquisador tenta se excluir — ErrCannotDeleteSelf, nada é tocado.
func TestAdminDelete_Self(t *testing.T) {
	d := newAdmin()
	id := uuid.New()

	err := d.uc.Delete(context.Background(), id, id)

	assert.ErrorIs(t, err, usecase.ErrCannotDeleteSelf)
	d.users.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

// Cenário: apaga todas as VMs e containers do usuário e depois o usuário.
func TestAdminDelete_RemovesResourcesThenUser(t *testing.T) {
	d := newAdmin()
	actor, target := uuid.New(), uuid.New()
	vm1, ct1 := uuid.New(), uuid.New()
	d.users.On("FindByID", mock.Anything, target).Return(&entity.User{ID: target}, nil)
	d.vms.On("FindByUserID", mock.Anything, target).Return([]entity.VM{{ID: vm1}}, nil)
	d.containers.On("FindByUserID", mock.Anything, target).Return([]entity.Container{{ID: ct1}}, nil)
	d.vmDel.On("DeleteVM", mock.Anything, target, vm1).Return(nil)
	d.ctDel.On("Delete", mock.Anything, target, ct1).Return(nil)
	d.ctDel.On("DeleteUserNamespace", mock.Anything, target).Return(nil)
	d.users.On("Delete", mock.Anything, target).Return(nil)

	err := d.uc.Delete(context.Background(), actor, target)

	assert.NoError(t, err)
	d.vmDel.AssertExpectations(t)
	d.ctDel.AssertExpectations(t)
	d.users.AssertExpectations(t)
}

// Cenário: um recurso falha — tenta os demais, NÃO apaga o usuário e diz o que ficou.
func TestAdminDelete_ResourceFails_KeepsUser(t *testing.T) {
	d := newAdmin()
	actor, target := uuid.New(), uuid.New()
	vm1, vm2, ct1 := uuid.New(), uuid.New(), uuid.New()
	d.users.On("FindByID", mock.Anything, target).Return(&entity.User{ID: target}, nil)
	d.vms.On("FindByUserID", mock.Anything, target).Return([]entity.VM{{ID: vm1}, {ID: vm2}}, nil)
	d.containers.On("FindByUserID", mock.Anything, target).Return([]entity.Container{{ID: ct1}}, nil)
	d.vmDel.On("DeleteVM", mock.Anything, target, vm1).Return(errors.New("terraform caiu"))
	d.vmDel.On("DeleteVM", mock.Anything, target, vm2).Return(nil)
	d.ctDel.On("Delete", mock.Anything, target, ct1).Return(errors.New("cluster fora"))

	err := d.uc.Delete(context.Background(), actor, target)

	var rem *usecase.ResourcesRemainingError
	assert.ErrorAs(t, err, &rem)
	assert.Equal(t, 1, rem.VMs)
	assert.Equal(t, 1, rem.Containers)
	d.vmDel.AssertExpectations(t)
	d.ctDel.AssertNotCalled(t, "DeleteUserNamespace", mock.Anything, mock.Anything)
	d.users.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

// Cenário: recursos saíram mas o namespace não — usuário mantido, erro aponta o namespace.
func TestAdminDelete_NamespaceFails_KeepsUser(t *testing.T) {
	d := newAdmin()
	actor, target := uuid.New(), uuid.New()
	d.users.On("FindByID", mock.Anything, target).Return(&entity.User{ID: target}, nil)
	d.vms.On("FindByUserID", mock.Anything, target).Return([]entity.VM{}, nil)
	d.containers.On("FindByUserID", mock.Anything, target).Return([]entity.Container{}, nil)
	d.ctDel.On("DeleteUserNamespace", mock.Anything, target).Return(errors.New("cluster fora"))

	err := d.uc.Delete(context.Background(), actor, target)

	var rem *usecase.ResourcesRemainingError
	assert.ErrorAs(t, err, &rem)
	assert.True(t, rem.Namespace)
	d.users.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}
