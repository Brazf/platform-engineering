# Provisionamento de VMs via Terraform — Design

**Data:** 2026-09-07
**Status:** aprovado para planejamento

## Objetivo

Substituir a criação e destruição de VMs via chamadas diretas à API REST do Proxmox
(`internal/proxmox/client.go`) por configuração Terraform gerada e aplicada pelo backend,
usando o provider `bpg/proxmox`. Operações de runtime (status, start, stop, terminal)
continuam via REST.

## Decisões

| Decisão | Escolha | Motivo |
|---|---|---|
| Escopo do Terraform | Só criar/destruir | Start/stop/status/terminal não se encaixam no modelo declarativo; REST já funciona |
| Execução | Binário `terraform` via `hashicorp/terraform-exec` | Lib oficial, evita parsear stdout na mão |
| State | Local, um diretório por VM em volume Docker | Simples, isolado; falha em uma VM não afeta outra |
| Provider | `bpg/proxmox` | Mantido ativamente, suporta `api_token` e `insecure` |
| Alocação de VMID | Delegada ao provider (omitir `vm_id`) | Provider usa `/cluster/nextid`; remove `nextVMID` do backend |
| Estado inicial | `started = false` | Igual ao comportamento atual (VM nasce parada) |

## Arquitetura

```
Handler → UseCase → Provisioner (interface)      → terraform-exec → provider bpg → Proxmox   (criar/destruir)
Handler → UseCase → ProxmoxClientInterface       → API REST Proxmox                          (status/start/stop)
WebSocket TerminalHandler → ProxmoxClientInterface → termproxy REST
```

### Nova porta de domínio — `internal/domain/repository/provisioner.go`

```go
// VMSpec descreve a VM a ser provisionada. Independente de Terraform.
type VMSpec struct {
    Name       string
    CPUCores   int
    MemoryMB   int
    DiskGB     int
    Storage    string   // default "local-zfs"
    OSType     string   // "l26" | "win10" | "win11" | "other"
    ISO        string   // ex: "local:iso/ubuntu-22.04.iso"; vazio = sem cdrom
    Bridge     string   // default "vmbr0"
    Pool       string
    Tags       []string
    RequireGPU bool
}

type Provisioner interface {
    // Provision gera a configuração, roda init+apply e retorna o VMID alocado.
    Provision(ctx context.Context, workspaceID uuid.UUID, spec VMSpec) (vmid int, err error)
    // Deprovision roda destroy e remove o workspace. Retorna ErrWorkspaceNotFound se não existir.
    Deprovision(ctx context.Context, workspaceID uuid.UUID) error
}

var ErrWorkspaceNotFound = errors.New("workspace terraform não encontrado")
```

`workspaceID` é o UUID da entidade `VM` — gerado no use case antes de chamar `Provision`.

### Implementação — `internal/terraform/`

- `runner.go` — `Runner` implementa `Provisioner`.
  - `NewRunner(cfg *config.Config) (*Runner, error)`: resolve o caminho do binário (`TERRAFORM_BIN`), cria a raiz de workspaces (`TERRAFORM_WORKDIR`) e o `TF_PLUGIN_CACHE_DIR` (`<root>/.plugin-cache`).
  - `Provision`: `os.MkdirAll(<root>/<uuid>)` → escreve `main.tf` via `template.go` → `tf.Init` → `tf.Apply` → `tf.Output` lê `vmid`. Em erro de `apply`: `tf.Destroy` best-effort, `os.RemoveAll`, retorna erro embrulhado com contexto (stderr do Terraform só em `log`).
  - `Deprovision`: se diretório não existe → `ErrWorkspaceNotFound`; senão `tf.Init` (idempotente, garante provider) → `tf.Destroy` → `os.RemoveAll`.
  - Cada operação recebe `context.Context` e aplica `context.WithTimeout(ctx, 5*time.Minute)`.
  - Credenciais do provider vão como env do processo Terraform (`tf.SetEnv`): `PROXMOX_VE_ENDPOINT`, `PROXMOX_VE_API_TOKEN` (`id=secret`), `PROXMOX_VE_INSECURE=true`. Nunca escritas em arquivo.
- `template.go` — `Render(spec VMSpec, node, gpuPCI string) ([]byte, error)` usando `text/template` embutido (`embed`). Nunca interpola credenciais.

