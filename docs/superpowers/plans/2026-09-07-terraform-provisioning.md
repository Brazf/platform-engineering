# Terraform Provisioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Criar e destruir VMs no Proxmox via Terraform (provider `bpg/proxmox`) executado pelo backend com `terraform-exec`, removendo o caminho REST de criação/destruição.

**Architecture:** Nova porta de domínio `repository.Provisioner` implementada por `internal/terraform.Runner`, que gera `main.tf` por template, roda `init/apply/destroy` em um workspace por VM (`$TERRAFORM_WORKDIR/<vm-uuid>`) e lê o `vmid` do output. `VMUseCase` usa `Provisioner` para criar/deletar e `ProxmoxClientInterface` (REST) só para status/start/stop/termproxy. Métodos REST de criação/destruição e os sem uso são removidos.

**Tech Stack:** Go 1.25, `github.com/hashicorp/terraform-exec v0.25.3`, `github.com/hashicorp/hcl/v2 v2.24.0` (só em teste), Terraform CLI ≥ 1.8, provider `bpg/proxmox ~> 0.112.0`, `text/template` + `embed`.

**Spec:** `docs/superpowers/specs/2026-09-07-terraform-provisioning-design.md`

## Global Constraints

- Fluxo obrigatório: Handler → UseCase → interface de domínio → implementação. Nunca GORM ou Terraform em handlers.
- Sempre `if err != nil`; `context.Context` em todo I/O.
- Nunca expor stderr/erros internos do Terraform ou Proxmox ao cliente HTTP — só em `log`.
- Nunca escrever credenciais em arquivo: `PROXMOX_VE_*` vão como env do processo Terraform.
- `TF_PLUGIN_CACHE_DIR = <TERRAFORM_WORKDIR>/.plugin-cache`.
- Timeout de 5 minutos por operação Terraform.
- Provider pinado em `~> 0.112.0`; `network_device` é **atributo** (lista de objetos), não bloco.
- Comentários e mensagens de erro em português, seguindo o código existente.
- Rodar `cd backend && go build ./... && go vet ./... && go test ./...` antes de cada commit.
- Nunca criar/operar VMs existentes no Proxmox durante testes; só a VM criada pelo próprio smoke test.

---

## File map

| Arquivo | Ação |
|---|---|
| `backend/go.mod`, `backend/go.sum` | adicionar `terraform-exec`, `hcl/v2` |
| `backend/internal/config/config.go` | campos `TerraformBin`, `TerraformWorkdir` |
| `backend/internal/domain/repository/provisioner.go` | **criar** — `VMSpec`, `Provisioner`, `ErrWorkspaceNotFound` |
| `backend/internal/terraform/template.go` | **criar** — `Render` |
| `backend/internal/terraform/main.tf.tmpl` | **criar** — template embutido |
| `backend/internal/terraform/template_test.go` | **criar** |
| `backend/internal/terraform/runner.go` | **criar** — `Runner` |
| `backend/internal/terraform/runner_test.go` | **criar** |
| `backend/internal/domain/repository/proxmox_client.go` | reduzir interface |
| `backend/internal/proxmox/client.go` | remover `CreateVM`, `nextVMID`, `DeleteVM`, `GetVNCProxy`, `ListVMs`, `GetTaskStatus` |
| `backend/internal/proxmox/types.go` | remover `VMCreateRequest`, `VNCProxyResponse`, `TaskStatus`, `pveVNCProxy`, `pveQEMUEntry`, `pveTaskStatus` |
| `backend/internal/usecase/vm_usecase.go` | usar `Provisioner` |
| `backend/internal/usecase/vm_usecase_test.go` | `mockProvisioner`, novos casos |
| `backend/internal/handler/vm_handler.go` | remover campo `Node` do payload |
| `backend/cmd/server/main.go` | wiring do `Runner` |
| `backend/cmd/proxmox-check/main.go` | reescrever como smoke test |
| `backend/proxmox-check` | `git rm` |
| `.gitignore` | ignorar binários |
| `backend/Dockerfile`, `docker-compose.yml`, `.env.example` | terraform binário, volume, envs |
| `CLAUDE.md` | fluxo definitivo, pendência concluída |

---

### Task 1: Configuração e dependências

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/go.mod`, `backend/go.sum`
- Modify: `.env.example`

**Interfaces:**
- Produces: `config.Config.TerraformBin string`, `config.Config.TerraformWorkdir string`.

- [ ] **Step 1: Adicionar dependências**

```bash
cd backend && go get github.com/hashicorp/terraform-exec@v0.25.3 github.com/hashicorp/hcl/v2@v2.24.0 && go mod tidy
```

`hcl/v2` fica como dependência direta mesmo sendo usada só em testes — `go mod tidy` a mantém.

- [ ] **Step 2: Adicionar campos ao Config**

Em `backend/internal/config/config.go`, dentro de `type Config struct`, após o bloco `// Proxmox`:

```go
	// Terraform
	TerraformBin     string // caminho do binário terraform. Default: "terraform" (resolve via PATH)
	TerraformWorkdir string // raiz dos workspaces por VM
```

Em `Load()`, após `ProxmoxGPUPCI:`:

```go
		TerraformBin:       getEnv("TERRAFORM_BIN", "terraform"),
		TerraformWorkdir:   getEnv("TERRAFORM_WORKDIR", "/var/lib/proxlab/terraform"),
```

- [ ] **Step 3: Atualizar `.env.example`**

Após o bloco `# ── Proxmox VE` (antes de `# ── Servidor`), inserir:

```
# ── Terraform ─────────────────────────────────────────────────────────────────
# Caminho do binário terraform (na imagem Docker já está em /usr/local/bin/terraform)
TERRAFORM_BIN=terraform
# Raiz dos workspaces (um subdiretório por VM com main.tf + tfstate). Montado como volume no compose.
TERRAFORM_WORKDIR=/var/lib/proxlab/terraform
```

- [ ] **Step 4: Verificar build**

Run: `cd backend && go build ./... && go vet ./...`
Expected: sem erros.

- [ ] **Step 5: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/config/config.go .env.example
git commit -m "feat(config): variáveis TERRAFORM_BIN e TERRAFORM_WORKDIR + deps terraform-exec"
```

---

### Task 2: Porta de domínio `Provisioner`

**Files:**
- Create: `backend/internal/domain/repository/provisioner.go`

**Interfaces:**
- Produces:
  ```go
  type VMSpec struct { Name string; CPUCores, MemoryMB, DiskGB int; Storage, OSType, ISO, Bridge, Pool string; Tags []string; RequireGPU bool }
  type Provisioner interface {
      Provision(ctx context.Context, workspaceID uuid.UUID, spec VMSpec) (int, error)
      Deprovision(ctx context.Context, workspaceID uuid.UUID) error
  }
  var ErrWorkspaceNotFound error
  ```

- [ ] **Step 1: Criar o arquivo**

```go
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrWorkspaceNotFound indica que não há workspace Terraform para a VM
// (ex.: VM criada antes da migração para Terraform).
var ErrWorkspaceNotFound = errors.New("workspace terraform não encontrado")

