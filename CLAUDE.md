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
│   ├── kube/                   # ContainerRuntime: client-go contra o k3s (apply/status/exec/logs)
│   ├── k3s/                    # bootstrap do cluster: chave SSH, instala k3s, kubeconfig
│   └── websocket/              # proxy terminal VMs + terminal/logs de containers
├── cmd/k3s-bootstrap/          # cria a VM k3s-server e instala o k3s (roda uma vez)
└── migrations/
```

## Fluxo obrigatório
Handler → UseCase → Repository Interface → GORM → PostgreSQL
Handler → UseCase → Provisioner Interface → terraform-exec (provider bpg/proxmox) → Proxmox   (criar/destruir VM)
Handler → UseCase → ProxmoxClient Interface → Proxmox API REST                                (status/start/stop/terminal)
Handler → UseCase → ContainerRuntime Interface → client-go → k3s                              (containers)

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
- Containers: cluster k3s single-node na VM `k3s-server` (criada por `cmd/k3s-bootstrap`, IP fixo `K3S_NODE_IP`). Backend fala com o cluster via `internal/kube` (client-go v0.35, kubeconfig em `KUBECONFIG_PATH`; ausente → rotas de containers respondem 503). Namespace por usuário `u-<8 chars>`, quota por perfil, status lido do cluster a cada request (sem coluna no banco). HTTP + `INGRESS_DOMAIN` → Ingress `http://<nome>-<ns>.<domínio>`; TCP ou sem domínio → NodePort 30000–32767 em `K3S_NODE_IP`; porta 0 = só terminal.
- Env k3s: `K3S_NODE_IP`, `K3S_NODE_GATEWAY`, `K3S_NODE_CPU`, `K3S_NODE_MEMORY_MB`, `K3S_NODE_DISK_GB`, `INGRESS_DOMAIN`, `KUBECONFIG_PATH`
- Smoke tests: `go run ./cmd/proxmox-check` (VM) e `go run ./cmd/proxmox-check --k3s` (container no cluster real)

## Autenticação
- JWT salvo no localStorage com chave `"token"`
- Todas as rotas /api/* exigem `Authorization: Bearer <token>`
- WebSocket aceita `?token=` como query param
- 401 → redirecionar para /login

## Regras
- Nunca: lógica em main.go, GORM em handlers, hardcode de credenciais, expor erros internos do Proxmox, alterar CSS/layout do frontend (exceção acordada: `/containers/criar` tem campos adicionados/removidos reutilizando as classes existentes — ver spec de 2026-09-10)
- Sempre: `if err != nil`, `context.Context` em I/O, validar payload nos handlers, UUIDs como PK

## Fases
1. Auth JWT + usuários ← ✅ concluído
2. CRUD de VMs no Proxmox ← ✅ concluído
3. Terminal web (xterm.js + WebSocket) ← ✅ concluído
4. Integração frontend ↔ backend (VMs) ← ✅ concluído (fluxo principal; ajustes pontuais ainda surgirão)
5. Migração do provisionamento para Terraform ← ✅ concluído
6. Containers de aplicação em k3s ← ✅ código concluído (v1); falta rodar `k3s-bootstrap` e o smoke `--k3s` no Proxmox real

## Pendências
- [x] **Migrar provisionamento de recursos para Terraform**: o backend deixou de criar/destruir VMs via API REST do Proxmox e passou a gerar/aplicar configuração Terraform (`internal/terraform/`, provider `bpg/proxmox`). Spec: `docs/superpowers/specs/2026-09-07-terraform-provisioning-design.md`.
- [ ] **Fase containers (k3s)**: brainstorming → spec → plano. VM do cluster nasce por um comando de bootstrap (`cmd/k3s-bootstrap`), executado uma vez; containers gerenciados via `client-go` (`internal/kube/`).