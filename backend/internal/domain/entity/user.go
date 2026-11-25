package entity

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleAluno       UserRole = "aluno"
	RolePesquisador UserRole = "pesquisador"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	Name         string    `gorm:"type:varchar(120);not null;default:''"`
	PasswordHash string    `gorm:"not null"`
	Role         UserRole  `gorm:"type:varchar(20);not null;default:'aluno'"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ValidateRole retorna true se o papel atribuído ao usuário é permitido na plataforma.
func (u *User) ValidateRole() bool {
	return u.Role == RoleAluno || u.Role == RolePesquisador
}