// VMSpec descreve a VM a ser provisionada. Independente da ferramenta de provisionamento.
type VMSpec struct {
	Name       string
	CPUCores   int
	MemoryMB   int
	DiskGB     int
	Storage    string   // storage Proxmox. Vazio → "local-zfs"
	OSType     string   // "l26" | "win10" | "win11" | "other"
	ISO        string   // ex: "local:iso/ubuntu-22.04.iso"; vazio = sem cdrom
	Bridge     string   // bridge de rede. Vazio → "vmbr0"
	Pool       string   // resource pool do Proxmox
	Tags       []string
	RequireGPU bool
}

// Provisioner é o port para criação e destruição de VMs no Proxmox.
// Implementado por internal/terraform.Runner.
type Provisioner interface {
	// Provision cria a VM e retorna o VMID alocado pelo Proxmox.
	// workspaceID identifica o estado da VM (é o UUID da entidade VM).
	Provision(ctx context.Context, workspaceID uuid.UUID, spec VMSpec) (vmid int, err error)

	// Deprovision destrói a VM e remove o workspace.
	// Retorna ErrWorkspaceNotFound se o workspace não existir.
	Deprovision(ctx context.Context, workspaceID uuid.UUID) error
}
```

- [ ] **Step 2: Verificar build**

Run: `cd backend && go build ./...`
Expected: sem erros.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/domain/repository/provisioner.go
git commit -m "feat(domain): porta Provisioner e VMSpec"
```

---

### Task 3: Template HCL

**Files:**
- Create: `backend/internal/terraform/main.tf.tmpl`
- Create: `backend/internal/terraform/template.go`
- Test: `backend/internal/terraform/template_test.go`

**Interfaces:**
- Consumes: `repository.VMSpec` (Task 2).
- Produces: `terraform.Render(spec repository.VMSpec, node, gpuPCI string) ([]byte, error)`.

- [ ] **Step 1: Escrever o teste**

`backend/internal/terraform/template_test.go`:

```go
package terraform_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseSpec() repository.VMSpec {
	return repository.VMSpec{
		Name:     "minha-vm",
		CPUCores: 2,
		MemoryMB: 2048,
		DiskGB:   20,
		OSType:   "l26",
	}
}

// parseHCL garante que a saída é HCL sintaticamente válido.
func parseHCL(t *testing.T, src []byte) {
	t.Helper()
	_, diags := hclparse.NewParser().ParseHCL(src, "main.tf")
	require.False(t, diags.HasErrors(), "HCL inválido: %s\n---\n%s", diags.Error(), src)
}

func TestRender_MinimalSpec(t *testing.T) {
	out, err := terraform.Render(baseSpec(), "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `source  = "bpg/proxmox"`)
	assert.Contains(t, s, `node_name     = "pve01"`)
	assert.Contains(t, s, `name          = "minha-vm"`)
	assert.Contains(t, s, `started       = false`)
	assert.Contains(t, s, `cores = 2`)
	assert.Contains(t, s, `dedicated = 2048`)
	assert.Contains(t, s, `datastore_id = "local-zfs"`)
	assert.Contains(t, s, `size         = 20`)
	assert.Contains(t, s, `bridge = "vmbr0"`)
	assert.Contains(t, s, `type = "l26"`)
	assert.Contains(t, s, `output "vmid"`)
	assert.NotContains(t, s, "pool_id")
	assert.NotContains(t, s, "tags")
	assert.NotContains(t, s, "cdrom")
	assert.NotContains(t, s, "hostpci")
	assert.NotContains(t, s, "vm_id ")
}

func TestRender_WithISO(t *testing.T) {
	spec := baseSpec()
	spec.ISO = "local:iso/ubuntu-22.04.iso"
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `file_id   = "local:iso/ubuntu-22.04.iso"`)
	assert.Contains(t, s, `interface = "ide2"`)
	assert.Contains(t, s, `boot_order = ["ide2", "scsi0"]`)
}

func TestRender_WithGPU(t *testing.T) {
	spec := baseSpec()
	spec.RequireGPU = true
	out, err := terraform.Render(spec, "pve01", "0000:01:00.0,pcie=1")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `device = "hostpci0"`)
	assert.Contains(t, s, `id     = "0000:01:00.0"`)
	assert.Contains(t, s, `pcie   = true`)
}

func TestRender_GPURequestedButNotConfigured(t *testing.T) {
	spec := baseSpec()
	spec.RequireGPU = true
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	assert.NotContains(t, string(out), "hostpci")
}

func TestRender_WithPoolAndTags(t *testing.T) {
	spec := baseSpec()
	spec.Pool = "students"
	spec.Tags = []string{"proxlab", "aluno"}
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `pool_id       = "students"`)
	assert.Contains(t, s, `tags          = ["proxlab", "aluno"]`)
}

func TestRender_EscapesQuotesAndInterpolation(t *testing.T) {
	spec := baseSpec()
	spec.Name = `vm "x" ${evil}`
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	assert.True(t, strings.Contains(string(out), `name          = "vm \"x\" $${evil}"`), string(out))
}

func TestRender_CustomStorageAndBridge(t *testing.T) {
	spec := baseSpec()
	spec.Storage = "local-lvm"
	spec.Bridge = "vmbr1"
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	assert.Contains(t, string(out), `datastore_id = "local-lvm"`)
	assert.Contains(t, string(out), `bridge = "vmbr1"`)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/terraform/ 2>&1 | head -5`
Expected: erro de compilação — pacote `terraform` não existe.

- [ ] **Step 3: Criar o template**

`backend/internal/terraform/main.tf.tmpl`:

```
terraform {
  required_providers {
    proxmox = {
      source  = "bpg/proxmox"
      version = "~> 0.112.0"
    }
  }
}

# Credenciais via env: PROXMOX_VE_ENDPOINT, PROXMOX_VE_API_TOKEN, PROXMOX_VE_INSECURE
provider "proxmox" {}

resource "proxmox_virtual_environment_vm" "vm" {
  node_name     = "{{ hcl .Node }}"
  name          = "{{ hcl .Name }}"
  machine       = "q35"
  scsi_hardware = "virtio-scsi-pci"
  started       = false
{{- if .Pool }}
  pool_id       = "{{ hcl .Pool }}"
{{- end }}
{{- if .Tags }}
  tags          = [{{ range $i, $t := .Tags }}{{ if $i }}, {{ end }}"{{ hcl $t }}"{{ end }}]
{{- end }}

  cpu {
    cores = {{ .CPUCores }}
  }

  memory {
    dedicated = {{ .MemoryMB }}
  }

  disk {
    datastore_id = "{{ hcl .Storage }}"
    interface    = "scsi0"
    size         = {{ .DiskGB }}
    file_format  = "raw"
  }

  network_device = [{
    model  = "virtio"
    bridge = "{{ hcl .Bridge }}"
  }]

  operating_system {
    type = "{{ hcl .OSType }}"
  }
{{- if .ISO }}

  cdrom {
    file_id   = "{{ hcl .ISO }}"
    interface = "ide2"
  }
  boot_order = ["ide2", "scsi0"]
{{- end }}
{{- if .GPUPCI }}

  hostpci {
    device = "hostpci0"
    id     = "{{ hcl .GPUPCI }}"
    pcie   = true
  }
{{- end }}
}

output "vmid" {
  value = proxmox_virtual_environment_vm.vm.vm_id
}
```

