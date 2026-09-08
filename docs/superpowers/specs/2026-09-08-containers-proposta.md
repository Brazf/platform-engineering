# Containers no ProxLab — proposta de arquitetura (rascunho para discussão)

**Data:** 2026-09-08
**Status:** proposta em discussão — nada implementado

## O problema

Vários usuários (`aluno`, `pesquisador`) poderão criar containers. Precisamos decidir a tecnologia
(LXC, Docker ou Kubernetes) considerando:

1. **Multi-tenancy real** — isolamento entre usuários, limites por perfil, sem "vizinho barulhento".
2. **Conflito de portas** — dezenas de containers expondo a porta 80/3000/8080 ao mesmo tempo.
3. **Origem das imagens** — registries públicos, imagem própria, Dockerfile enviado, tarball de imagem.
4. **Custo operacional** — é um laboratório acadêmico; quem mantém a infra depois do TCC?
5. **Coerência com o que já existe** — backend Go, Terraform, terminal web via WebSocket.

## Opções avaliadas

### A. LXC nativo do Proxmox

Container de sistema (um "mini-Linux" com init), criado via `proxmox_virtual_environment_container`
do mesmo provider Terraform já em uso.

| Prós | Contras |
|---|---|
| Zero infra nova; mesmo fluxo de VM (Terraform, termproxy, pool, tags) | **Não roda imagem Docker nem Dockerfile** — o usuário recebe um Ubuntu/Debian/Alpine vazio |
| Cada container tem IP próprio na `vmbr0` → **nenhum conflito de porta** | Templates limitados aos do Proxmox (`datapool-templates`) |
| Quotas de CPU/RAM/disco nativas por container | Rodar Docker *dentro* do LXC (nesting) funciona mas é frágil e não é multi-tenant |
| Terminal já funciona (mesmo termproxy) | Não atende ao requisito de imagens |

**Veredito:** ótimo como "VM leve", mas não resolve o que os usuários vão pedir (imagens). Pode ser um
*quick win* de baixo custo, mas não deve ser a resposta final para "containers".

### B. Docker em host(s) dedicado(s)

Uma ou mais VMs Proxmox com Docker; o backend fala com a Docker API (ou via Terraform provider `kreuzwerker/docker`).

| Prós | Contras |
|---|---|
| Modelo mental que os alunos já conhecem | **Isolamento fraco**: todos compartilham um daemon; root no container ≈ root no host se houver descuido em volumes/capabilities |
| Simples de operar em pequena escala | Conflito de portas precisa ser resolvido por nós (alocador de portas + reverse proxy) |
| Suporta Dockerfile, pull de registry, `docker load` | Quotas por usuário só via disciplina do backend (nada nativo por "tenant") |
| | Sem scheduler: com 2+ hosts, o backend vira orquestrador improvisado |

**Veredito:** funciona para um piloto pequeno; envelhece mal com muitos usuários.

### C. Kubernetes (k3s) em VMs do Proxmox

Cluster k3s (1 control-plane + N workers, todos VMs do Proxmox — podem ser criadas pelo mesmo Terraform).
Cada usuário vira um **namespace**.

| Prós | Contras |
|---|---|
| **Multi-tenancy nativa**: `Namespace` + `ResourceQuota` + `LimitRange` + `NetworkPolicy` + RBAC | Curva de aprendizado e operação maior (upgrades, storage, ingress) |
| **Portas resolvidas pelo `Ingress`**: `app.aluno1.lab.ifpe.edu.br` → nunca há conflito | Precisa de DNS wildcard (ou fallback com NodePort alocado pelo backend) |
| Imagens de qualquer registry; **build de Dockerfile dentro do cluster** (kaniko) | Mais componentes: registry, ingress-controller, storage class |
| Terraform provider `kubernetes`/`helm` — coerente com a direção do projeto | k3s single-node é fácil; HA é trabalho |
| Escala horizontal adicionando worker VMs | |

**Veredito:** é a única opção que atende todos os requisitos (isolamento, portas, imagens, quotas)
sem que o backend reimplemente um orquestrador.

## Recomendação

**Opção C (k3s), entregue em fases**, mantendo a Opção A como complemento barato:

- **Fase 5 — LXC nativo** (pequena): entrega "container" como VM leve reaproveitando 90% do código de VM.
  Útil para quem quer só um Linux rápido. Não bloqueia nem conflita com a fase seguinte.
- **Fase 6 — k3s "containers de aplicação"** (a proposta principal, detalhada abaixo).

Se preferirmos pular a Fase 5 e ir direto ao k3s, nada se perde.

## Desenho da Fase 6 — k3s

### Infra

