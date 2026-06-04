# ProxLab — CLAUDE.md

## Visão Geral do Projeto

**ProxLab** é uma plataforma web para abstração de infraestrutura virtualizada, permitindo que alunos e pesquisadores criem e gerenciem Máquinas Virtuais (VMs) e Containers LXC no Proxmox VE por meio de uma interface web simplificada, sem necessidade de conhecimento técnico avançado.

### Stack Tecnológica

| Camada       | Tecnologia                          |
|--------------|-------------------------------------|
| Frontend     | (já existente — não alterar estrutura visual) |
| Backend      | Go (Golang) + Gin + GORM            |
| Banco de Dados | PostgreSQL                        |
| Contêinerização | Docker + Docker Compose          |
| Virtualização | Proxmox VE (API REST)              |
| Terminal Web | xterm.js + WebSocket (noVNC / SPICE para VMs) |

### Arquitetura

```
proxlab/
├── frontend/          # Frontend já existente (não alterar estrutura)
├── backend/           # API em Go (Gin + GORM)
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── config/        # Configurações (env, Proxmox, DB)
│   │   ├── domain/        # Entidades e interfaces (Clean Architecture)
│   │   │   ├── entity/
│   │   │   └── repository/
│   │   ├── usecase/       # Lógica de negócio
│   │   ├── handler/       # Controllers HTTP (Gin)
│   │   ├── repository/    # Implementações de repositório (GORM)
│   │   ├── middleware/     # Auth JWT, CORS, logging
│   │   ├── proxmox/       # Cliente da API do Proxmox VE
│   │   └── websocket/     # Terminal WebSocket (para VMs)
│   ├── migrations/        # Scripts SQL de migração
│   ├── Dockerfile
│   └── go.mod
├── docker-compose.yml
└── CLAUDE.md
```

---

## Papéis de Usuário

| Role         | Descrição                                                              |
|--------------|------------------------------------------------------------------------|
| `aluno`      | Pode criar VMs e containers dentro de limites pré-definidos            |
| `pesquisador`| Acesso ampliado; pode definir configurações mais avançadas             |

> As permissões no Proxmox já estão configuradas externamente. O backend apenas orquestra chamadas à API do Proxmox com as credenciais do papel correto.

---

## Regras de Arquitetura

### Clean Architecture + MVC

- **Entities** (`internal/domain/entity/`): structs puras de domínio, sem dependência de framework
- **Repositories** (`internal/domain/repository/`): interfaces — nunca implementações
- **Use Cases** (`internal/usecase/`): lógica de negócio; dependem apenas de interfaces
- **Handlers** (`internal/handler/`): recebem HTTP, chamam use cases, retornam JSON
- **Repository Impl** (`internal/repository/`): implementações GORM das interfaces

**Fluxo obrigatório:**
```
Handler → Use Case → Repository Interface → Repository Impl (GORM) → PostgreSQL
Handler → Use Case → ProxmoxClient Interface → ProxmoxClient Impl → Proxmox VE API
```

**Nunca** chame GORM diretamente em handlers ou use cases. Sempre via interface de repositório.

### Convenções de Código Go

- Todos os erros devem ser tratados explicitamente — `if err != nil`
- Nomeação em inglês no código; comentários podem ser em português
- Variáveis de ambiente carregadas via struct de `Config` em `internal/config/`
- Nenhuma lógica de negócio em `main.go` — apenas bootstrap (DI, roteamento, servidor)
- Use `context.Context` em todas as chamadas ao banco e à API do Proxmox

---

## Módulos Funcionais

### 1. Autenticação

- JWT (access token + refresh token)
- Login por email/senha (sem OAuth por enquanto)
- Middleware que injeta `userID` e `role` no contexto Gin
- Rotas protegidas por role: `/api/student/*` e `/api/researcher/*`

### 2. Máquinas Virtuais (fase atual)

**Criação de VM:**
- Endpoint: `POST /api/vms`
- Payload: `{ name, cpu_cores, memory_mb, disk_gb, os_template, node }`
- O backend chama a API REST do Proxmox para criar a VM
- Retorna: `{ vm_id, status, websocket_url }`

**Terminal interativo:**
- Quando a VM está rodando, o frontend abre um WebSocket com o backend
- O backend faz proxy do terminal SPICE/VNC da VM via Proxmox API
- O frontend renderiza o terminal com `xterm.js`

**Listagem e status:**
- `GET /api/vms` — lista VMs do usuário
- `GET /api/vms/:id` — status atual (running, stopped, creating)
- `POST /api/vms/:id/start` — iniciar VM
- `POST /api/vms/:id/stop` — parar VM
- `DELETE /api/vms/:id` — deletar VM

