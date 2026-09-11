package usecase_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockContainerRepo struct{ mock.Mock }

func (m *mockContainerRepo) Create(ctx context.Context, c *entity.Container) error {
	return m.Called(ctx, c).Error(0)
}
func (m *mockContainerRepo) FindByUserID(ctx context.Context, u uuid.UUID) ([]entity.Container, error) {
	a := m.Called(ctx, u)
	return a.Get(0).([]entity.Container), a.Error(1)
}
func (m *mockContainerRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Container, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*entity.Container), a.Error(1)
}
func (m *mockContainerRepo) FindByUserIDAndName(ctx context.Context, u uuid.UUID, n string) (*entity.Container, error) {
	a := m.Called(ctx, u, n)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*entity.Container), a.Error(1)
}
func (m *mockContainerRepo) CountByUserID(ctx context.Context, u uuid.UUID) (int64, error) {
	a := m.Called(ctx, u)
	return int64(a.Int(0)), a.Error(1)
}
func (m *mockContainerRepo) SumPersistentGBByUserID(ctx context.Context, u uuid.UUID) (int, error) {
	a := m.Called(ctx, u)
	return a.Int(0), a.Error(1)
}
func (m *mockContainerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type mockRuntime struct{ mock.Mock }

func (m *mockRuntime) EnsureNamespace(ctx context.Context, ns string, q repository.ResourceQuotaSpec) error {
	return m.Called(ctx, ns, q).Error(0)
}
func (m *mockRuntime) Apply(ctx context.Context, c *entity.Container, d string) error {
	return m.Called(ctx, c, d).Error(0)
}
func (m *mockRuntime) Delete(ctx context.Context, ns, n string) error {
	return m.Called(ctx, ns, n).Error(0)
}
func (m *mockRuntime) Status(ctx context.Context, ns string, names []string) (map[string]repository.ContainerStatus, error) {
	a := m.Called(ctx, ns, names)
	return a.Get(0).(map[string]repository.ContainerStatus), a.Error(1)
}
func (m *mockRuntime) Exec(ctx context.Context, ns, n string, c, r uint16) (repository.TerminalSession, error) {
	return nil, errors.New("não usado")
}
func (m *mockRuntime) Logs(ctx context.Context, ns, n string, f bool) (io.ReadCloser, error) {
	return nil, errors.New("não usado")
}

func baseReq() usecase.CreateContainerRequest {
	return usecase.CreateContainerRequest{
		Name: "meu-pg", Image: "postgres:16", CPUMillis: 500, MemoryMB: 512,
		Port: 5432, Protocol: "tcp", Env: map[string]string{"POSTGRES_PASSWORD": "x"},
		PersistentGB: 5, MountPath: "/var/lib/postgresql/data", Role: "aluno",
	}
}

func TestCreate_TCPAllocatesNodePortAndAppliesQuota(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "lab.example.edu", "10.0.0.50")
	user := uuid.New()

	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *entity.Container) bool {
		return c.NodePort >= 30000 && c.NodePort <= 32767 && c.Namespace == entity.NamespaceFor(user)
	})).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, entity.NamespaceFor(user),
		repository.ResourceQuotaSpec{CPUMillis: 2000, MemoryMB: 4096, Pods: 5, StorageGB: 20}).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "lab.example.edu").Return(nil)
	rt.On("Status", mock.Anything, entity.NamespaceFor(user), []string{"meu-pg"}).
		Return(map[string]repository.ContainerStatus{"meu-pg": {Phase: "pending"}}, nil)

	view, err := uc.Create(context.Background(), user, baseReq())
	require.NoError(t, err)
	assert.Equal(t, "pending", view.Status.Phase)
	assert.Regexp(t, `^10\.0\.0\.50:3[0-2]\d{3}$`, view.ExternalURL)
	assert.Equal(t, "meu-pg:5432", view.InternalAddress)
	repo.AssertExpectations(t)
	rt.AssertExpectations(t)
}