- [ ] **Step 4: Implementar `Render`**

`backend/internal/terraform/template.go`:

```go
package terraform

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/proxlab/backend/internal/domain/repository"
)

//go:embed main.tf.tmpl
var mainTemplateSrc string

var mainTemplate = template.Must(
	template.New("main.tf").Funcs(template.FuncMap{"hcl": hclString}).Parse(mainTemplateSrc),
)

const (
	defaultStorage = "local-zfs"
	defaultBridge  = "vmbr0"
)

// templateData é o que o template enxerga. Já com defaults aplicados.
type templateData struct {
	Node     string
	Name     string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Storage  string
	OSType   string
	ISO      string
	Bridge   string
	Pool     string
	Tags     []string
	GPUPCI   string // vazio = sem passthrough
}

// Render gera o main.tf para a spec. gpuPCI é o valor de PROXMOX_GPU_PCI
// (ex: "0000:01:00.0,pcie=1"); só a parte antes da vírgula é usada, e apenas
// quando spec.RequireGPU é true.
func Render(spec repository.VMSpec, node, gpuPCI string) ([]byte, error) {
	data := templateData{
		Node:     node,
		Name:     spec.Name,
		CPUCores: spec.CPUCores,
		MemoryMB: spec.MemoryMB,
		DiskGB:   spec.DiskGB,
		Storage:  spec.Storage,
		OSType:   spec.OSType,
		ISO:      spec.ISO,
		Bridge:   spec.Bridge,
		Pool:     spec.Pool,
		Tags:     spec.Tags,
	}
	if data.Storage == "" {
		data.Storage = defaultStorage
	}
	if data.Bridge == "" {
		data.Bridge = defaultBridge
	}
	if spec.RequireGPU && gpuPCI != "" {
		data.GPUPCI, _, _ = strings.Cut(gpuPCI, ",")
	}

	var buf bytes.Buffer
	if err := mainTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("erro ao renderizar template terraform: %w", err)
	}
	return buf.Bytes(), nil
}

// hclString escapa um valor para uso dentro de uma string HCL entre aspas.
// "${" vira "$${" para não ser interpretado como interpolação.
func hclString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "${", "$${")
	s = strings.ReplaceAll(s, "%{", "%%{")
	return s
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/terraform/ -v -run TestRender`
Expected: todos PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/terraform/
git commit -m "feat(terraform): template HCL da VM e Render com escape"
```

---

### Task 4: Runner (`Provisioner` via terraform-exec)

**Files:**
- Create: `backend/internal/terraform/runner.go`
- Test: `backend/internal/terraform/runner_test.go`

**Interfaces:**
- Consumes: `Render` (Task 3), `config.Config` (Task 1), `repository.Provisioner`/`ErrWorkspaceNotFound` (Task 2).
- Produces: `terraform.NewRunner(cfg *config.Config) (*Runner, error)`; `*Runner` implementa `repository.Provisioner`.

- [ ] **Step 1: Escrever os testes**

Os testes usam um script shell fake como `TERRAFORM_BIN`. O `terraform-exec` chama `terraform version -json` ao construir; o fake precisa responder a isso.

`backend/internal/terraform/runner_test.go`:

```go
package terraform_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTerraform escreve um script que responde a `version -json` e falha em
// qualquer outro subcomando, registrando os argumentos em <dir>/calls.log.
func fakeTerraform(t *testing.T, dir string, failApply bool) string {
	t.Helper()
	exit := "0"
	if failApply {
		exit = "1"
	}
	script := `#!/bin/sh
echo "$@" >> "` + filepath.Join(dir, "calls.log") + `"
case "$1" in
  version) echo '{"terraform_version":"1.8.3","platform":"linux_amd64","provider_selections":{},"terraform_outdated":false}' ;;
  init) exit 0 ;;
  apply) exit ` + exit + ` ;;
  destroy) exit 0 ;;
  output) echo '{"vmid":{"sensitive":false,"type":"number","value":123}}' ;;
  *) exit 0 ;;
esac
`
	path := filepath.Join(dir, "terraform")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func newTestRunner(t *testing.T, failApply bool) (*terraform.Runner, string) {
	t.Helper()
	dir := t.TempDir()
	bin := fakeTerraform(t, dir, failApply)
	cfg := &config.Config{
		ProxmoxHost:        "https://pve:8006",
		ProxmoxNode:        "pve01",
		ProxmoxTokenID:     "user@pam!tok",
		ProxmoxTokenSecret: "secret",
		TerraformBin:       bin,
		TerraformWorkdir:   filepath.Join(dir, "work"),
	}
	r, err := terraform.NewRunner(cfg)
	require.NoError(t, err)
	return r, dir
}

func TestNewRunner_CreatesWorkdirAndPluginCache(t *testing.T) {
	_, dir := newTestRunner(t, false)
	assert.DirExists(t, filepath.Join(dir, "work"))
	assert.DirExists(t, filepath.Join(dir, "work", ".plugin-cache"))
}

func TestProvision_WritesMainTFAndReturnsVMID(t *testing.T) {
	r, dir := newTestRunner(t, false)
	id := uuid.New()

	vmid, err := r.Provision(context.Background(), id, repository.VMSpec{Name: "vm", CPUCores: 1, MemoryMB: 512, DiskGB: 4, OSType: "l26"})

	require.NoError(t, err)
	assert.Equal(t, 123, vmid)
	assert.FileExists(t, filepath.Join(dir, "work", id.String(), "main.tf"))

	calls, _ := os.ReadFile(filepath.Join(dir, "calls.log"))
	assert.Contains(t, string(calls), "init")
	assert.Contains(t, string(calls), "apply")
	assert.Contains(t, string(calls), "output")
}

func TestProvision_ApplyFailsCleansWorkspace(t *testing.T) {
	r, dir := newTestRunner(t, true)
	id := uuid.New()

	_, err := r.Provision(context.Background(), id, repository.VMSpec{Name: "vm", CPUCores: 1, MemoryMB: 512, DiskGB: 4, OSType: "l26"})

	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(dir, "work", id.String()))
	calls, _ := os.ReadFile(filepath.Join(dir, "calls.log"))
	assert.Contains(t, string(calls), "destroy")
}

