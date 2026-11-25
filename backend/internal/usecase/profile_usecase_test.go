package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func userWithPassword(t *testing.T, password string) *entity.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	assert.NoError(t, err)
	return &entity.User{ID: uuid.New(), Email: "a@b.com", PasswordHash: string(hash), Role: entity.RoleAluno}
}

// Cenário: usuário do token não existe mais — ErrUserNotFound.
func TestGetProfile_NotFound(t *testing.T) {
	repo := new(mockUserRepo)
	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(nil, nil)

	_, err := usecase.NewProfileUseCase(repo).Get(context.Background(), id)

	assert.ErrorIs(t, err, usecase.ErrUserNotFound)
}

// Cenário: nome é salvo sem espaços nas pontas; e-mail e perfil não mudam.
func TestUpdateName_TrimsAndSaves(t *testing.T) {
	repo := new(mockUserRepo)
	u := userWithPassword(t, "senha1234")
	repo.On("FindByID", mock.Anything, u.ID).Return(u, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(x *entity.User) bool {
		return x.Name == "Maria Silva" && x.Email == "a@b.com" && x.Role == entity.RoleAluno
	})).Return(nil)

	got, err := usecase.NewProfileUseCase(repo).UpdateName(context.Background(), u.ID, "  Maria Silva ")

	assert.NoError(t, err)
	assert.Equal(t, "Maria Silva", got.Name)
	repo.AssertExpectations(t)
}

// Cenário: nome maior que o limite — ErrInvalidName, nada é salvo.
func TestUpdateName_TooLong(t *testing.T) {
	repo := new(mockUserRepo)
	long := make([]byte, 121)
	for i := range long {
		long[i] = 'a'
	}

	_, err := usecase.NewProfileUseCase(repo).UpdateName(context.Background(), uuid.New(), string(long))

	assert.ErrorIs(t, err, usecase.ErrInvalidName)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// Cenário: senha atual errada — ErrWrongPassword, nada é salvo.
func TestChangePassword_WrongCurrent(t *testing.T) {
	repo := new(mockUserRepo)
	u := userWithPassword(t, "senha1234")
	repo.On("FindByID", mock.Anything, u.ID).Return(u, nil)

	err := usecase.NewProfileUseCase(repo).ChangePassword(context.Background(), u.ID, "errada123", "novasenha1")

	assert.ErrorIs(t, err, usecase.ErrWrongPassword)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// Cenário: nova senha curta — ErrWeakPassword antes de consultar o banco.
func TestChangePassword_Weak(t *testing.T) {
	repo := new(mockUserRepo)

	err := usecase.NewProfileUseCase(repo).ChangePassword(context.Background(), uuid.New(), "senha1234", "curta")

	assert.ErrorIs(t, err, usecase.ErrWeakPassword)
	repo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

// Cenário: senha atual correta — novo hash salvo e confere com a nova senha.
func TestChangePassword_OK(t *testing.T) {
	repo := new(mockUserRepo)
	u := userWithPassword(t, "senha1234")
	repo.On("FindByID", mock.Anything, u.ID).Return(u, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(x *entity.User) bool {
		return bcrypt.CompareHashAndPassword([]byte(x.PasswordHash), []byte("novasenha1")) == nil
	})).Return(nil)

	err := usecase.NewProfileUseCase(repo).ChangePassword(context.Background(), u.ID, "senha1234", "novasenha1")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}