- VMs no Proxmox: `k3s-server` (4 vCPU / 8 GB) + `k3s-worker-{1..N}` (dimensionar conforme demanda). Criadas
  pelo mesmo `internal/terraform` (clone do template Ubuntu + cloud-init instalando k3s).
- Componentes no cluster: `traefik` (já vem no k3s) como ingress, `local-path` storage (vem no k3s;
  Longhorn se precisarmos de replicação), **registry privado** (`registry:2` no início; Harbor se quisermos
  UI, scan e RBAC de imagens), **kaniko** para builds.
- DNS wildcard `*.lab.<dominio>` apontando para o IP do ingress. Sem isso, cair no fallback de NodePort.

### Modelo de multi-tenancy

Ao criar o usuário na plataforma (ou no primeiro container), o backend cria:

```
Namespace        user-<uuid-curto>
ResourceQuota    conforme role: aluno {cpu: 2, memory: 4Gi, pods: 5, storage: 20Gi}
                                pesquisador {cpu: 8, memory: 16Gi, pods: 20, storage: 100Gi}
LimitRange       defaults por container (ex.: 250m CPU / 256Mi) para o quota funcionar
NetworkPolicy    default-deny entre namespaces; permite ingress do traefik e egress à internet
ServiceAccount   sem permissões (os pods não falam com a API do cluster)
```

Os limites por role já existem no `VMUseCase` (`roleLimits`); a mesma tabela alimenta o `ResourceQuota`.

### Modelo de "Container" na plataforma

Entidade `Container` (Postgres): `id, user_id, name, image, cpu_millis, memory_mb, storage_gb, ports[], env{},
command, status, url, created_at`. No cluster vira `Deployment` (1 réplica) + `Service` + `Ingress` +
opcional `PVC`. Aplicado via Terraform (`kubernetes` provider) num workspace por container — **mesmo
padrão** do `internal/terraform` de VMs (`Provisioner` com um segundo template), ou via `client-go`
direto se o Terraform mostrar-se lento para operações frequentes (avaliar no piloto).

### Portas — como evitar conflito

1. **Padrão: Ingress por subdomínio.** O usuário declara a porta *interna* do container (ex.: 3000). O backend
   cria `Service` (ClusterIP) + `Ingress` em `https://<container>-<user>.lab.<dominio>`. Nenhuma porta
   externa é reservada; conflito é impossível.
2. **Fallback (sem DNS wildcard ou protocolos não-HTTP, ex.: banco, jogo UDP):** o backend aloca um
   `NodePort` no range 30000–32767 a partir de uma tabela `allocated_ports` no Postgres (unique constraint
   → alocação atômica). O usuário recebe `ip-do-cluster:31234`.
3. Portas privilegiadas (<1024) nunca são expostas diretamente.

### Imagens — origens suportadas

| Origem | Como funciona | Controles |
|---|---|---|
| **Registry público** (Docker Hub, ghcr.io, quay.io) | usuário informa `nginx:1.27`; kubelet faz o pull | allowlist de registries; rate limit do Docker Hub → configurar `registries.yaml` do k3s com mirror/credencial institucional |
| **Registry privado da plataforma** | imagens construídas pela plataforma ficam em `registry.lab.<dominio>/user-<id>/<nome>:<tag>` | um repositório por usuário; pull só do próprio namespace |
| **Dockerfile enviado** (+ contexto `.zip`/`.tar.gz`) | backend grava contexto num PVC/bucket e cria um `Job` kaniko que builda e faz push no registry privado; logs do build via WebSocket (mesmo padrão do terminal) | limite de tamanho do contexto (ex.: 50 MB), timeout do build (10 min), `FROM` restrito à allowlist |
| **Tarball de imagem** (`docker save`) | upload → backend usa `crane push`/`skopeo copy` para o registry privado | limite de tamanho (ex.: 2 GB); scan opcional com Trivy antes de liberar |
| **Imagem pública de "catálogo"** | lista curada no front (Ubuntu, Python, Node, Postgres, Jupyter…) | melhor UX para alunos; é só um atalho para a origem 1 |

Todo pull/push passa pelo registry privado quando a imagem foi gerada pela plataforma; imagens públicas
podem ser cacheadas no mesmo registry (pull-through cache) para economizar banda e evitar rate limit.

### Segurança mínima

- `PodSecurity` admission em `restricted` nos namespaces de usuário: sem `privileged`, sem `hostPath`,
  sem `hostNetwork`, `runAsNonRoot` quando a imagem permitir (`baseline` como escape para imagens que
  exigem root, decidido por role).
