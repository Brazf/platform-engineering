package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"golang.org/x/crypto/bcrypt"
)

const maxNameLen = 120

var (
	ErrUserNotFound  = errors.New("usuário não encontrado")
	ErrInvalidName   = errors.New("nome deve ter no máximo 120 caracteres")
	ErrWrongPassword = errors.New("senha atual incorreta")
	ErrWeakPassword  = errors.New("a nova senha precisa ter pelo menos 8 caracteres")
)

// ProfileUseCase cuida dos dados do próprio usuário logado ("Meu perfil").
// E-mail e perfil não são alteráveis aqui.
type ProfileUseCase struct {
	userRepo repository.UserRepository
}

func NewProfileUseCase(repo repository.UserRepository) *ProfileUseCase {
	return &ProfileUseCase{userRepo: repo}
}

func (uc *ProfileUseCase) Get(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar usuário: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// UpdateName grava o nome exibido (vazio é permitido).
func (uc *ProfileUseCase) UpdateName(ctx context.Context, userID uuid.UUID, name string) (*entity.User, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxNameLen {
		return nil, ErrInvalidName
	}
	user, err := uc.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	user.Name = name
	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("erro ao salvar nome: %w", err)
	}
	return user, nil
}

// ChangePassword troca a senha exigindo a senha atual.
func (uc *ProfileUseCase) ChangePassword(ctx context.Context, userID uuid.UUID, current, next string) error {
	if len(next) < 8 {
		return ErrWeakPassword
	}
	user, err := uc.Get(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)); err != nil {
		return ErrWrongPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("erro ao processar senha: %w", err)
	}
	user.PasswordHash = string(hash)
	if err := uc.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("erro ao salvar senha: %w", err)
	}
	return nil
}
