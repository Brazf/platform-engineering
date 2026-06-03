# Domain Entities & Database Setup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Completar as entidades de domínio com campos corretos, adicionar métodos de validação, implementar os repositórios GORM com queries reais, consolidar as migrations em um único arquivo e expor `ConnectDatabase()` no config.

**Architecture:** Sem alterar o fluxo existente (Handler → UseCase → Repository). As mudanças são de completude: entidades ganham `UpdatedAt` e método `ValidateRole()`; repositórios saem de stubs para queries GORM reais; `config` absorve a responsabilidade de abrir e migrar o DB para manter `main.go` limpo.

**Tech Stack:** Go 1.22, GORM v2 + PostgreSQL driver, google/uuid.

---

## Delta: o que existe vs. o que o spec pede

| Arquivo | Estado atual | O que muda |
|---|---|---|
| `entity/user.go` | `CreatedAt` apenas, `RoleAdmin` extra, sem `ValidateRole()` | Adiciona `UpdatedAt`, remove `RoleAdmin`, adiciona `ValidateRole()` |
| `entity/vm.go` | Campos `CPU`, `Memory`, `Disk`; sem `OSTemplate`; sem `UpdatedAt` | Renomeia para `CPUCores`/`MemoryMB`/`DiskGB`, adiciona `OSTemplate` e `UpdatedAt` |
| `domain/repository/user_repository.go` | Faltam `Update` e `Delete` | Adiciona os dois métodos |
| `repository/user_repository.go` | Stubs `errors.New("not implemented")` | Queries GORM reais |
| `repository/vm_repository.go` | Stubs `errors.New("not implemented")` | Queries GORM reais |
| `config/config.go` | Apenas `DSN()` | Adiciona `ConnectDatabase()` com auto-migrate em dev |
| `cmd/server/main.go` | Chama `gorm.Open` diretamente | Delega para `cfg.ConnectDatabase()` |
| `migrations/` | Dois arquivos separados | Cria `001_initial.sql` unificado (os antigos são mantidos sem uso) |

## Atenção — renomear campos de `VM` quebra o compilador
Renomear `CPU → CPUCores`, `Memory → MemoryMB`, `Disk → DiskGB` afeta qualquer código que use esses campos diretamente. No scaffold atual, apenas os stubs dos repositórios referenciam `entity.VM` como tipo — nenhum campo é lido/escrito ainda. Mesmo assim, o plano atualiza os dois arquivos de repo na mesma task da entidade para garantir que `go build ./...` passe após cada commit.

---

### Task 1: Entidade User

**Files:**
- Modify: `backend/internal/domain/entity/user.go`

- [ ] Reescrever o arquivo com `UpdatedAt`, sem `RoleAdmin`, com `ValidateRole()`:

```go
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
	PasswordHash string    `gorm:"not null"`
	Role         UserRole  `gorm:"type:varchar(20);not null;default:'aluno'"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ValidateRole retorna true se o papel atribuído ao usuário é permitido na plataforma.
func (u *User) ValidateRole() bool {
	return u.Role == RoleAluno || u.Role == RolePesquisador
}
```

- [ ] Verificar compilação: `cd backend && go build ./internal/domain/...`
  Expected: sem erros

- [ ] Commit: `git add backend/internal/domain/entity/user.go && git commit -m "feat(entity): add UpdatedAt, remove admin role, add ValidateRole"`

---

### Task 2: Entidade VM

**Files:**
- Modify: `backend/internal/domain/entity/vm.go`

- [ ] Reescrever com campos renomeados, `OSTemplate` e `UpdatedAt`:

```go
package entity

import (
	"time"

	"github.com/google/uuid"
)

type VMStatus string

const (
	VMStatusCreating VMStatus = "creating"
	VMStatusRunning  VMStatus = "running"
	VMStatusStopped  VMStatus = "stopped"
	VMStatusError    VMStatus = "error"
)

type VM struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	ProxmoxVMID int       `gorm:"not null"`
	Name        string    `gorm:"not null"`
	CPUCores    int       `gorm:"column:cpu_cores;not null"`
	MemoryMB    int       `gorm:"column:memory_mb;not null"`
	DiskGB      int       `gorm:"column:disk_gb;not null"`
	OSTemplate  string    `gorm:"not null"`
	Node        string    `gorm:"not null"`
	Status      VMStatus  `gorm:"type:varchar(20);not null;default:'creating'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

- [ ] Verificar compilação: `go build ./...`
  Expected: sem erros (os stubs de repo não leem campos de `VM`)

- [ ] Commit: `git add backend/internal/domain/entity/vm.go && git commit -m "feat(entity): rename VM fields, add OSTemplate and UpdatedAt"`

---

### Task 3: Interface UserRepository — adicionar Update e Delete

**Files:**
- Modify: `backend/internal/domain/repository/user_repository.go`

- [ ] Adicionar `Update` e `Delete` à interface:

```go
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}
```

- [ ] Verificar compilação: `go build ./...`
  Expected: FALHA — `userRepository` no `internal/repository/` não implementa `Update` e `Delete` ainda. Isso é esperado; será corrigido na Task 4.

---

### Task 4: Implementação GORM de UserRepository

**Files:**
- Modify: `backend/internal/repository/user_repository.go`

- [ ] Substituir stubs por queries GORM reais (e adicionar `Update`/`Delete` para satisfazer a interface):

```go
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	domainrepo "github.com/proxlab/backend/internal/domain/repository"
	"gorm.io/gorm"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) domainrepo.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func (r *userRepository) Update(ctx context.Context, user *entity.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.User{}, "id = ?", id).Error
}
```

- [ ] Verificar compilação: `go build ./...`
  Expected: sem erros

- [ ] Commit: `git add backend/internal/domain/repository/user_repository.go backend/internal/repository/user_repository.go && git commit -m "feat(repository): implement UserRepository with GORM queries"`

---

### Task 5: Implementação GORM de VMRepository

**Files:**
- Modify: `backend/internal/repository/vm_repository.go`

- [ ] Substituir stubs por queries GORM reais:

```go
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	domainrepo "github.com/proxlab/backend/internal/domain/repository"
	"gorm.io/gorm"
)