### 3. Containers LXC (fase futura — não implementar agora)

- Criação via imagem Docker Hub ou template local
- Listagem, start/stop, logs
- Retorno de endereço de acesso (IP interno da rede da faculdade)

---

## Integração com o Proxmox VE

### Cliente Proxmox

- Localização: `internal/proxmox/client.go`
- Autenticação: ticket de sessão (API token preferido)
- Base URL configurável via env: `PROXMOX_HOST`, `PROXMOX_TOKEN_ID`, `PROXMOX_TOKEN_SECRET`
- TLS: aceitar certificado self-signed via `InsecureSkipVerify: true` (ambiente interno da faculdade)

### Chamadas Principais

```go
// Criar VM
POST https://<proxmox>/api2/json/nodes/<node>/qemu

// Listar VMs
GET  https://<proxmox>/api2/json/nodes/<node>/qemu

// Status de VM
GET  https://<proxmox>/api2/json/nodes/<node>/qemu/<vmid>/status/current

// Iniciar / Parar
POST https://<proxmox>/api2/json/nodes/<node>/qemu/<vmid>/status/start
POST https://<proxmox>/api2/json/nodes/<node>/qemu/<vmid>/status/stop

// Console VNC/SPICE
POST https://<proxmox>/api2/json/nodes/<node>/qemu/<vmid>/vncproxy
```

---

## Docker Compose

```yaml
# Serviços esperados no docker-compose.yml
services:
  backend:    # Go API — porta 8080
  postgres:   # PostgreSQL 15 — porta 5432
  frontend:   # Frontend estático servido por nginx — porta 3000
```

Variáveis de ambiente obrigatórias do backend:

```env
DB_HOST=postgres
DB_PORT=5432
DB_USER=proxlab
DB_PASSWORD=...
DB_NAME=proxlab

JWT_SECRET=...
JWT_EXPIRY_HOURS=24

PROXMOX_HOST=https://192.168.x.x:8006
PROXMOX_NODE=pve
PROXMOX_TOKEN_ID=user@pam!token
PROXMOX_TOKEN_SECRET=...

SERVER_PORT=8080
```

---

## Banco de Dados (PostgreSQL + GORM)

### Tabelas principais

```
users           — id, email, password_hash, role, created_at
vms             — id, user_id, proxmox_vm_id, name, cpu, memory, disk, status, node, created_at
containers      — (fase futura)
```

- GORM com auto-migrate apenas em desenvolvimento
- Em produção usar migrations SQL versionadas em `migrations/`
- UUIDs como chave primária (`github.com/google/uuid`)

---

## Regras para Claude ao Trabalhar neste Projeto

### O que NUNCA fazer

- ❌ Alterar estrutura visual ou CSS do frontend existente
- ❌ Adicionar lógica de negócio em `main.go`
- ❌ Chamar GORM diretamente em handlers
- ❌ Hardcodar IPs, senhas ou tokens — sempre via variáveis de ambiente
- ❌ Retornar erros internos do Proxmox diretamente ao frontend — abstrair sempre
- ❌ Ignorar tratamento de erros em Go

### O que SEMPRE fazer

- ✅ Seguir o fluxo Handler → Use Case → Repository/ProxmoxClient
- ✅ Documentar endpoints com comentários de exemplo de request/response
- ✅ Usar `context.Context` em operações de I/O
- ✅ Validar payload de entrada nos handlers antes de chamar use cases
- ✅ Logar erros relevantes sem expor detalhes ao cliente
- ✅ Testar a lógica de use cases com interfaces mockadas

### Ao adicionar uma nova feature

1. Criar/atualizar a entidade em `internal/domain/entity/`
2. Definir a interface de repositório em `internal/domain/repository/`
3. Implementar o use case em `internal/usecase/`
4. Implementar o handler HTTP em `internal/handler/`
5. Implementar o repositório GORM em `internal/repository/`
6. Registrar a rota em `cmd/server/main.go`
7. Atualizar `docker-compose.yml` se necessário

---

## Fases do Projeto

| Fase | Escopo                                     | Status    |
|------|--------------------------------------------|-----------|
| 1    | Autenticação (JWT) + CRUD de usuários      | ✅ concluído |
| 2    | Criação e listagem de VMs no Proxmox       | ✅ concluído |
| 3    | Terminal web interativo para VMs (xterm.js) | ✅ concluído |
| 4    | Containers LXC / Docker Hub                | 🔲 Futura  |
| 5    | Dashboard com uso de recursos              | 🔲 Futura  |