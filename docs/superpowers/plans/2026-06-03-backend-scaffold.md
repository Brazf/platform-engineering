# Backend Scaffold (Go — Clean Architecture) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Criar o scaffold completo do backend Go com Clean Architecture + MVC que compila sem erros, sem implementar nenhuma feature de negócio.

**Architecture:** Clean Architecture com separação Handler → UseCase → RepositoryInterface → RepositoryImpl. Entities puras sem dependência de framework. Proxmox client e WebSocket são ports separados.

**Tech Stack:** Go 1.22, Gin, GORM + PostgreSQL driver, golang-jwt/jwt/v5, google/uuid, joho/godotenv, gorilla/websocket.

---

## File Map

| Arquivo | Responsabilidade |
|---|---|
| `backend/go.mod` | Módulo e dependências |
| `backend/cmd/server/main.go` | Bootstrap: config → DB → rotas → servidor |
| `backend/internal/config/config.go` | Struct Config + Load() via env |
| `backend/internal/domain/entity/user.go` | Entidade User (pura) |
| `backend/internal/domain/entity/vm.go` | Entidade VM (pura) |
| `backend/internal/domain/repository/user_repository.go` | Interface UserRepository |
| `backend/internal/domain/repository/vm_repository.go` | Interface VMRepository |
| `backend/internal/usecase/auth_usecase.go` | Stub AuthUseCase |
| `backend/internal/usecase/vm_usecase.go` | Stub VMUseCase |
| `backend/internal/handler/auth_handler.go` | Stub AuthHandler |
| `backend/internal/handler/vm_handler.go` | Stub VMHandler |
| `backend/internal/repository/user_repository.go` | Impl GORM UserRepository |
| `backend/internal/repository/vm_repository.go` | Impl GORM VMRepository |
| `backend/internal/middleware/auth.go` | Stub JWT middleware |
| `backend/internal/middleware/cors.go` | Stub CORS middleware |
| `backend/internal/proxmox/client.go` | Stub ProxmoxClient |
| `backend/internal/websocket/terminal.go` | Stub terminal WS handler |
| `backend/migrations/001_create_users.sql` | DDL tabela users |
| `backend/migrations/002_create_vms.sql` | DDL tabela vms |
| `backend/Dockerfile` | Multi-stage build |

---

### Task 1: go.mod com todas as dependências

**Files:**
- Create: `backend/go.mod`

- [ ] Criar go.mod com módulo `github.com/proxlab/backend` e todas as dependências listadas no CLAUDE.md
- [ ] Criar go.sum executando `go mod tidy` após criação dos arquivos Go
- [ ] Verificar que `go build ./...` passa sem erro

---

### Task 2: Config

**Files:**
- Create: `backend/internal/config/config.go`

- [ ] Criar struct Config com todos os campos de env do CLAUDE.md (DB, JWT, Proxmox, Server)
- [ ] Implementar função `Load() (*Config, error)` com godotenv + os.Getenv

---

### Task 3: Domain Entities

**Files:**
- Create: `backend/internal/domain/entity/user.go`
- Create: `backend/internal/domain/entity/vm.go`

- [ ] Criar User com campos: ID (UUID), Email, PasswordHash, Role, CreatedAt
- [ ] Criar VM com campos: ID (UUID), UserID, ProxmoxVMID, Name, CPU, Memory, Disk, Status, Node, CreatedAt

---

### Task 4: Repository Interfaces

**Files:**
- Create: `backend/internal/domain/repository/user_repository.go`
- Create: `backend/internal/domain/repository/vm_repository.go`

- [ ] Definir interface UserRepository com métodos: Create, FindByEmail, FindByID
- [ ] Definir interface VMRepository com métodos: Create, FindByUserID, FindByID, UpdateStatus, Delete

---

### Task 5: Use Cases (stubs)

**Files:**
- Create: `backend/internal/usecase/auth_usecase.go`
- Create: `backend/internal/usecase/vm_usecase.go`

- [ ] Criar structs com dependências injetadas via interface (AuthUseCase, VMUseCase)
- [ ] Stubs: métodos com assinatura correta retornando `nil, errors.New("not implemented")`

---

### Task 6: Handlers (stubs)

**Files:**
- Create: `backend/internal/handler/auth_handler.go`
- Create: `backend/internal/handler/vm_handler.go`

- [ ] Criar structs Handler com UseCase injetado
- [ ] Stubs de handler Gin: `func (h *XHandler) Method(c *gin.Context) { c.JSON(501, ...) }`

---

### Task 7: Repository Implementations (GORM)

**Files:**
- Create: `backend/internal/repository/user_repository.go`
- Create: `backend/internal/repository/vm_repository.go`

- [ ] Implementar interfaces de repositório usando `*gorm.DB`
- [ ] Cada método retorna `errors.New("not implemented")` por enquanto

---

### Task 8: Middleware stubs

**Files:**
- Create: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/cors.go`

- [ ] JWT middleware: `func Auth() gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }`
- [ ] CORS middleware: configuração básica Gin

---

### Task 9: Proxmox Client stub

**Files:**
- Create: `backend/internal/proxmox/client.go`

- [ ] Struct ProxmoxClient com Config
- [ ] Interface ProxmoxClientInterface com métodos VM (CreateVM, ListVMs, GetVMStatus, StartVM, StopVM, DeleteVM, GetVNCProxy)
- [ ] NewClient() retorna *ProxmoxClient

---

### Task 10: WebSocket stub

**Files:**
- Create: `backend/internal/websocket/terminal.go`

- [ ] Struct TerminalHandler com ProxmoxClient
- [ ] HandleTerminal(c *gin.Context) como stub

---

### Task 11: main.go bootstrap

**Files:**
- Create: `backend/cmd/server/main.go`

- [ ] Carregar config, abrir DB com GORM, instanciar repositórios → use cases → handlers, registrar rotas, iniciar servidor

---

### Task 12: Migrations SQL

**Files:**
- Create: `backend/migrations/001_create_users.sql`
- Create: `backend/migrations/002_create_vms.sql`

- [ ] DDL para tabela `users`
- [ ] DDL para tabela `vms`

---

### Task 13: Dockerfile multi-stage

**Files:**
- Create: `backend/Dockerfile`

- [ ] Stage `builder`: `golang:1.22-alpine`, `go build -o /app ./cmd/server`
- [ ] Stage `runtime`: `alpine:3.19`, copia binário, expõe porta 8080, CMD

---

### Task 14: Verificação final

- [ ] `go mod tidy` sem erros
- [ ] `go build ./...` compila com sucesso
- [ ] `go vet ./...` sem warnings
