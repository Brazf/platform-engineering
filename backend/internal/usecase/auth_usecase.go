package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailAlreadyInUse  = errors.New("email já está em uso")
	ErrInvalidCredentials = errors.New("email ou senha inválidos")
	ErrInvalidRole        = errors.New("perfil inválido: use 'aluno' ou 'pesquisador'")
)

// LoginOutput agrupa token JWT e dados do usuário autenticado.
type LoginOutput struct {
	Token string
	User  *entity.User
}

type jwtClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type AuthUseCase struct {
	userRepo  repository.UserRepository
	jwtSecret string
	expiryH   int
}

func NewAuthUseCase(repo repository.UserRepository, jwtSecret string, expiryH int) *AuthUseCase {
	return &AuthUseCase{userRepo: repo, jwtSecret: jwtSecret, expiryH: expiryH}
}

// Register cria um novo usuário com senha hasheada via bcrypt.
func (uc *AuthUseCase) Register(email, password, role string) (*entity.User, error) {
	user := &entity.User{Role: entity.UserRole(role)}
	if !user.ValidateRole() {
		return nil, ErrInvalidRole
	}

	existing, err := uc.userRepo.FindByEmail(context.Background(), email)
	if err != nil {
		return nil, fmt.Errorf("erro ao verificar email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyInUse
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("erro ao processar senha: %w", err)
	}

	user.ID = uuid.New()
	user.Email = email
	user.PasswordHash = string(hash)

	if err := uc.userRepo.Create(context.Background(), user); err != nil {
		return nil, fmt.Errorf("erro ao criar usuário: %w", err)
	}

	return user, nil
}

// Login verifica credenciais e retorna token JWT assinado e dados do usuário.
func (uc *AuthUseCase) Login(email, password string) (*LoginOutput, error) {
	user, err := uc.userRepo.FindByEmail(context.Background(), email)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar usuário: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	expiry := time.Now().Add(time.Duration(uc.expiryH) * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		UserID: user.ID.String(),
		Role:   string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	})

	signed, err := token.SignedString([]byte(uc.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("erro ao assinar token: %w", err)
	}

	return &LoginOutput{Token: signed, User: user}, nil
}