type vmRepository struct {
	db *gorm.DB
}

func NewVMRepository(db *gorm.DB) domainrepo.VMRepository {
	return &vmRepository{db: db}
}

func (r *vmRepository) Create(ctx context.Context, vm *entity.VM) error {
	return r.db.WithContext(ctx).Create(vm).Error
}

func (r *vmRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.VM, error) {
	var vm entity.VM
	err := r.db.WithContext(ctx).First(&vm, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &vm, err
}

func (r *vmRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.VM, error) {
	var vms []entity.VM
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&vms).Error
	return vms, err
}

func (r *vmRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status entity.VMStatus) error {
	return r.db.WithContext(ctx).
		Model(&entity.VM{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *vmRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.VM{}, "id = ?", id).Error
}
```

- [ ] Verificar compilação: `go build ./...`
  Expected: sem erros

- [ ] Commit: `git add backend/internal/repository/vm_repository.go && git commit -m "feat(repository): implement VMRepository with GORM queries"`

---

### Task 6: ConnectDatabase no Config

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] Adicionar campo `AppEnv` e função `ConnectDatabase()` com auto-migrate em dev:

```go
// Adicionar ao struct Config:
AppEnv string  // "development" | "production"

// Em Load(), dentro do bloco de campos:
AppEnv: getEnv("APP_ENV", "development"),

// Adicionar no final do arquivo:

func (c *Config) ConnectDatabase() (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(c.DSN()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}

	if c.AppEnv == "development" {
		if err := db.AutoMigrate(&entity.User{}, &entity.VM{}); err != nil {
			return nil, fmt.Errorf("erro no auto-migrate: %w", err)
		}
	}

	return db, nil
}
```

Imports adicionados: `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/proxlab/backend/internal/domain/entity`

- [ ] Verificar compilação: `go build ./...`
  Expected: sem erros

- [ ] Commit: `git add backend/internal/config/config.go && git commit -m "feat(config): add ConnectDatabase with dev auto-migrate"`

---

### Task 7: Simplificar main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] Substituir o bloco de `gorm.Open` por `cfg.ConnectDatabase()`:

```go
// Remover import: "gorm.io/driver/postgres" e "gorm.io/gorm"
// Substituir:
db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
if err != nil {
    log.Fatalf("erro ao conectar ao banco de dados: %v", err)
}
// Por:
db, err := cfg.ConnectDatabase()
if err != nil {
    log.Fatalf("%v", err)
}
```

- [ ] Verificar compilação: `go build ./...`
  Expected: sem erros

- [ ] Commit: `git add backend/cmd/server/main.go && git commit -m "refactor(main): delegate DB setup to cfg.ConnectDatabase"`

---

### Task 8: Migration SQL unificada

**Files:**
- Create: `backend/migrations/001_initial.sql`

- [ ] Criar o arquivo com ambas as tabelas em SQL puro PostgreSQL:

```sql
BEGIN;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Tipos enumerados
DO $$ BEGIN
  CREATE TYPE user_role AS ENUM ('aluno', 'pesquisador');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE vm_status AS ENUM ('creating', 'running', 'stopped', 'error');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Tabela de usuários
CREATE TABLE IF NOT EXISTS users (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          user_role    NOT NULL DEFAULT 'aluno',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Tabela de máquinas virtuais
CREATE TABLE IF NOT EXISTS vms (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    proxmox_vm_id INTEGER      NOT NULL,
    name          VARCHAR(255) NOT NULL,
    cpu_cores     INTEGER      NOT NULL,
    memory_mb     INTEGER      NOT NULL,
    disk_gb       INTEGER      NOT NULL,
    os_template   VARCHAR(255) NOT NULL DEFAULT '',
    node          VARCHAR(100) NOT NULL,
    status        vm_status    NOT NULL DEFAULT 'creating',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vms_user_id ON vms(user_id);

COMMIT;
```

- [ ] Commit: `git add backend/migrations/001_initial.sql && git commit -m "feat(migrations): add unified initial SQL migration"`

---

### Task 9: Verificação final

- [ ] `go build ./...` — sem erros
- [ ] `go vet ./...` — sem warnings
- [ ] Confirmar que `ValidateRole()` existe: `grep -r "ValidateRole" backend/`
- [ ] Confirmar campos renomeados: `grep -r "CPUCores\|MemoryMB\|DiskGB\|OSTemplate" backend/internal/domain/`
- [ ] Confirmar `ConnectDatabase` no config: `grep -r "ConnectDatabase" backend/internal/config/`