func TestCreate_HTTPWithDomainUsesIngress(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "lab.example.edu", "10.0.0.50")
	user := uuid.New()
	req := baseReq()
	req.Name, req.Protocol, req.Port, req.PersistentGB = "site", "http", 80, 0

	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "site").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *entity.Container) bool { return c.NodePort == 0 })).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "lab.example.edu").Return(nil)
	rt.On("Status", mock.Anything, mock.Anything, mock.Anything).Return(map[string]repository.ContainerStatus{}, nil)

	view, err := uc.Create(context.Background(), user, req)
	require.NoError(t, err)
	assert.Equal(t, "http://site-"+entity.NamespaceFor(user)+".lab.example.edu", view.ExternalURL)
	assert.Equal(t, "unknown", view.Status.Phase, "sem entrada no mapa → unknown")
}

func TestCreate_HTTPWithoutDomainFallsBackToNodePort(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	req := baseReq()
	req.Protocol, req.Port = "http", 80
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *entity.Container) bool { return c.NodePort > 0 })).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "").Return(nil)
	rt.On("Status", mock.Anything, mock.Anything, mock.Anything).Return(map[string]repository.ContainerStatus{}, nil)
	view, err := uc.Create(context.Background(), user, req)
	require.NoError(t, err)
	assert.Contains(t, view.ExternalURL, "10.0.0.50:")
}

func TestCreate_RetriesOnNodePortCollision(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(errors.New(`duplicate key value violates unique constraint "ux_containers_node_port"`)).Once()
	repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "").Return(nil)
	rt.On("Status", mock.Anything, mock.Anything, mock.Anything).Return(map[string]repository.ContainerStatus{}, nil)
	_, err := uc.Create(context.Background(), user, baseReq())
	require.NoError(t, err)
	repo.AssertNumberOfCalls(t, "Create", 2)
}

func TestCreate_LimitErrors(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()

	req := baseReq()
	req.CPUMillis = 3000
	_, err := uc.Create(context.Background(), user, req)
	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)

	repo.On("CountByUserID", mock.Anything, user).Return(5, nil).Once()
	_, err = uc.Create(context.Background(), user, baseReq())
	assert.ErrorIs(t, err, usecase.ErrLimitExceeded, "5 containers já é o teto de aluno")

	repo.On("CountByUserID", mock.Anything, user).Return(0, nil).Once()
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(18, nil).Once()
	_, err = uc.Create(context.Background(), user, baseReq())
	assert.ErrorIs(t, err, usecase.ErrLimitExceeded, "18+5 > 20 GB")
}

func TestCreate_NameTakenAndInvalid(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(&entity.Container{}, nil)
	_, err := uc.Create(context.Background(), user, baseReq())
	assert.ErrorIs(t, err, usecase.ErrContainerNameTaken)

	req := baseReq()
	req.Name = "Inválido"
	_, err = uc.Create(context.Background(), user, req)
	assert.ErrorIs(t, err, usecase.ErrInvalidContainer)
}

func TestCreate_ApplyFailureRollsBack(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	repo.On("Delete", mock.Anything, mock.Anything).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "").Return(errors.New("boom"))
	_, err := uc.Create(context.Background(), user, baseReq())
	require.Error(t, err)
	repo.AssertCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestCreate_RuntimeNil(t *testing.T) {
	uc := usecase.NewContainerUseCase(new(mockContainerRepo), nil, "", "")
	_, err := uc.Create(context.Background(), uuid.New(), baseReq())
	assert.ErrorIs(t, err, repository.ErrRuntimeUnavailable)
}

func TestDelete_RuntimeFirstThenDB(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user, id := uuid.New(), uuid.New()
	c := &entity.Container{ID: id, UserID: user, Name: "x", Namespace: "u-1"}
	repo.On("FindByID", mock.Anything, id).Return(c, nil)
	rt.On("Delete", mock.Anything, "u-1", "x").Return(errors.New("down")).Once()
	err := uc.Delete(context.Background(), user, id)
	require.Error(t, err)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)

	rt.On("Delete", mock.Anything, "u-1", "x").Return(nil).Once()
	repo.On("Delete", mock.Anything, id).Return(nil)
	require.NoError(t, uc.Delete(context.Background(), user, id))
}

func TestGet_OtherUserIsNotFound(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(&entity.Container{ID: id, UserID: uuid.New()}, nil)
	_, err := uc.Get(context.Background(), uuid.New(), id)
	assert.ErrorIs(t, err, usecase.ErrContainerNotFound)
}
