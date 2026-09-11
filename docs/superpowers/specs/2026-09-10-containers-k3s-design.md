# Containers em k3s — design da implementação

**Data:** 2026-09-10
**Status:** aprovado em brainstorming — pronto para plano de implementação
**Antecede:** `2026-09-08-containers-proposta.md` (10 decisões) e `2026-09-07-terraform-provisioning-design.md`

## Objetivo

Permitir que `aluno` e `pesquisador` criem containers de aplicação a partir de uma imagem, com
CPU/memória/volume limitados pelo perfil, endereço de acesso, terminal web e logs em tempo real.
O runtime é um cluster k3s single-node numa VM do Proxmox, criada pelo próprio ProxLab.

## Decisões fechadas nesta rodada

| # | Tema | Decisão |
|---|---|---|
| 11 | Instalação do k3s na VM | **SSH a partir do `cmd/k3s-bootstrap`**: cloud-init só injeta usuário + chave; o comando instala k3s e manifests via SSH |
| 12 | Endereço da VM do cluster | **IP fixo via cloud-init** (`K3S_NODE_IP`, `K3S_NODE_GATEWAY`), reservado na rede do laboratório |
| 13 | TLS no Ingress | **HTTP puro na v1**. Wildcard institucional ou cert-manager ficam como evolução |
| 14 | Status dos containers | **Consulta ao cluster a cada request** (sem cache/informer) |
| 15 | Protocolo de exposição | **Campo do usuário** (`http` padrão, `tcp` para banco etc.). HTTP + `INGRESS_DOMAIN` → Ingress; senão NodePort; TCP → sempre NodePort |
| 16 | Mount path do volume | **Não é campo**: vem do catálogo por imagem; `/data` para imagem livre |
| 17 | Variáveis de ambiente | Persistidas no banco (jsonb) para exibição no detalhe; no cluster viram `Secret` |
| 18 | GPU em containers | Fora da v1; campo removido do formulário |

### Desvios registrados na implementação (2026-09-11)

- `ContainerRuntime.Apply(ctx, c, ingressDomain)` recebe o domínio como argumento, para o pacote `kube` não depender de `config`.
- O `Provisioner` continua recebendo `uuid.UUID`; o bootstrap usa o workspace determinístico `uuid.NewSHA1(uuid.NameSpaceOID, "proxlab-k3s")`.
- `client-go` fixado em **v0.35.x** (última linha compatível com Go 1.25; v0.36+ exige Go 1.26).
- Nível PodSecurity do namespace do usuário: `restricted` (provisório). Confirmar no smoke test real; se as imagens do catálogo forem rejeitadas, baixar para `baseline`.
- Páginas dinâmicas do frontend (`/containers/[id]`, `/containers/[id]/utilizar`) são client components sem `generateStaticParams`, igual às páginas de VM (build `standalone`).

## Seção 1 — Bootstrap do cluster (`cmd/k3s-bootstrap`)

Comando Go executado uma vez pelo admin (`docker compose run backend k3s-bootstrap`), reutilizando
`internal/terraform` e o volume `terraform_data`.

Variáveis (além das já existentes de Proxmox/Terraform):

```
K3S_NODE_IP=10.x.x.x/24        IP fixo reservado (CIDR)
K3S_NODE_GATEWAY=10.x.x.1
K3S_NODE_CPU=8
K3S_NODE_MEMORY_MB=16384
K3S_NODE_DISK_GB=200
INGRESS_DOMAIN=                vazio → só NodePort
KUBECONFIG_PATH=               default $TERRAFORM_WORKDIR/k3s/kubeconfig
```

Fluxo:

1. Gera par ed25519 em `$TERRAFORM_WORKDIR/k3s/id_ed25519` (reutiliza se existir).
2. `repository.VMSpec` ganha `CloudInitSSHKeys []string`, `StaticIP string` e `Gateway string`.
   `main.tf.tmpl` renderiza `user_account.keys` e `ip_config` estático quando presentes; DHCP continua
   sendo o default. Nenhum template novo. Workspace fixo `k3s` (o `Provisioner` passa a aceitar um
   `workspaceID` string; VMs de usuário continuam usando o UUID).
