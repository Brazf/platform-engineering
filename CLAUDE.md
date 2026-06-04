# ProxLab — CLAUDE.md

Plataforma web para criação de VMs e Containers no Proxmox VE via interface simplificada.
Usuários: `aluno` (limites menores) e `pesquisador` (limites maiores). Permissões já configuradas no Proxmox externamente.

## Stack
- Backend: Go + Gin + GORM | DB: PostgreSQL | Infra: Docker Compose | Virtualização: Proxmox VE API REST

## Estrutura
```
backend/
├── cmd/server/main.go          # apenas bootstrap
├── internal/
│   ├── config/                 # variáveis de ambiente
│   ├── domain/entity/          # structs de domínio puras
│   ├── domain/repository/      # interfaces (nunca implementações)
│   ├── usecase/                # lógica de negócio
│   ├── handler/                # controllers HTTP (Gin)
│   ├── repository/             # implementações GORM
│   ├── middleware/             # JWT, CORS
│   ├── proxmox/                # cliente API Proxmox
│   └── websocket/              # proxy terminal VMs
└── migrations/
```

## Fluxo obrigatório
Handler → UseCase → Repository Interface → GORM → PostgreSQL
Handler → UseCase → ProxmoxClient Interface → Proxmox API

## Proxmox
- Auth: API Token via header `Authorization: PVEAPIToken=<id>=<secret>`
- TLS: `InsecureSkipVerify: true`
- Node: `pve01`
- Env: `PROXMOX_HOST`, `PROXMOX_NODE`, `PROXMOX_TOKEN_ID`, `PROXMOX_TOKEN_SECRET`
- WebSocket termproxy: Backend → Proxmox usa `Subprotocols: []string{"binary"}`

## Autenticação
- JWT salvo no localStorage com chave `"token"`
- Todas as rotas /api/* exigem `Authorization: Bearer <token>`
- WebSocket aceita `?token=` como query param
- 401 → redirecionar para /login

## Regras
- Nunca: lógica em main.go, GORM em handlers, hardcode de credenciais, expor erros internos do Proxmox, alterar CSS/layout do frontend
- Sempre: `if err != nil`, `context.Context` em I/O, validar payload nos handlers, UUIDs como PK

## Fases
1. Auth JWT + usuários ← ✅ concluído
2. CRUD de VMs no Proxmox ← ✅ concluído
3. Terminal web (xterm.js + WebSocket) ← ✅ concluído
4. Integração frontend ↔ backend (VMs) ← **atual**
5. Containers LXC (futura)