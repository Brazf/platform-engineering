package usecase_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"

	"context"
)

// ─── Mock: UserRepository ────────────────────────────────────────────────────

type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) Create(ctx context.Context, user *entity.User) error {
	return m.Called(ctx, user).Error(0)
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.User), args.Error(1)
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.User), args.Error(1)
}

func (m *mockUserRepo) List(ctx context.Context) ([]entity.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]entity.User), args.Error(1)
}

func (m *mockUserRepo) Update(ctx context.Context, user *entity.User) error {
	return m.Called(ctx, user).Error(0)
}

func (m *mockUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newAuthUC(repo *mockUserRepo) *usecase.AuthUseCase {
	return usecase.NewAuthUseCase(repo, "test-secret", 24)
}

// ─── Testes: Register ────────────────────────────────────────────────────────

// Cenário: email já cadastrado — deve retornar ErrEmailAlreadyInUse.
func TestRegister_DuplicateEmail(t *testing.T) {
	repo := new(mockUserRepo)
	existing := &entity.User{ID: uuid.New(), Email: "a@b.com", Role: entity.RoleAluno}
	repo.On("FindByEmail", mock.Anything, "a@b.com").Return(existing, nil)

	uc := newAuthUC(repo)
	_, err := uc.Register("a@b.com", "senha123", "aluno")

	assert.ErrorIs(t, err, usecase.ErrEmailAlreadyInUse)
	repo.AssertExpectations(t)
}

// Cenário: role inválido — deve ser rejeitado antes de consultar o banco.
func TestRegister_InvalidRole(t *testing.T) {
	repo := new(mockUserRepo)

	uc := newAuthUC(repo)
	_, err := uc.Register("a@b.com", "senha123", "admin")

	assert.ErrorIs(t, err, usecase.ErrInvalidRole)
	repo.AssertNotCalled(t, "FindByEmail")
}

// ─── Testes: Login ───────────────────────────────────────────────────────────

// Cenário: senha incorreta — deve retornar ErrInvalidCredentials.
func TestLogin_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correta"), bcrypt.MinCost)
	user := &entity.User{
		ID:           uuid.New(),
		Email:        "a@b.com",
		PasswordHash: string(hash),
		Role:         entity.RoleAluno,
	}

	repo := new(mockUserRepo)
	repo.On("FindByEmail", mock.Anything, "a@b.com").Return(user, nil)

	uc := newAuthUC(repo)
	_, err := uc.Login("a@b.com", "errada")

	assert.ErrorIs(t, err, usecase.ErrInvalidCredentials)
	repo.AssertExpectations(t)
}

// Cenário: usuário não encontrado — deve retornar ErrInvalidCredentials (não revela que o email não existe).
func TestLogin_UserNotFound(t *testing.T) {
	repo := new(mockUserRepo)
	repo.On("FindByEmail", mock.Anything, "nao@existe.com").Return(nil, nil)

	uc := newAuthUC(repo)
	_, err := uc.Login("nao@existe.com", "qualquer")

	assert.ErrorIs(t, err, usecase.ErrInvalidCredentials)
}

// Cenário: credenciais válidas — deve retornar JWT não-vazio e os dados do usuário.
func TestLogin_ValidCredentials(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("senha123"), bcrypt.MinCost)
	user := &entity.User{
		ID:           uuid.New(),
		Email:        "a@b.com",
		PasswordHash: string(hash),
		Role:         entity.RolePesquisador,
	}

	repo := new(mockUserRepo)
	repo.On("FindByEmail", mock.Anything, "a@b.com").Return(user, nil)

	uc := newAuthUC(repo)
	out, err := uc.Login("a@b.com", "senha123")

	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.NotEmpty(t, out.Token, "JWT não deve ser vazio")
	assert.Equal(t, user.ID, out.User.ID)
}

// Cenário: erro inesperado do repositório — deve propagar o erro.
func TestLogin_RepositoryError(t *testing.T) {
	repo := new(mockUserRepo)
	repo.On("FindByEmail", mock.Anything, "a@b.com").Return(nil, errors.New("db down"))

	uc := newAuthUC(repo)
	_, err := uc.Login("a@b.com", "senha123")

	assert.Error(t, err)
	assert.NotErrorIs(t, err, usecase.ErrInvalidCredentials)
}
