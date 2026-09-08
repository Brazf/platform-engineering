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
│   ├── proxmox/                # cliente REST Proxmox (status/start/stop/termproxy)
│   ├── terraform/              # Provisioner: gera main.tf e roda terraform-exec
│   └── websocket/              # proxy terminal VMs
└── migrations/
```

## Fluxo obrigatório
Handler → UseCase → Repository Interface → GORM → PostgreSQL
Handler → UseCase → Provisioner Interface → terraform-exec (provider bpg/proxmox) → Proxmox   (criar/destruir VM)
Handler → UseCase → ProxmoxClient Interface → Proxmox API REST                                (status/start/stop/terminal)

## Proxmox
- Auth: API Token via header `Authorization: PVEAPIToken=<id>=<secret>`
- TLS: `InsecureSkipVerify: true`
- Node: `pve01`
- Env: `PROXMOX_HOST`, `PROXMOX_NODE`, `PROXMOX_TOKEN_ID`, `PROXMOX_TOKEN_SECRET`
- WebSocket termproxy: Backend → Proxmox usa `Subprotocols: []string{"binary"}`; protocolo: 1ª mensagem `"<user>:<ticket>\n"` → resposta `OK`; entrada do teclado enquadrada `0:<len>:<dados>`, resize `1:cols:rows:`, ping `2` a cada 30s. Sem o handshake o Proxmox falha com `failed reading ticket: timed out`
- Provisionamento: Terraform `bpg/proxmox ~> 0.112.0`; um workspace por VM em `$TERRAFORM_WORKDIR/<vm-uuid>`; credenciais só via env do processo (`PROXMOX_VE_ENDPOINT`, `PROXMOX_VE_API_TOKEN`, `PROXMOX_VE_INSECURE`)
- Env Terraform: `TERRAFORM_BIN`, `TERRAFORM_WORKDIR`
- VMs Ubuntu: clone completo do template cloud-init `PROXMOX_UBUNTU_TEMPLATE_VMID` (default 9000, `ubuntu-2404-cloud`); nascem ligadas, com usuário `ubuntu` e senha gerada por VM, devolvida uma única vez em `credentials` no `201` de `POST /api/vms` (não persistida). Outros SOs seguem o fluxo de VM vazia.
- VMs criadas antes da migração não têm workspace: ao deletar, só o registro sai do banco; a VM é removida manualmente no Proxmox

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
6. Migração do provisionamento para Terraform ← ✅ concluído

## Pendências
- [x] **Migrar provisionamento de recursos para Terraform**: o backend deixou de criar/destruir VMs via API REST do Proxmox e passou a gerar/aplicar configuração Terraform (`internal/terraform/`, provider `bpg/proxmox`). Spec: `docs/superpowers/specs/2026-09-07-terraform-provisioning-design.md`.