3. `Provision` clona o template `PROXMOX_UBUNTU_TEMPLATE_VMID` com usuário `ubuntu`, chave e IP fixo.
4. Espera SSH responder (timeout 5 min). Via `golang.org/x/crypto/ssh` executa, em ordem:
   - instalador oficial do k3s (Traefik e local-path mantidos; `--write-kubeconfig-mode 644`);
   - `kubectl apply -f -` dos manifests embutidos no binário (`//go:embed manifests/*.yaml`):
     namespace `proxlab-system`, `registry:2` com PVC e Service ClusterIP, `registries.yaml` do k3s
     apontando para ele como mirror;
   - `cat /etc/rancher/k3s/k3s.yaml`.
5. Reescreve `server:` do kubeconfig para `https://<K3S_NODE_IP>:6443` e grava em `KUBECONFIG_PATH`.
6. Idempotente: se o workspace `k3s` já tem `terraform.tfstate`, pula o Terraform e refaz os passos 4–5
   (reinstalar/atualizar manifests).

O backend lê `KUBECONFIG_PATH` no boot. Se o arquivo não existir, sobe normalmente e as rotas de containers
respondem `503 cluster indisponível`.

## Seção 2 — Domínio e porta `ContainerRuntime`

### Entidade `Container` (`internal/domain/entity/container.go`)

```
ID uuid (PK)          UserID uuid (index)
Name string           Description string
Image string
CPUMillis int         MemoryMB int
Port int              Protocol string ("http"|"tcp")
NodePort int          (0 quando exposto por Ingress)
Env map[string]string (jsonb)
Command string        (vazio = CMD da imagem)
PersistentGB int      (0 = sem volume)
MountPath string
Namespace string      ("u-" + 8 primeiros chars do UserID)
CreatedAt, UpdatedAt
```

Constraints: unique `(user_id, name)`; unique parcial em `node_port` quando `> 0` (alocação atômica por
colisão + retry). **Sem coluna de status** (decisão 10): status vem do cluster.

### Catálogo (`internal/domain/entity/catalog.go`)

Lista estática `[]CatalogImage{Key, Label, Image, Port, Protocol, MountPath, RequiredEnv []string}`:

| Key | Image | Port | Proto | MountPath | RequiredEnv |
|---|---|---|---|---|---|
| nginx | nginx:1.27 | 80 | http | /usr/share/nginx/html | — |
| python | python:3.12 | 8000 | http | /data | — |
| node | node:22 | 3000 | http | /data | — |
| postgres | postgres:16 | 5432 | tcp | /var/lib/postgresql/data | POSTGRES_PASSWORD |
| jupyter | jupyter/minimal-notebook:latest | 8888 | http | /home/jovyan | — |
| ubuntu | ubuntu:24.04 | 0 | — | /data | — (Command `sleep infinity`; sem exposição, uso só via terminal) |

Exposto em `GET /api/containers/catalog`. Imagem livre ("Outra") usa `/data` e `Port`/`Protocol` do usuário.
`Port = 0` significa "não expor": sem `Service`, `Ingress` ou NodePort; o container só é acessível pelo terminal.

### Porta `ContainerRuntime` (`internal/domain/repository/container_runtime.go`)

```go
type ContainerStatus struct { Phase string /* pending|running|failed|unknown */; Message string }

type TerminalSession interface {
    Stdin() io.Writer
    Stdout() io.Reader
    Resize(cols, rows uint16) error
    Close() error
}

type ContainerRuntime interface {
    EnsureNamespace(ctx context.Context, ns string, q ResourceQuotaSpec) error
    Apply(ctx context.Context, c *entity.Container) error
    Delete(ctx context.Context, ns, name string) error
    Status(ctx context.Context, ns string, names []string) (map[string]ContainerStatus, error)
    Exec(ctx context.Context, ns, name string, cols, rows uint16) (TerminalSession, error)
    Logs(ctx context.Context, ns, name string, follow bool) (io.ReadCloser, error)
}
```