func TestDeprovision_MissingWorkspace(t *testing.T) {
	r, _ := newTestRunner(t, false)
	err := r.Deprovision(context.Background(), uuid.New())
	assert.ErrorIs(t, err, repository.ErrWorkspaceNotFound)
}

func TestDeprovision_RemovesWorkspace(t *testing.T) {
	r, dir := newTestRunner(t, false)
	id := uuid.New()
	_, err := r.Provision(context.Background(), id, repository.VMSpec{Name: "vm", CPUCores: 1, MemoryMB: 512, DiskGB: 4, OSType: "l26"})
	require.NoError(t, err)

	require.NoError(t, r.Deprovision(context.Background(), id))
	assert.NoDirExists(t, filepath.Join(dir, "work", id.String()))
}

func TestProvision_DoesNotWriteCredentialsToFile(t *testing.T) {
	r, dir := newTestRunner(t, false)
	id := uuid.New()
	_, err := r.Provision(context.Background(), id, repository.VMSpec{Name: "vm", CPUCores: 1, MemoryMB: 512, DiskGB: 4, OSType: "l26"})
	require.NoError(t, err)

	tf, _ := os.ReadFile(filepath.Join(dir, "work", id.String(), "main.tf"))
	assert.NotContains(t, string(tf), "secret")
	assert.NotContains(t, string(tf), "user@pam")
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/terraform/ 2>&1 | head -5`
Expected: erro de compilação — `terraform.Runner`/`NewRunner` não definidos.

- [ ] **Step 3: Implementar o Runner**

`backend/internal/terraform/runner.go`:

```go
package terraform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/domain/repository"
)

const (
	operationTimeout = 5 * time.Minute
	pluginCacheDir   = ".plugin-cache"
)

// Runner implementa repository.Provisioner executando o binário terraform
// em um workspace por VM: <root>/<vm-uuid>/{main.tf, terraform.tfstate}.
type Runner struct {
	execPath string
	root     string
	node     string
	gpuPCI   string
	env      map[string]string // env completo passado ao processo terraform
}

// NewRunner resolve o binário, cria a raiz dos workspaces e o cache de providers.
func NewRunner(cfg *config.Config) (*Runner, error) {
	if err := os.MkdirAll(filepath.Join(cfg.TerraformWorkdir, pluginCacheDir), 0o750); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório de trabalho do terraform: %w", err)
	}

	env := inheritedEnv()
	env["TF_PLUGIN_CACHE_DIR"] = filepath.Join(cfg.TerraformWorkdir, pluginCacheDir)
	env["PROXMOX_VE_ENDPOINT"] = cfg.ProxmoxHost
	env["PROXMOX_VE_API_TOKEN"] = cfg.ProxmoxTokenID + "=" + cfg.ProxmoxTokenSecret
	env["PROXMOX_VE_INSECURE"] = "true"

	return &Runner{
		execPath: cfg.TerraformBin,
		root:     cfg.TerraformWorkdir,
		node:     cfg.ProxmoxNode,
		gpuPCI:   cfg.ProxmoxGPUPCI,
		env:      env,
	}, nil
}

// inheritedEnv copia os.Environ sem as variáveis que o terraform-exec
// proíbe em SetEnv (TF_LOG, TF_INPUT, TF_VAR_*, etc.).
func inheritedEnv() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		env[k] = v
	}
	for _, k := range tfexec.ProhibitedEnv(env) {
		delete(env, k)
	}
	return env
}

func (r *Runner) workspacePath(id uuid.UUID) string {
	return filepath.Join(r.root, id.String())
}

// newTF cria a instância terraform-exec apontando para o workspace.
func (r *Runner) newTF(dir string) (*tfexec.Terraform, error) {
	tf, err := tfexec.NewTerraform(dir, r.execPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao iniciar terraform: %w", err)
	}
	if err := tf.SetEnv(r.env); err != nil {
		return nil, fmt.Errorf("erro ao configurar ambiente do terraform: %w", err)
	}
	tf.SetStderr(logWriter{prefix: "[terraform] "})
	return tf, nil
}

// Provision escreve main.tf, roda init+apply e lê o output vmid.
// Em falha de apply, tenta destroy e remove o workspace para não deixar lixo.
func (r *Runner) Provision(ctx context.Context, workspaceID uuid.UUID, spec repository.VMSpec) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	dir := r.workspacePath(workspaceID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, fmt.Errorf("erro ao criar workspace: %w", err)
	}

	hcl, err := Render(spec, r.node, r.gpuPCI)
	if err != nil {
		_ = os.RemoveAll(dir)
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), hcl, 0o640); err != nil {
		_ = os.RemoveAll(dir)
		return 0, fmt.Errorf("erro ao escrever main.tf: %w", err)
	}

	tf, err := r.newTF(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return 0, err
	}

	if err := tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		_ = os.RemoveAll(dir)
		return 0, fmt.Errorf("terraform init falhou: %w", err)
	}

	if err := tf.Apply(ctx); err != nil {
		log.Printf("[terraform] apply falhou no workspace %s, executando destroy best-effort", workspaceID)
		if derr := tf.Destroy(ctx); derr != nil {
			log.Printf("[terraform] destroy após falha também falhou: %v", derr)
		}
		_ = os.RemoveAll(dir)
		return 0, fmt.Errorf("terraform apply falhou: %w", err)
	}

	outputs, err := tf.Output(ctx)
	if err != nil {
		return 0, fmt.Errorf("erro ao ler outputs do terraform: %w", err)
	}
	raw, ok := outputs["vmid"]
	if !ok {
		return 0, errors.New("output vmid ausente no terraform")
	}
	var vmid int
	if err := json.Unmarshal(raw.Value, &vmid); err != nil {
		return 0, fmt.Errorf("output vmid inválido: %w", err)
	}
	return vmid, nil
}

// Deprovision roda destroy e remove o workspace.
func (r *Runner) Deprovision(ctx context.Context, workspaceID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	dir := r.workspacePath(workspaceID)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return repository.ErrWorkspaceNotFound
		}
		return fmt.Errorf("erro ao acessar workspace: %w", err)
	}

	tf, err := r.newTF(dir)
	if err != nil {
		return err
	}
	if err := tf.Init(ctx, tfexec.Upgrade(false)); err != nil {
		return fmt.Errorf("terraform init falhou: %w", err)
	}
	if err := tf.Destroy(ctx); err != nil {
		return fmt.Errorf("terraform destroy falhou: %w", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("erro ao remover workspace: %w", err)
	}
	return nil
}