### Template `main.tf` (forma final)

```hcl
terraform {
  required_providers {
    proxmox = { source = "bpg/proxmox", version = "~> 0.66" }
  }
}

provider "proxmox" {}   # endpoint/token/insecure via env

resource "proxmox_virtual_environment_vm" "vm" {
  node_name     = "{{ .Node }}"
  name          = "{{ .Name }}"
  machine       = "q35"
  scsi_hardware = "virtio-scsi-pci"
  started       = false
  {{- if .Pool }}
  pool_id       = "{{ .Pool }}"
  {{- end }}
  {{- if .Tags }}
  tags          = [{{ range $i, $t := .Tags }}{{ if $i }}, {{ end }}"{{ $t }}"{{ end }}]
  {{- end }}

  cpu    { cores = {{ .CPUCores }} }
  memory { dedicated = {{ .MemoryMB }} }

  disk {
    datastore_id = "{{ .Storage }}"
    interface    = "scsi0"
    size         = {{ .DiskGB }}
    file_format  = "raw"
  }

  network_device {
    model  = "virtio"
    bridge = "{{ .Bridge }}"
  }

  operating_system { type = "{{ .OSType }}" }

  {{- if .ISO }}
  cdrom {
    file_id   = "{{ .ISO }}"
    interface = "ide2"
  }
  boot_order = ["ide2", "scsi0"]
  {{- end }}

  {{- if .GPUMapping }}
  hostpci {
    device  = "hostpci0"
    mapping = "{{ .GPUMapping }}"
    pcie    = true
  }
  {{- end }}
}

output "vmid" { value = proxmox_virtual_environment_vm.vm.vm_id }
```

`GPUMapping` (nome de resource mapping PCI, escolhido pelo backend a partir de
`PROXMOX_GPU_MAPPINGS`) gera o bloco `hostpci` com `mapping`; `id` (endereço PCI cru) não é usado
porque o provider não o aceita com API token. VM clonada com GPU nasce desligada (`started = false`).
Detalhes na spec `2026-11-17-gpu-mapping-e-conflito-start.md`. Strings vindas do usuário (`Name`, `Tags`, `ISO`) passam por escape HCL
(aspas e barras) antes de renderizar.

### Layout em disco

```
$TERRAFORM_WORKDIR/                 (default /var/lib/proxlab/terraform)
├── .plugin-cache/                  TF_PLUGIN_CACHE_DIR — provider baixado uma vez
└── <vm-uuid>/
    ├── main.tf
    ├── terraform.tfstate
    ├── .terraform.lock.hcl
    └── .terraform/
```

## Mudanças no use case — `internal/usecase/vm_usecase.go`

- `NewVMUseCase(vmRepo, proxmoxClient, provisioner)`.
- `CreateVM`: valida limites → `id := uuid.New()` → `provisioner.Provision(ctx, id, spec)` → persiste `entity.VM{ID: id, ProxmoxVMID: vmid, Status: creating, ...}`. Se `Provision` falhar, nada é persistido. Se `vmRepo.Create` falhar após provisionar, roda `Deprovision` best-effort para não deixar VM órfã no Proxmox.
- `DeleteVM`: `provisioner.Deprovision(ctx, vm.ID)`; se `errors.Is(err, ErrWorkspaceNotFound)` (VM criada antes desta migração, sem tfstate) → `log.Printf` de aviso com o VMID e segue; qualquer outro erro aborta. Depois `vmRepo.Delete`. VMs pré-migração ficam órfãs no Proxmox e são removidas manualmente pela interface — decisão explícita para não manter caminho REST de destruição.
- Default de `node` continua vindo de `cfg.ProxmoxNode` (runner injeta no template); o campo `Node` de `CreateVMRequest` deixa de ser usado para escolher nó — o valor gravado na entidade passa a ser sempre `cfg.ProxmoxNode`. O use case recebe `node string` no construtor para isso.

## Configuração

Novos campos em `config.Config` / env:

| Env | Default | Uso |
|---|---|---|
| `TERRAFORM_BIN` | `terraform` | caminho do binário |
| `TERRAFORM_WORKDIR` | `/var/lib/proxlab/terraform` | raiz dos workspaces |