`EnsureNamespace` cria/atualiza: `Namespace` (labels `pod-security.kubernetes.io/enforce=restricted`),
`ResourceQuota` (cpu, memory, pods, requests.storage), `LimitRange` (default 250m/256Mi),
`NetworkPolicy` (default-deny ingress entre namespaces; permite Traefik e egress).

`Apply` cria/atualiza via server-side apply (field manager `proxlab`): `Deployment` (1 réplica, label
`app=<name>`, `securityContext` restricted, requests=limits), `Secret` com `Env`, `Service` (ClusterIP ou
NodePort com `nodePort` fixo), `Ingress` (host `<name>-<ns>.<INGRESS_DOMAIN>`) quando http + domínio,
`PVC` (`local-path`, `PersistentGB`) quando `> 0`.

`ResourceQuotaSpec` vem de `roleLimits`, que sai de `vm_usecase.go` para `internal/usecase/limits.go`:

| Role | CPU | Mem | Containers | Storage |
|---|---|---|---|---|
| aluno | 2 | 4 GB | 5 | 20 GB |
| pesquisador | 8 | 16 GB | 20 | 100 GB |

### Implementação `internal/kube/`

`client-go` com kubeconfig de `KUBECONFIG_PATH`. Um arquivo por preocupação: `client.go`,
`namespace.go`, `workload.go`, `status.go`, `exec.go`, `logs.go`. Sem lógica de negócio.

### `ContainerRepository` (`internal/domain/repository` + GORM em `internal/repository`)

`Create, FindByUserID, FindByID, FindByUserIDAndName, CountByUserID, Delete`.

## Seção 3 — API e usecase

Rotas `/api/containers` (JWT, mesmo middleware das VMs):

```
GET    /catalog        catálogo
POST   ""              cria → 201 {container, status, external_url, internal_address}
GET    ""              lista + status do cluster
GET    /:id            detalhe + status + endereços
DELETE /:id
GET    /:id/terminal   WebSocket exec
GET    /:id/logs       WebSocket logs
```

Payload `POST`:

```json
{ "name": "meu-pg", "description": "", "image": "postgres:16",
  "cpu_millis": 500, "memory_mb": 512,
  "port": 5432, "protocol": "tcp",
  "env": {"POSTGRES_PASSWORD": "..."}, "command": "",
  "persistent_gb": 5, "mount_path": "/var/lib/postgresql/data" }
```

Validação no handler: `name` `^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`; `image` sem espaços e com tag;
`protocol` ∈ {http, tcp}; `port` 0–65535 (0 = não expor, `protocol` ignorado); `mount_path` opcional (default `/data`, absoluto).

`ContainerUseCase.Create`:

1. Valida limites do perfil: CPU, memória, `persistent_gb` e `CountByUserID < MaxContainers`.
2. Rejeita nome duplicado (`FindByUserIDAndName`) → 409.
3. Se `port > 0` e (`protocol == tcp` ou `INGRESS_DOMAIN` vazio): sorteia NodePort em 30000–32767 e insere; em violação
   de unique sorteia de novo (até 5 tentativas).
4. `EnsureNamespace` com a quota do perfil (idempotente).
5. `Apply`. Em erro: remove o registro, loga o erro real, devolve genérico "não foi possível criar o container".
6. Retorna entidade + campos calculados.

`Delete`: `runtime.Delete` e só então remove do banco. Cluster fora → 503 e registro mantido (sem órfão).

Campos calculados em lista/detalhe: `status {phase, message}`, `external_url`
(`http://<name>-<ns>.<INGRESS_DOMAIN>`, `<K3S_NODE_IP>:<node_port>`, ou vazio quando `port = 0`), `internal_address` (`<name>:<port>`, vazio quando `port = 0`).

Erros: limite → 422; duplicado → 409; cluster indisponível → 503; não encontrado ou de outro usuário → 404.

## Seção 4 — Terminal e logs (WebSocket)

Contrato igual ao terminal de VM: `?token=` na query, entrada como texto, saída como binário, resize como
JSON `{"type":"resize","cols":..,"rows":..}`. Upgrader e validação de token do `terminal.go` atual são
extraídos para `internal/websocket/common.go`.