// logWriter encaminha stderr do terraform para o log da aplicação.
type logWriter struct{ prefix string }

func (w logWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			log.Print(w.prefix + line)
		}
	}
	return len(p), nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/terraform/ -v`
Expected: todos PASS (template + runner).

- [ ] **Step 5: Confirmar que Runner satisfaz a interface**

Adicionar no fim de `runner.go`:

```go
var _ repository.Provisioner = (*Runner)(nil)
```

Run: `cd backend && go build ./... && go vet ./...`
Expected: sem erros.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/terraform/
git commit -m "feat(terraform): Runner implementa Provisioner via terraform-exec"
```

---

### Task 5: Use case usa `Provisioner`; limpeza do cliente REST

**Files:**
- Modify: `backend/internal/usecase/vm_usecase.go`
- Modify: `backend/internal/usecase/vm_usecase_test.go`
- Modify: `backend/internal/domain/repository/proxmox_client.go`
- Modify: `backend/internal/proxmox/client.go`
- Modify: `backend/internal/proxmox/types.go`
- Modify: `backend/internal/handler/vm_handler.go:23-64`
- Modify: `backend/cmd/server/main.go` (mínimo para compilar; wiring final na Task 6)

**Interfaces:**
- Consumes: `repository.Provisioner`, `repository.VMSpec`, `repository.ErrWorkspaceNotFound` (Task 2).
- Produces: `usecase.NewVMUseCase(vmRepo repository.VMRepository, proxmoxClient repository.ProxmoxClientInterface, provisioner repository.Provisioner, node string) *VMUseCase`; `CreateVMRequest` sem campo `Node`.

- [ ] **Step 1: Reescrever os mocks e testes de CreateVM/DeleteVM**

Em `backend/internal/usecase/vm_usecase_test.go`:

Substituir o bloco `// ─── Mock: ProxmoxClientInterface` inteiro (do `type mockProxmoxClient` até o fim de `GetTaskStatus`) por:

```go
// ─── Mock: ProxmoxClientInterface ────────────────────────────────────────────

type mockProxmoxClient struct{ mock.Mock }

func (m *mockProxmoxClient) GetVMStatus(ctx context.Context, vmid int) (proxmox.VMStatus, error) {
	args := m.Called(ctx, vmid)
	return args.Get(0).(proxmox.VMStatus), args.Error(1)
}

func (m *mockProxmoxClient) StartVM(ctx context.Context, vmid int) (string, error) {
	args := m.Called(ctx, vmid)
	return args.String(0), args.Error(1)
}

func (m *mockProxmoxClient) StopVM(ctx context.Context, vmid int) (string, error) {
	args := m.Called(ctx, vmid)
	return args.String(0), args.Error(1)
}

func (m *mockProxmoxClient) GetTermProxy(ctx context.Context, vmid int) (proxmox.TermProxyResponse, error) {
	args := m.Called(ctx, vmid)
	return args.Get(0).(proxmox.TermProxyResponse), args.Error(1)
}

// ─── Mock: Provisioner ───────────────────────────────────────────────────────

type mockProvisioner struct{ mock.Mock }

func (m *mockProvisioner) Provision(ctx context.Context, workspaceID uuid.UUID, spec repository.VMSpec) (int, error) {
	args := m.Called(ctx, workspaceID, spec)
	return args.Int(0), args.Error(1)
}

func (m *mockProvisioner) Deprovision(ctx context.Context, workspaceID uuid.UUID) error {
	return m.Called(ctx, workspaceID).Error(0)
}
```

Adicionar `"github.com/proxlab/backend/internal/domain/repository"` aos imports.

Substituir o helper `newVMUC` e `baseCreateReq`:

```go
func newVMUC(vmRepo *mockVMRepo, px *mockProxmoxClient, prov *mockProvisioner) *usecase.VMUseCase {
	return usecase.NewVMUseCase(vmRepo, px, prov, "pve01")
}

func baseCreateReq() usecase.CreateVMRequest {
	return usecase.CreateVMRequest{
		Name:       "minha-vm",
		CPUCores:   1,
		MemoryMB:   1024,
		DiskGB:     4,
		OSTemplate: "ubuntu-22.04",
		Pool:       "students",
		Tags:       []string{"proxlab", "aluno"},
		Role:       "aluno",
	}
}
```

Em **todos** os testes existentes, trocar `px := new(mockProxmoxClient)` + `uc := newVMUC(vmRepo, px)` por:

```go
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	uc := newVMUC(vmRepo, px, prov)
```

e trocar `px.AssertNotCalled(t, "CreateVM")` por `prov.AssertNotCalled(t, "Provision")`.

Substituir `TestCreateVM_PesquisadorWithinLimit`, `TestCreateVM_ProxmoxError`, `TestDeleteVM_Success` e `TestDeleteVM_AnotherUsersVM` por:

```go
// Cenário: pesquisador com 8 CPUs — dentro do limite → provisiona e persiste com o VMID retornado.
func TestCreateVM_PesquisadorWithinLimit(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	prov.On("Provision", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.MatchedBy(func(s repository.VMSpec) bool {
		return s.Name == "minha-vm" && s.CPUCores == 8 && s.MemoryMB == 16384 && s.OSType == "l26" && s.Pool == "researchers"
	})).Return(101, nil)
	vmRepo.On("Create", mock.Anything, mock.MatchedBy(func(vm *entity.VM) bool {
		return vm.ProxmoxVMID == 101 && vm.Node == "pve01" && vm.Status == entity.VMStatusCreating && vm.Tags == "proxlab;aluno"
	})).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	req := baseCreateReq()
	req.CPUCores = 8
	req.MemoryMB = 16384
	req.Pool = "researchers"
	req.Role = "pesquisador"

	vm, err := uc.CreateVM(context.Background(), uuid.New(), req)

	assert.NoError(t, err)
	assert.NotNil(t, vm)
	assert.Equal(t, 101, vm.ProxmoxVMID)
	prov.AssertExpectations(t)
	vmRepo.AssertExpectations(t)
}

// Cenário: o UUID passado ao Provision é o mesmo ID da entidade persistida.
func TestCreateVM_WorkspaceIDMatchesEntityID(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	var provisionedID uuid.UUID
	prov.On("Provision", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.Anything).
		Run(func(args mock.Arguments) { provisionedID = args.Get(1).(uuid.UUID) }).
		Return(102, nil)
	vmRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.VM")).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	vm, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.NoError(t, err)
	assert.Equal(t, provisionedID, vm.ID)
}

// Cenário: Provision falha → erro propagado, nada persistido.
func TestCreateVM_ProvisionError(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).Return(0, errors.New("terraform apply falhou"))

	uc := newVMUC(vmRepo, px, prov)
	_, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.Error(t, err)
	vmRepo.AssertNotCalled(t, "Create")
}

// Cenário: Provision ok mas banco falha → Deprovision best-effort para não deixar VM órfã.
func TestCreateVM_RepoErrorTriggersDeprovision(t *testing.T) {
	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)

	prov.On("Provision", mock.Anything, mock.Anything, mock.Anything).Return(103, nil)
	vmRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db down"))
	prov.On("Deprovision", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	_, err := uc.CreateVM(context.Background(), uuid.New(), baseCreateReq())

	assert.Error(t, err)
	prov.AssertCalled(t, "Deprovision", mock.Anything, mock.AnythingOfType("uuid.UUID"))
}

// ─── Testes: DeleteVM ─────────────────────────────────────────────────────────

// Cenário: dono da VM chama Delete → Deprovision e remove do banco.
func TestDeleteVM_Success(t *testing.T) {
	userID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: userID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)
	prov.On("Deprovision", mock.Anything, vmID).Return(nil)
	vmRepo.On("Delete", mock.Anything, vmID).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), userID, vmID)

	assert.NoError(t, err)
	prov.AssertExpectations(t)
	vmRepo.AssertExpectations(t)
}

// Cenário: VM pré-migração (sem workspace) → aviso em log, ainda remove do banco.
func TestDeleteVM_MissingWorkspaceStillDeletesRecord(t *testing.T) {
	userID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: userID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)
	prov.On("Deprovision", mock.Anything, vmID).Return(repository.ErrWorkspaceNotFound)
	vmRepo.On("Delete", mock.Anything, vmID).Return(nil)

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), userID, vmID)

	assert.NoError(t, err)
	vmRepo.AssertCalled(t, "Delete", mock.Anything, vmID)
}

// Cenário: Deprovision falha por outro motivo → não remove do banco.
func TestDeleteVM_DeprovisionErrorKeepsRecord(t *testing.T) {
	userID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: userID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)
	prov.On("Deprovision", mock.Anything, vmID).Return(errors.New("terraform destroy falhou"))

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), userID, vmID)

	assert.Error(t, err)
	vmRepo.AssertNotCalled(t, "Delete")
}

// Cenário: usuário tenta deletar VM de outro → ErrAccessDenied, sem Deprovision.
func TestDeleteVM_AnotherUsersVM(t *testing.T) {
	ownerID := uuid.New()
	vmID := uuid.New()
	storedVM := &entity.VM{ID: vmID, UserID: ownerID, ProxmoxVMID: 300}

	vmRepo := new(mockVMRepo)
	px := new(mockProxmoxClient)
	prov := new(mockProvisioner)
	vmRepo.On("FindByID", mock.Anything, vmID).Return(storedVM, nil)

	uc := newVMUC(vmRepo, px, prov)
	err := uc.DeleteVM(context.Background(), uuid.New(), vmID)

	assert.ErrorIs(t, err, usecase.ErrAccessDenied)
	prov.AssertNotCalled(t, "Deprovision")
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/usecase/ 2>&1 | head -5`
Expected: erro de compilação (`NewVMUseCase` com assinatura antiga, `Node` inexistente etc.).

- [ ] **Step 3: Reduzir `ProxmoxClientInterface`**

Substituir o conteúdo de `backend/internal/domain/repository/proxmox_client.go` por:

```go
package repository

import (
	"context"

	"github.com/proxlab/backend/internal/proxmox"
)

// ProxmoxClientInterface é o port para operações de runtime na API REST do Proxmox VE
// (status, start, stop, terminal). Criação e destruição ficam em Provisioner.
// Implementado por internal/proxmox.ProxmoxClient via duck typing.
type ProxmoxClientInterface interface {
	// GetVMStatus retorna o estado atual de runtime de uma VM.
	// Proxmox: GET /nodes/{node}/qemu/{vmid}/status/current
	GetVMStatus(ctx context.Context, vmid int) (proxmox.VMStatus, error)

	// StartVM inicia uma VM parada de forma assíncrona.
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/start
	// Retorna o UPID da tarefa.
	StartVM(ctx context.Context, vmid int) (taskID string, err error)

	// StopVM para uma VM em execução (hard stop imediato).
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/stop
	// Retorna o UPID da tarefa.
	StopVM(ctx context.Context, vmid int) (taskID string, err error)

	// GetTermProxy abre um terminal de texto para a VM via Proxmox termproxy.
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/termproxy
	// Retorna ticket e porta para conexão WebSocket direta ao nó Proxmox.
	GetTermProxy(ctx context.Context, vmid int) (proxmox.TermProxyResponse, error)
}
```

- [ ] **Step 4: Remover métodos legados de `proxmox/client.go`**

Remover inteiros, com seus comentários: `nextVMID`, `CreateVM`, `DeleteVM`, `GetVNCProxy`, `ListVMs`, `GetTaskStatus`. Remover o campo `gpuPCI` do struct `ProxmoxClient` e a linha `gpuPCI: cfg.ProxmoxGPUPCI,` em `NewProxmoxClient`. Remover o import `"strconv"` (deixa de ser usado). Manter `doRequest`, `decodeBody`, `GetVMStatus`, `StartVM`, `StopVM`, `GetTermProxy`, `WsBase`, `Node`, `AuthHeader`, `parseHostname`.

Ao final, `client.go` deve conter apenas: imports (`context`, `crypto/tls`, `encoding/json`, `fmt`, `io`, `log`, `net/http`, `net/url`, `strings`, `config`), `parseHostname`, `ProxmoxClient` struct (sem `gpuPCI`), `WsBase`, `Node`, `AuthHeader`, `NewProxmoxClient`, `doRequest`, `decodeBody`, `GetVMStatus`, `StartVM`, `StopVM`, `GetTermProxy`.

- [ ] **Step 5: Remover tipos legados de `proxmox/types.go`**

Remover: `VMCreateRequest`, `VNCProxyResponse`, `TaskStatus`, `pveVNCProxy`, `pveQEMUEntry`, `pveTaskStatus`. Manter `VMStatus`, `TermProxyResponse`, `pveEnvelope`, `pveVMStatus`, `pveTermProxy`, `flexInt`.

- [ ] **Step 6: Reescrever o use case**

Em `backend/internal/usecase/vm_usecase.go`:

Imports: adicionar `"log"`; remover `"github.com/proxlab/backend/internal/proxmox"`.

Substituir `CreateVMRequest`, `VMUseCase`, `NewVMUseCase`, `CreateVM` e `DeleteVM` por:

```go
// CreateVMRequest é o input para criação de VM, incluindo role para validação de limites.
type CreateVMRequest struct {
	Name       string
	CPUCores   int
	MemoryMB   int
	DiskGB     int
	OSTemplate string
	Pool       string   // "researchers" ou "students"
	Tags       []string // ex: ["proxlab","aluno"]
	Role       string   // role do usuário autenticado, para validar limites
	RequireGPU bool
}

type VMUseCase struct {
	vmRepo        repository.VMRepository
	proxmoxClient repository.ProxmoxClientInterface
	provisioner   repository.Provisioner
	node          string // nó Proxmox onde as VMs são criadas (PROXMOX_NODE)
}

func NewVMUseCase(
	vmRepo repository.VMRepository,
	proxmoxClient repository.ProxmoxClientInterface,
	provisioner repository.Provisioner,
	node string,
) *VMUseCase {
	return &VMUseCase{vmRepo: vmRepo, proxmoxClient: proxmoxClient, provisioner: provisioner, node: node}
}

// CreateVM valida limites por role, provisiona a VM via Terraform e persiste no banco.
// O UUID da entidade é gerado antes do provisionamento e usado como ID do workspace.
func (uc *VMUseCase) CreateVM(ctx context.Context, userID uuid.UUID, req CreateVMRequest) (*entity.VM, error) {
	if err := validateRoleLimits(req.Role, req.CPUCores, req.MemoryMB); err != nil {
		return nil, err
	}

	id := uuid.New()
	vmid, err := uc.provisioner.Provision(ctx, id, repository.VMSpec{
		Name:       req.Name,
		CPUCores:   req.CPUCores,
		MemoryMB:   req.MemoryMB,
		DiskGB:     req.DiskGB,
		OSType:     osTypeFromTemplate(req.OSTemplate),
		Pool:       req.Pool,
		Tags:       req.Tags,
		RequireGPU: req.RequireGPU,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao provisionar VM: %w", err)
	}

	vm := &entity.VM{
		ID:          id,
		UserID:      userID,
		ProxmoxVMID: vmid,
		Name:        req.Name,
		CPUCores:    req.CPUCores,
		MemoryMB:    req.MemoryMB,
		DiskGB:      req.DiskGB,
		OSTemplate:  req.OSTemplate,
		Node:        uc.node,
		Pool:        req.Pool,
		Tags:        strings.Join(req.Tags, ";"),
		RequireGPU:  req.RequireGPU,
		Status:      entity.VMStatusCreating,
	}

	if err := uc.vmRepo.Create(ctx, vm); err != nil {
		// VM já existe no Proxmox; desfaz para não deixar recurso órfão
		if derr := uc.provisioner.Deprovision(ctx, id); derr != nil {
			log.Printf("[vm] falha ao desprovisionar VM %d após erro no banco: %v", vmid, derr)
		}
		return nil, fmt.Errorf("erro ao salvar VM no banco: %w", err)
	}

	return vm, nil
}
```

```go
// DeleteVM destrói a VM via Terraform e remove do banco.
// VMs criadas antes da migração não têm workspace: o registro é removido e a VM
// fica no Proxmox para remoção manual.
func (uc *VMUseCase) DeleteVM(ctx context.Context, userID, vmID uuid.UUID) error {
	vm, err := uc.findOwned(ctx, userID, vmID)
	if err != nil {
		return err
	}
	if err := uc.provisioner.Deprovision(ctx, vm.ID); err != nil {
		if !errors.Is(err, repository.ErrWorkspaceNotFound) {
			return fmt.Errorf("erro ao desprovisionar VM: %w", err)
		}
		log.Printf("[vm] VM %d (id=%s) sem workspace terraform; removendo só o registro — apagar manualmente no Proxmox", vm.ProxmoxVMID, vm.ID)
	}
	return uc.vmRepo.Delete(ctx, vm.ID)
}
```

- [ ] **Step 7: Remover `Node` do handler**

Em `backend/internal/handler/vm_handler.go`: remover a linha `Node       string   \`json:"node"\`` do struct `createVMRequest` e a linha `Node:       req.Node,` da chamada a `CreateVM`. Atualizar o comentário godoc do `Create` para remover `"node": "pve"` do exemplo de body.

- [ ] **Step 8: Ajuste mínimo em `main.go` para compilar**

Em `backend/cmd/server/main.go`, trocar a linha `vmUC := usecase.NewVMUseCase(vmRepo, proxmoxClient)` por:

```go
	provisioner, err := terraform.NewRunner(cfg)
	if err != nil {
		log.Fatalf("erro ao iniciar terraform runner: %v", err)
	}

	vmUC := usecase.NewVMUseCase(vmRepo, proxmoxClient, provisioner, cfg.ProxmoxNode)
```

e adicionar o import `"github.com/proxlab/backend/internal/terraform"`.

- [ ] **Step 9: Rodar tudo**

Run: `cd backend && go build ./... && go vet ./... && go test ./internal/... -count=1`
Expected: build sem erros; todos os testes PASS. `go vet` vai falhar em `cmd/proxmox-check` — isso é esperado e corrigido na Task 7; por enquanto rode `go vet ./internal/... ./cmd/server/...`.

- [ ] **Step 10: Commit**

```bash
git add backend/internal backend/cmd/server/main.go
git commit -m "feat(usecase): criar/destruir VM via Provisioner; remover caminho REST de criação e métodos sem uso"
```

---

### Task 6: Docker, compose e artefatos rastreados

**Files:**
- Modify: `backend/Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `.gitignore`
- Delete (git rm): `backend/proxmox-check`

- [ ] **Step 1: Dockerfile — copiar binário terraform e criar workdir**

Substituir o stage runtime de `backend/Dockerfile` por:

```dockerfile
# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM hashicorp/terraform:1.9 AS terraform

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser && \
    mkdir -p /var/lib/proxlab/terraform && \
    chown appuser:appuser /var/lib/proxlab/terraform

WORKDIR /app

COPY --from=terraform /bin/terraform /usr/local/bin/terraform
COPY --from=builder /app /app/server

USER appuser

ENV TERRAFORM_BIN=/usr/local/bin/terraform \
    TERRAFORM_WORKDIR=/var/lib/proxlab/terraform

EXPOSE 8080

CMD ["/app/server"]
```

- [ ] **Step 2: compose — volume dos workspaces**

Em `docker-compose.yml`, no serviço `backend`, adicionar após `environment:`:

```yaml
    volumes:
      - terraform_data:/var/lib/proxlab/terraform