- Sem acesso ao `kubectl`/API do cluster pelos usuários — tudo via backend.
- Egress à internet permitido; ingress só via traefik.
- Segredos (`env` sensíveis) como `Secret`, não em `ConfigMap`.

### Terminal e logs

- Terminal: backend faz `exec` no pod via `client-go` (`remotecommand`, stream WebSocket/SPDY) e proxya
  para o xterm.js — mesmo contrato `GET /api/containers/:id/terminal?token=` do terminal de VM.
- Logs: `GET /api/containers/:id/logs` (stream `kubectl logs -f`).

### API (espelha VMs)

```
POST   /api/containers                 cria (imagem ou build)
GET    /api/containers                 lista do usuário
GET    /api/containers/:id             detalhe + url + status
POST   /api/containers/:id/start|stop|restart
DELETE /api/containers/:id
GET    /api/containers/:id/terminal    WebSocket exec
GET    /api/containers/:id/logs        WebSocket logs
POST   /api/images/build               upload Dockerfile+contexto → job kaniko
POST   /api/images/upload              upload tarball → registry
GET    /api/images                     imagens do usuário no registry privado
```

### Riscos e o que validar no piloto

| Risco | Mitigação / validação |
|---|---|
| DNS wildcard indisponível na rede do IFPE | Fallback NodePort desde o início; testar cedo com a equipe de rede |
| Terraform lento para criar/destruir muitos containers (init por workspace ≈ 5–10 s) | Medir; se doer, usar `client-go` direto no `ContainerProvisioner` (a interface de domínio isola essa troca) |
| Builds kaniko consumindo o cluster | Namespace `builds` com quota própria; fila (1 build por usuário por vez) |
| Alunos rodando mineração/abuso | Quotas + NetworkPolicy + TTL opcional (container para após X horas para `aluno`) |
| Operação do cluster após o TCC | k3s single-server + backups do etcd/sqlite; documentar runbook; começar com 1 worker |

## Decisões já tomadas (2026-09-08)

- **Decisão 2 (DNS):** Ingress por subdomínio como padrão, **NodePort como fallback automático**, controlado por
  config (`INGRESS_DOMAIN` vazio → NodePort). Não bloqueia o desenvolvimento. Em paralelo, o usuário vai pedir
  ao TI do campus um wildcard `*.lab.cces.belojardim.ifpe.edu.br` apontando para o IP da VM do k3s (o DNS
  `cces.belojardim.ifpe.edu.br` já existe — é o do `pve01`). Para testes locais sem DNS institucional, usar
  `sslip.io`/`nip.io` (`app.10.0.0.50.sslip.io` resolve para `10.0.0.50`).

## Decisões em aberto (para discutirmos)

1. Fazer a Fase 5 (LXC) ou ir direto ao k3s?
3. Registry: `registry:2` (simples) ou Harbor (UI, scan, RBAC) desde o começo?
4. `Provisioner` de containers via Terraform (coerência) ou `client-go` (velocidade)? Proposta: começar com
   Terraform, medir, trocar se necessário.
5. Persistência: os containers de aluno têm disco persistente ou são efêmeros por padrão?
6. Limites concretos por role (a tabela acima é chute inicial).

## Fora de escopo por enquanto

GPU em containers (device plugin NVIDIA — possível depois, reaproveitando o passthrough das VMs);
containers Windows; multi-cluster.

## Estado do projeto ao fim de 2026-09-08 (para retomar)

Tudo em `dev`, commitado. Feito nesta rodada:

- Provisionamento de VMs migrado para Terraform (`internal/terraform`, provider `bpg/proxmox ~> 0.112.0`), spec em
  `docs/superpowers/specs/2026-09-07-terraform-provisioning-design.md`.
- VMs Ubuntu clonam o template cloud-init 9000, nascem ligadas, senha gerada por VM e exibida uma vez no front.
- Terminal web funcionando: handshake do termproxy, reconexão automática, Enter até `login:`; botões
  Reiniciar (`POST /api/vms/:id/reboot`) e Desligar funcionais.
- Testado de ponta a ponta pelo usuário contra o Proxmox real (VM 102).

Pendências pequenas: apagar branch `feat/terraform-provisioning`; `.env.example` está no `.gitignore`, então as
envs novas (`TERRAFORM_BIN`, `TERRAFORM_WORKDIR`, `PROXMOX_UBUNTU_TEMPLATE_VMID`) precisam ser adicionadas à mão;
Windows precisará de noVNC (sem console serial).

**Próximo passo sugerido:** responder às decisões em aberto acima (principalmente a 1 — LXC antes ou k3s direto)
e então rodar brainstorming → spec → plano da fase de containers, no mesmo fluxo usado para o Terraform.