`.env.example` e `docker-compose.yml` atualizados. `Dockerfile`: stage runtime copia `/bin/terraform`
de `hashicorp/terraform:1.9` e cria `/var/lib/proxlab/terraform` com dono `appuser`. Compose ganha
volume `terraform_data:/var/lib/proxlab/terraform`.

## Limpeza de código legado

O que sai, e por quê:

| Item | Ação | Motivo |
|---|---|---|
| `ProxmoxClient.CreateVM`, `nextVMID` | remover | provisionamento vai para Terraform |
| `proxmox.VMCreateRequest` | remover | substituído por `repository.VMSpec` |
| `ProxmoxClient.GetVNCProxy`, `VNCProxyResponse`, `pveVNCProxy` | remover | sem nenhum uso (terminal usa termproxy) |
| `ProxmoxClient.ListVMs`, `pveQEMUEntry` | remover | usado só pela ferramenta de diagnóstico |
| `ProxmoxClient.GetTaskStatus`, `TaskStatus`, `pveTaskStatus` | remover | usado só pela ferramenta de diagnóstico; UPIDs não são mais consumidos |
| `ProxmoxClient.DeleteVM` | remover | destruição vai para Terraform; VMs pré-migração são apagadas manualmente |
| `ProxmoxClientInterface` | reduzir a `GetVMStatus`, `StartVM`, `StopVM`, `GetTermProxy` | reflete o uso real |
| `cmd/proxmox-check/main.go` | reescrever como smoke test do fluxo novo: `Provision` (TF) → `GetVMStatus` (REST) → `Deprovision` (TF) | valida a integração real; só toca em VM criada por ele mesmo |
| `backend/proxmox-check` (binário compilado, rastreado no git) | `git rm` + adicionar `backend/proxmox-check` e `bin/` ao `.gitignore` | artefato de build não deve estar no repo |
| Mocks em `vm_usecase_test.go` dos métodos removidos | remover | acompanham a interface |
| Comentários "em migração para Terraform" em `CLAUDE.md` | substituir pelo fluxo definitivo | pendência concluída |

`ProxmoxClient` continua exposto porque `TerminalHandler` precisa de `WsBase()`, `Node()`, `AuthHeader()` e `GetTermProxy`.

## Testes

- `internal/terraform/template_test.go` — golden tests de `Render` para: spec mínima; com ISO; com GPU; com pool+tags; nome com aspas (escape). Parse com `hashicorp/hcl/v2/hclparse` para garantir HCL válido.
- `internal/terraform/runner_test.go` — testes que não precisam de Proxmox: `Deprovision` em workspace inexistente retorna `ErrWorkspaceNotFound`; `Provision` escreve `main.tf` no diretório certo antes de chamar o terraform (usando `TERRAFORM_BIN` apontando para um script fake que falha, verificando limpeza do diretório).
- `internal/usecase/vm_usecase_test.go` — novo mock `mockProvisioner`; casos novos: `CreateVM` sucesso chama `Provision` e persiste com o VMID retornado; `Provision` falha → nada persistido; `vmRepo.Create` falha → `Deprovision` chamado; `DeleteVM` com workspace → `Deprovision` + `vmRepo.Delete`; `DeleteVM` sem workspace (`ErrWorkspaceNotFound`) → ainda remove do banco; `Deprovision` com outro erro → não remove do banco. Casos de limite de role permanecem.
- Verificação manual contra o Proxmox real via `go run ./cmd/proxmox-check`, respeitando a regra: só criar/destruir VM criada pelo próprio teste.

## Documentação

- `CLAUDE.md`: marcar pendência `[x]`; "Fluxo obrigatório" → `Handler → UseCase → Provisioner → terraform-exec → Proxmox` (criar/destruir) e `Handler → UseCase → ProxmoxClient → Proxmox API` (runtime); "Proxmox" ganha envs `TERRAFORM_*`; "Estrutura" ganha `internal/terraform/`; fase 6 marcada concluída.
- `.env.example`: novas variáveis.
- `swagger.yaml`: sem mudança (contrato HTTP não muda).

## Fora de escopo

- Containers LXC (fase 5).
- State remoto / locking entre múltiplas réplicas do backend.
- Atualização (resize) de VM via `apply` incremental.
- Importar VMs pré-migração para tfstate (`terraform import`).