```

E em `volumes:` (raiz), adicionar `terraform_data:` abaixo de `postgres_data:`.

- [ ] **Step 3: Remover binário rastreado e ignorar builds**

```bash
git rm --cached backend/proxmox-check && rm -f backend/proxmox-check
```

Adicionar ao final de `.gitignore`:

```
# binários Go
bin/
backend/proxmox-check
backend/server
```

- [ ] **Step 4: Verificar build da imagem**

Run: `docker compose build backend 2>&1 | tail -3 && docker compose run --rm --no-deps backend terraform version`
Expected: imagem constrói; saída `Terraform v1.9.x`.

Se o Docker não estiver disponível, registrar isso no relatório da task e seguir.

- [ ] **Step 5: Commit**

```bash
git add backend/Dockerfile docker-compose.yml .gitignore
git commit -m "build: binário terraform na imagem, volume de workspaces, ignorar binários"
```

---

### Task 7: Smoke test `cmd/proxmox-check` com o fluxo novo

**Files:**
- Rewrite: `backend/cmd/proxmox-check/main.go`

**Interfaces:**
- Consumes: `terraform.NewRunner`, `Runner.Provision/Deprovision` (Task 4); `proxmox.NewProxmoxClient`, `GetVMStatus` (Task 5).

- [ ] **Step 1: Reescrever a ferramenta**

Substituir todo o conteúdo de `backend/cmd/proxmox-check/main.go` por:

```go
// Smoke test do provisionamento via Terraform contra o Proxmox real.
// Cria UMA VM de teste (proxlab-smoke-<sufixo>), consulta o status via REST
// e a destrói. Nunca toca em VMs existentes.
//
// Uso: go run ./cmd/proxmox-check  (requer .env com PROXMOX_* e opcionalmente TERRAFORM_*)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/proxmox"
	"github.com/proxlab/backend/internal/terraform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("configuração: %v", err)
	}
	if os.Getenv("TERRAFORM_WORKDIR") == "" {
		cfg.TerraformWorkdir = os.TempDir() + "/proxlab-smoke"
	}

	fmt.Printf("Proxmox %s (nó %s) · terraform=%s · workdir=%s\n\n",
		cfg.ProxmoxHost, cfg.ProxmoxNode, cfg.TerraformBin, cfg.TerraformWorkdir)

	runner, err := terraform.NewRunner(cfg)
	if err != nil {
		fatalf("runner: %v", err)
	}
	client := proxmox.NewProxmoxClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	id := uuid.New()
	spec := repository.VMSpec{
		Name:     "proxlab-smoke-" + id.String()[:8],
		CPUCores: 1,
		MemoryMB: 512,
		DiskGB:   4,
		OSType:   "l26",
		Tags:     []string{"proxlab", "smoke-test"},
	}

	fmt.Printf("▶ 1/3 Provision (%s)\n", spec.Name)
	vmid, err := runner.Provision(ctx, id, spec)
	if err != nil {
		fatalf("Provision: %v", err)
	}
	fmt.Printf("  ok — VMID=%d workspace=%s\n\n", vmid, id)

	fmt.Printf("▶ 2/3 GetVMStatus(%d) via REST\n", vmid)
	status, err := client.GetVMStatus(ctx, vmid)
	if err != nil {
		fmt.Printf("  falhou: %v (seguindo para destroy)\n\n", err)
	} else {
		fmt.Printf("  ok — status=%s name=%s\n\n", status.Status, status.Name)
	}

	fmt.Printf("▶ 3/3 Deprovision(%s)\n", id)
	if err := runner.Deprovision(ctx, id); err != nil {
		fatalf("Deprovision: %v — VMID=%d precisa ser removida manualmente!", err, vmid)
	}
	fmt.Println("  ok — VM destruída e workspace removido")
	fmt.Println("\nSmoke test concluído.")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERRO: "+format+"\n", args...)
	os.Exit(1)
}
```

`config.Load()` exige `JWT_SECRET`; o `.env` do projeto já o tem. Se rodar fora do projeto, exporte um valor qualquer.

- [ ] **Step 2: Verificar build e vet completos**

Run: `cd backend && go build ./... && go vet ./... && go test ./... -count=1`
Expected: tudo OK, sem referências a métodos removidos.

- [ ] **Step 3: Rodar o smoke test contra o Proxmox real (verificação manual)**

Só com `.env` válido e autorização do usuário. Run: `cd backend && go run ./cmd/proxmox-check`
Expected: as 3 etapas com `ok`; ao final, nenhuma VM `proxlab-smoke-*` restante no Proxmox.

Se falhar no `Provision`, ler o stderr do terraform no log (`[terraform] ...`) — causas comuns: storage `local-zfs` inexistente (ajustar `Storage` na spec do smoke), token sem permissão `VM.Allocate`, provider sem acesso à internet para download.

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/proxmox-check/main.go
git commit -m "chore(proxmox-check): smoke test do provisionamento via Terraform"
```

---

### Task 8: Documentação — CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Atualizar seções**

Em `CLAUDE.md`:

"Estrutura" — adicionar após a linha `│   ├── proxmox/                # cliente API Proxmox`:

```
│   ├── terraform/              # Provisioner: gera main.tf e roda terraform-exec
```

e ajustar o comentário de `proxmox/` para `# cliente REST Proxmox (status/start/stop/termproxy)`.

"Fluxo obrigatório" — substituir a linha do Proxmox por:

```
Handler → UseCase → Provisioner Interface → terraform-exec (provider bpg/proxmox) → Proxmox   (criar/destruir VM)
Handler → UseCase → ProxmoxClient Interface → Proxmox API REST                                (status/start/stop/terminal)
```

"Proxmox" — adicionar ao final da lista:

```
- Provisionamento: Terraform `bpg/proxmox ~> 0.112.0`; um workspace por VM em `$TERRAFORM_WORKDIR/<vm-uuid>`; credenciais só via env do processo (`PROXMOX_VE_ENDPOINT`, `PROXMOX_VE_API_TOKEN`, `PROXMOX_VE_INSECURE`)
- Env Terraform: `TERRAFORM_BIN`, `TERRAFORM_WORKDIR`
- VMs criadas antes da migração não têm workspace: ao deletar, só o registro sai do banco; a VM é removida manualmente no Proxmox
```

"Fases" — trocar `6. Migração do provisionamento para Terraform (futura, ver Pendências)` por `6. Migração do provisionamento para Terraform ← ✅ concluído`.

"Pendências" — trocar `- [ ]` por `- [x]` e remover a frase `Ajuste em execução via modelo de IA Fable.`.

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: CLAUDE.md reflete provisionamento via Terraform"
```

---

## Self-review

- **Cobertura do spec:** porta de domínio (T2), template + escape + GPU (T3), runner + timeout + env + cleanup (T4), use case + fallback de workspace ausente + rollback em falha de banco (T5), limpeza de legado incl. binário e `.gitignore` (T5, T6), Docker/compose/env (T1, T6), smoke test (T7), CLAUDE.md (T8). `swagger.yaml` inalterado — o campo `node` do payload passa a ser ignorado; se o swagger o documentar, remova a propriedade na T5 Step 7.
- **Tipos consistentes:** `Provision(ctx, uuid.UUID, repository.VMSpec) (int, error)`, `Deprovision(ctx, uuid.UUID) error`, `NewVMUseCase(vmRepo, proxmoxClient, provisioner, node)`, `Render(spec, node, gpuPCI)`, `NewRunner(cfg)` usados igual em todas as tasks.
- **Sem placeholders.**