**Terminal** (`internal/websocket/container_terminal.go`, `internal/kube/exec.go`): `remotecommand`
SPDY no pod mais recente em `Running` com label `app=<name>`, comando
`/bin/sh -c 'command -v bash >/dev/null && exec bash || exec sh'`, `tty=true`, `stdin=true`. Ponte com
duas goroutines, sem handshake nem enquadramento. Imagem sem shell → mensagem "esta imagem não possui shell".

**Logs** (`internal/websocket/container_logs.go`, `internal/kube/logs.go`): `GetLogs` com
`tailLines=200`, `follow=true`, copiado linha a linha como texto. Enquanto o pod não existe, envia a
mensagem de `Status()` a cada 2 s. Somente leitura. Fecha ao desconectar ou expirar o contexto.

## Seção 5 — Frontend

**Exceção acordada à regra "não alterar CSS/layout":** a página `/containers/criar` tem sua estrutura de
campos alterada para refletir o formulário abaixo. Campos novos reutilizam exatamente as classes, medidas
e tipografia dos inputs existentes; nenhum estilo novo é criado.

`frontend/src/lib/api.ts`: tipos `Container`, `ContainerStatus`, `CatalogImage`; funções `apiGetCatalog`,
`apiListContainers`, `apiGetContainer`, `apiCreateContainer`, `apiDeleteContainer`; helpers de URL WS
para terminal e logs.

**`/containers/criar`** — formulário final, nesta ordem:

1. Nome (regra de nome validada no front).
2. Imagem: select do catálogo + opção "Outra" que revela o input livre. Escolher item do catálogo
   pré-preenche porta, protocolo, mount path (oculto) e variáveis obrigatórias.
3. CPU e memória: selects limitados pelo perfil (CPU 0.5/1/2/4/8; RAM 256 MB–16 GB).
4. Porta interna + radios "HTTP (site/API)" / "TCP (banco, outro serviço)". Checkbox "não expor" zera a porta e oculta os radios.
5. Variáveis de ambiente: lista chave/valor com "adicionar" e "remover".
6. Armazenamento persistente: SIM/NÃO (mantido) → campo tamanho (GB) quando SIM.
7. Comando (opcional, colapsado em "Avançado").
8. Descrição (mantida, opcional).

Removidos: "Requer GPU", "Definição de portas" (substituído pelo item 4).

**`/containers`**: nome, imagem, status do cluster, endereço externo. Polling 5 s enquanto houver `pending`.

**`/containers/[id]`**: endereço externo, endereço interno, variáveis (valores ocultáveis), excluir com confirmação.

**`/containers/[id]/utilizar`**: terminal xterm.js (mesmo componente da VM) + painel de logs com
auto-scroll, cada um no seu WebSocket.

Sem start/stop/restart (v2), sem GPU.

## Seção 6 — Testes e verificação

- **Unitários (usecase/handler)**: validação de payload e nome; limites por perfil; alocação de NodePort
  com colisão; mapeamento de status e endereços; catálogo. `ContainerRuntime` e repositório mockados.
- **`internal/terraform`**: `Render` com IP fixo + chave SSH; DHCP quando ausentes.
- **`internal/kube`**: `fake.NewSimpleClientset` para `EnsureNamespace`, `Apply`, `Status`.
- **Smoke real**: `cmd/proxmox-check --k3s` cria um `nginx` do catálogo, espera `running`, lê logs,
  deleta. Toca apenas recursos criados por ele.
- **Ponta a ponta manual**: bootstrap numa VM nova; criar postgres pelo front; conectar com cliente
  externo em `K3S_NODE_IP:nodeport`; terminal e logs; excluir.

## Fora de escopo (v1)

Start/stop/restart, Dockerfile/tarball (kaniko), GPU, TLS, múltiplas portas, réplicas, healthchecks,
workers adicionais, informer/cache.

## Configuração nova

`.env.example`: `K3S_NODE_IP`, `K3S_NODE_GATEWAY`, `K3S_NODE_CPU`, `K3S_NODE_MEMORY_MB`,
`K3S_NODE_DISK_GB`, `INGRESS_DOMAIN`, `KUBECONFIG_PATH`. Dockerfile do backend passa a compilar também
`cmd/k3s-bootstrap`.
