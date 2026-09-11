# Containers em k3s — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Usuários criam containers de aplicação (imagem + CPU/RAM + porta + env + volume) num cluster k3s single-node provisionado pelo ProxLab, com terminal web e logs em tempo real.

**Architecture:** `cmd/k3s-bootstrap` cria a VM do cluster via o `Provisioner` Terraform existente (cloud-init com chave SSH + IP fixo) e instala k3s por SSH. O backend fala com o cluster por uma nova porta de domínio `ContainerRuntime`, implementada em `internal/kube/` com `client-go` (server-side apply). Status vem do cluster a cada request; o Postgres guarda só a definição do container.

**Tech Stack:** Go 1.25, Gin, GORM/Postgres, terraform-exec + bpg/proxmox, `k8s.io/client-go`, `golang.org/x/crypto/ssh`, Next.js 16 + xterm.js.

**Spec:** `docs/superpowers/specs/2026-09-10-containers-k3s-design.md`

## Global Constraints

- Fluxo obrigatório: Handler → UseCase → Repository/Runtime Interface → implementação. Nunca GORM ou client-go em handlers; nunca lógica em `main.go`.
- Sempre `if err != nil`, `context.Context` em I/O, validar payload nos handlers, UUIDs como PK.
- Nunca expor erros internos (k8s/SSH/terraform) ao cliente HTTP; logar o erro real e responder mensagem genérica.
- Frontend: não alterar CSS/layout. Exceção acordada: `/containers/criar` tem campos adicionados/removidos reutilizando exatamente as classes existentes.
- Nomes de container: `^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`. Namespace do usuário: `"u-" + primeiros 8 chars do UserID`.
- NodePort: 30000–32767, unique em `containers.node_port` quando `> 0`, até 5 tentativas.
- Limites por perfil (usados em VMs e containers): aluno 2 CPU / 4096 MB / 5 containers / 20 GB; pesquisador 8 CPU / 16384 MB / 20 containers / 100 GB.
- Testes: `cd backend && go test ./...` deve passar ao fim de cada task. Commits pequenos, mensagens em português no padrão `tipo(escopo): descrição`.
- Regra de Proxmox produção: smoke tests só tocam recursos criados por eles mesmos.
- Desvio registrado em relação à spec: o `Provisioner` continua recebendo `uuid.UUID`; o bootstrap usa o UUID determinístico `uuid.NewSHA1(uuid.NameSpaceOID, []byte("proxlab-k3s"))` como workspace, evitando alterar o código de VMs.

## File Structure

```
backend/
├── cmd/k3s-bootstrap/main.go                 bootstrap: só parse de env + chama internal/k3s
├── cmd/proxmox-check/main.go                 ganha flag --k3s
├── internal/config/config.go                 novas envs K3S_*, INGRESS_DOMAIN, KUBECONFIG_PATH
├── internal/domain/entity/container.go       entidade + EnvMap (jsonb)
├── internal/domain/entity/catalog.go         catálogo estático
├── internal/domain/repository/container_repository.go
├── internal/domain/repository/container_runtime.go   porta ContainerRuntime + tipos
├── internal/domain/repository/provisioner.go VMSpec ganha SSH keys + IP fixo
├── internal/repository/container_repository.go       GORM
├── internal/terraform/main.tf.tmpl           user_account.keys + ip_config estático
├── internal/terraform/template.go
├── internal/kube/client.go                   New(kubeconfigPath) → *Runtime
├── internal/kube/namespace.go                EnsureNamespace
├── internal/kube/workload.go                 Apply / Delete
├── internal/kube/status.go                   Status
├── internal/kube/exec.go                     Exec
├── internal/kube/logs.go                     Logs
├── internal/kube/pod.go                      findRunningPod (compartilhado por exec/logs)
├── internal/k3s/keys.go                      chave ed25519
├── internal/k3s/ssh.go                       waitSSH, runScript
├── internal/k3s/install.go                   Install(ctx, opts): fluxo completo
├── internal/k3s/manifests/*.yaml             embed
├── internal/usecase/limits.go                roleLimits extraído + containers/storage
├── internal/usecase/container_usecase.go
├── internal/handler/container_handler.go
├── internal/websocket/common.go              upgrader + ownerFromContext extraídos
├── internal/websocket/container_terminal.go
├── internal/websocket/container_logs.go
└── migrations/003_create_containers.sql
frontend/src/lib/api.ts                       tipos + funções de containers
frontend/src/app/(provisionamento)/containers/{page,criar/page,[id]/page,[id]/utilizar/page}.tsx
```

---

### Task 1: Config e limites por perfil

**Files:**
- Modify: `backend/internal/config/config.go`
- Create: `backend/internal/usecase/limits.go`
- Create: `backend/internal/usecase/limits_test.go`
- Modify: `backend/internal/usecase/vm_usecase.go` (remover `roleLimits` e `validateRoleLimits`)
- Modify: `.env.example` (ignorado pelo git; editar à mão)

**Interfaces:**
- Produces: `config.Config` campos `K3SNodeIP, K3SNodeGateway string; K3SNodeCPU, K3SNodeMemoryMB, K3SNodeDiskGB int; IngressDomain, KubeconfigPath string`.
- Produces: `usecase.RoleLimits{MaxCPU, MaxMemMB, MaxContainers, MaxStorageGB int}`, `usecase.LimitsFor(role string) RoleLimits`, `usecase.validateRoleLimits(role string, cpuCores, memoryMB int) error` (mantido para VMs).

- [ ] **Step 1: Teste dos limites**

`backend/internal/usecase/limits_test.go`:
```go
package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitsFor(t *testing.T) {
	assert.Equal(t, RoleLimits{MaxCPU: 2, MaxMemMB: 4096, MaxContainers: 5, MaxStorageGB: 20}, LimitsFor("aluno"))
	assert.Equal(t, RoleLimits{MaxCPU: 8, MaxMemMB: 16384, MaxContainers: 20, MaxStorageGB: 100}, LimitsFor("pesquisador"))
	assert.Equal(t, LimitsFor("aluno"), LimitsFor("desconhecido"), "role desconhecida cai em aluno")
}

func TestValidateRoleLimits(t *testing.T) {
	assert.NoError(t, validateRoleLimits("aluno", 2, 4096))
	assert.ErrorIs(t, validateRoleLimits("aluno", 3, 1024), ErrLimitExceeded)
	assert.ErrorIs(t, validateRoleLimits("aluno", 1, 8192), ErrLimitExceeded)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/usecase/ -run 'TestLimitsFor|TestValidateRoleLimits' -v`
Expected: FAIL — `undefined: RoleLimits`, `undefined: LimitsFor`.

- [ ] **Step 3: Criar `limits.go` e remover o bloco antigo do `vm_usecase.go`**

`backend/internal/usecase/limits.go`:
```go
package usecase

import "fmt"

// RoleLimits são os tetos por perfil, compartilhados por VMs e containers.
type RoleLimits struct {
	MaxCPU        int // cores (VM) ou CPU inteira (container: MaxCPU*1000 millis)
	MaxMemMB      int
	MaxContainers int
	MaxStorageGB  int // soma dos volumes persistentes
}

var roleLimits = map[string]RoleLimits{
	"aluno":       {MaxCPU: 2, MaxMemMB: 4096, MaxContainers: 5, MaxStorageGB: 20},
	"pesquisador": {MaxCPU: 8, MaxMemMB: 16384, MaxContainers: 20, MaxStorageGB: 100},
}

// LimitsFor devolve os limites do perfil; role desconhecida cai em aluno por segurança.
func LimitsFor(role string) RoleLimits {
	if l, ok := roleLimits[role]; ok {
		return l
	}
	return roleLimits["aluno"]
}

func validateRoleLimits(role string, cpuCores, memoryMB int) error {
	limits := LimitsFor(role)
	if cpuCores > limits.MaxCPU {
		return fmt.Errorf("%w: perfil '%s' permite no máximo %d CPU(s)", ErrLimitExceeded, role, limits.MaxCPU)
	}
	if memoryMB > limits.MaxMemMB {
		return fmt.Errorf("%w: perfil '%s' permite no máximo %dMB de RAM", ErrLimitExceeded, role, limits.MaxMemMB)
	}
	return nil
}
```

Em `vm_usecase.go`: apagar o `var roleLimits = ...` (linhas do comentário "limites máximos por role" até o `}`) e a função `validateRoleLimits` inteira. `ErrLimitExceeded` continua onde está.

- [ ] **Step 4: Adicionar envs no config**

Em `config.go`, struct `Config`, após o bloco Terraform:
```go
	// k3s / containers
	K3SNodeIP       string // IP fixo da VM do cluster em CIDR, ex: "10.0.0.50/24"
	K3SNodeGateway  string
	K3SNodeCPU      int
	K3SNodeMemoryMB int
	K3SNodeDiskGB   int
	IngressDomain   string // vazio → containers HTTP expostos por NodePort
	KubeconfigPath  string // default: <TerraformWorkdir>/k3s/kubeconfig
```
Em `Load()`, antes de `cfg := &Config{`:
```go
	k3sCPU, err := strconv.Atoi(getEnv("K3S_NODE_CPU", "8"))
	if err != nil {
		return nil, fmt.Errorf("K3S_NODE_CPU inválido: %w", err)
	}
	k3sMem, err := strconv.Atoi(getEnv("K3S_NODE_MEMORY_MB", "16384"))
	if err != nil {
		return nil, fmt.Errorf("K3S_NODE_MEMORY_MB inválido: %w", err)
	}
	k3sDisk, err := strconv.Atoi(getEnv("K3S_NODE_DISK_GB", "200"))
	if err != nil {
		return nil, fmt.Errorf("K3S_NODE_DISK_GB inválido: %w", err)
	}
	tfWorkdir := getEnv("TERRAFORM_WORKDIR", "/var/lib/proxlab/terraform")
```
No literal: `TerraformWorkdir: tfWorkdir,` e
```go
		K3SNodeIP:       getEnv("K3S_NODE_IP", ""),
		K3SNodeGateway:  getEnv("K3S_NODE_GATEWAY", ""),
		K3SNodeCPU:      k3sCPU,
		K3SNodeMemoryMB: k3sMem,
		K3SNodeDiskGB:   k3sDisk,
		IngressDomain:   getEnv("INGRESS_DOMAIN", ""),
		KubeconfigPath:  getEnv("KUBECONFIG_PATH", filepath.Join(tfWorkdir, "k3s", "kubeconfig")),
```
Importar `path/filepath`. `K3S_NODE_IP` e gateway NÃO entram em `validate()` (só o bootstrap exige).

- [ ] **Step 5: `.env.example`** — acrescentar seção:
```
# ── k3s / Containers ──────────────────────────────────────────────────────────
# IP fixo reservado para a VM do cluster (CIDR) e gateway — usados só pelo k3s-bootstrap
K3S_NODE_IP=
K3S_NODE_GATEWAY=
K3S_NODE_CPU=8
K3S_NODE_MEMORY_MB=16384
K3S_NODE_DISK_GB=200
# Domínio wildcard para Ingress (ex: lab.cces.belojardim.ifpe.edu.br). Vazio → NodePort
INGRESS_DOMAIN=
# Kubeconfig gerado pelo bootstrap. Default: $TERRAFORM_WORKDIR/k3s/kubeconfig
KUBECONFIG_PATH=
```

- [ ] **Step 6: Rodar tudo**

Run: `cd backend && go build ./... && go test ./...`
Expected: PASS (testes de VM continuam passando com `validateRoleLimits` movido).

- [ ] **Step 7: Commit**
```bash
git add backend/internal/config/config.go backend/internal/usecase/limits.go backend/internal/usecase/limits_test.go backend/internal/usecase/vm_usecase.go
git commit -m "feat(config): envs do cluster k3s e limites por perfil extraídos para limits.go"
```

---

### Task 2: Terraform — chave SSH e IP fixo no cloud-init

**Files:**
- Modify: `backend/internal/domain/repository/provisioner.go`
- Modify: `backend/internal/terraform/template.go`
- Modify: `backend/internal/terraform/main.tf.tmpl`
- Modify: `backend/internal/terraform/template_test.go`

**Interfaces:**
- Produces: `repository.VMSpec` campos novos `CloudInitSSHKeys []string`, `StaticIP string` (CIDR), `Gateway string`. Vazios → comportamento atual (senha + DHCP).

- [ ] **Step 1: Testes**

Acrescentar em `template_test.go`:
```go
func TestRender_CloneWithSSHKeyAndStaticIP(t *testing.T) {
	spec := baseSpec()
	spec.CloneVMID = 9000
	spec.CloudInitUser = "ubuntu"
	spec.CloudInitSSHKeys = []string{"ssh-ed25519 AAAAC3 k3s"}
	spec.StaticIP = "10.0.0.50/24"
	spec.Gateway = "10.0.0.1"
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	parseHCL(t, out)

	s := string(out)
	assert.Contains(t, s, `keys     = ["ssh-ed25519 AAAAC3 k3s"]`)
	assert.Contains(t, s, `address = "10.0.0.50/24"`)
	assert.Contains(t, s, `gateway = "10.0.0.1"`)
	assert.NotContains(t, s, `address = "dhcp"`)
	assert.NotContains(t, s, "password")
}

func TestRender_CloneDefaultsToDHCPAndPassword(t *testing.T) {
	spec := baseSpec()
	spec.CloneVMID = 9000
	spec.CloudInitUser = "ubuntu"
	spec.CloudInitPassword = "s3cret"
	out, err := terraform.Render(spec, "pve01", "")
	require.NoError(t, err)
	s := string(out)
	assert.Contains(t, s, `address = "dhcp"`)
	assert.Contains(t, s, `password = "s3cret"`)
	assert.NotContains(t, s, "keys")
	assert.NotContains(t, s, "gateway")
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/terraform/ -run TestRender_Clone -v`
Expected: FAIL — campos desconhecidos em `VMSpec`.

- [ ] **Step 3: VMSpec** — em `provisioner.go`, após `CloudInitPassword string`:
```go
	CloudInitSSHKeys  []string // chaves públicas autorizadas para CloudInitUser
	StaticIP          string   // CIDR, ex: "10.0.0.50/24". Vazio → DHCP
	Gateway           string   // obrigatório quando StaticIP != ""
```

- [ ] **Step 4: templateData e Render** — em `template.go`, adicionar a `templateData`:
```go
	CloudInitSSHKeys  []string
	StaticIP          string
	Gateway           string
```
e em `Render`, no literal: `CloudInitSSHKeys: spec.CloudInitSSHKeys, StaticIP: spec.StaticIP, Gateway: spec.Gateway,`.

- [ ] **Step 5: Template** — substituir o bloco `initialization` em `main.tf.tmpl` por:
```
  initialization {
    datastore_id = "{{ hcl .Storage }}"

    user_account {
      username = "{{ hcl .CloudInitUser }}"
{{- if .CloudInitPassword }}
      password = "{{ hcl .CloudInitPassword }}"
{{- end }}
{{- if .CloudInitSSHKeys }}
      keys     = [{{ range $i, $k := .CloudInitSSHKeys }}{{ if $i }}, {{ end }}"{{ hcl $k }}"{{ end }}]
{{- end }}
    }

    ip_config {
      ipv4 {
{{- if .StaticIP }}
        address = "{{ hcl .StaticIP }}"
        gateway = "{{ hcl .Gateway }}"
{{- else }}
        address = "dhcp"
{{- end }}
      }
    }
  }
```

- [ ] **Step 6: Rodar**

Run: `cd backend && go test ./internal/terraform/ -v`
Expected: PASS (inclusive testes antigos).

- [ ] **Step 7: Commit**
```bash
git add backend/internal/domain/repository/provisioner.go backend/internal/terraform/
git commit -m "feat(terraform): cloud-init com chaves SSH e IP fixo opcionais no VMSpec"
```

---

### Task 3: Entidade Container, catálogo, repositório GORM e migration

**Files:**
- Create: `backend/internal/domain/entity/container.go`
- Create: `backend/internal/domain/entity/container_test.go`
- Create: `backend/internal/domain/entity/catalog.go`
- Create: `backend/internal/domain/entity/catalog_test.go`
- Create: `backend/internal/domain/repository/container_repository.go`
- Create: `backend/internal/repository/container_repository.go`
- Create: `backend/migrations/003_create_containers.sql`
- Modify: `backend/internal/config/config.go` (AutoMigrate)

**Interfaces:**
- Produces: `entity.Container`, `entity.EnvMap`, `entity.NamespaceFor(userID uuid.UUID) string`, `entity.ContainerNameRegexp`, `entity.Catalog() []CatalogImage`, `entity.CatalogByKey(key string) (CatalogImage, bool)`.
- Produces: `repository.ContainerRepository` com `Create, FindByUserID, FindByID, FindByUserIDAndName, CountByUserID, SumPersistentGBByUserID, Delete`.

- [ ] **Step 1: Testes de entidade e catálogo**

`container_test.go`:
```go
package entity_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespaceFor(t *testing.T) {
	id := uuid.MustParse("0f8fad5b-d9cb-469f-a165-70867728950e")
	assert.Equal(t, "u-0f8fad5b", entity.NamespaceFor(id))
}

func TestContainerNameRegexp(t *testing.T) {
	for _, ok := range []string{"a", "meu-pg", "app1", "x-y-z"} {
		assert.True(t, entity.ContainerNameRegexp.MatchString(ok), ok)
	}
	for _, bad := range []string{"", "-a", "a-", "Meu", "a_b", "a b", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		assert.False(t, entity.ContainerNameRegexp.MatchString(bad), bad)
	}
}

func TestEnvMapScanValue(t *testing.T) {
	m := entity.EnvMap{"A": "1"}
	v, err := m.Value()
	require.NoError(t, err)
	var back entity.EnvMap
	require.NoError(t, back.Scan(v))
	assert.Equal(t, m, back)
	require.NoError(t, back.Scan(nil))
	assert.Empty(t, back)
}
```
`catalog_test.go`:
```go
package entity_test

import (
	"testing"

	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestCatalog(t *testing.T) {
	pg, ok := entity.CatalogByKey("postgres")
	assert.True(t, ok)
	assert.Equal(t, "postgres:16", pg.Image)
	assert.Equal(t, 5432, pg.Port)
	assert.Equal(t, "tcp", pg.Protocol)
	assert.Equal(t, "/var/lib/postgresql/data", pg.MountPath)
	assert.Equal(t, []string{"POSTGRES_PASSWORD"}, pg.RequiredEnv)

	ub, ok := entity.CatalogByKey("ubuntu")
	assert.True(t, ok)
	assert.Equal(t, 0, ub.Port)
	assert.Equal(t, "sleep infinity", ub.Command)

	_, ok = entity.CatalogByKey("nao-existe")
	assert.False(t, ok)
	assert.Len(t, entity.Catalog(), 6)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/domain/entity/ -v`
Expected: FAIL — pacote sem os símbolos.

- [ ] **Step 3: Entidade**

`container.go`:
```go
package entity

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// ContainerNameRegexp: minúsculo, alfanumérico e hífen, 1–40 chars, sem hífen nas pontas.
var ContainerNameRegexp = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

const (
	ProtocolHTTP = "http"
	ProtocolTCP  = "tcp"
)

// EnvMap é persistido como jsonb.
type EnvMap map[string]string

func (m EnvMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func (m *EnvMap) Scan(src any) error {
	if src == nil {
		*m = EnvMap{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("EnvMap: tipo inesperado no Scan")
	}
	return json.Unmarshal(b, m)
}

// Container é a definição persistida. Status NÃO é coluna: vem do cluster.
type Container struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"                          json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:ux_containers_user_name" json:"user_id"`
	Name         string    `gorm:"size:40;not null;uniqueIndex:ux_containers_user_name"        json:"name"`
	Description  string    `gorm:"default:''"                                    json:"description"`
	Image        string    `gorm:"not null"                                      json:"image"`
	CPUMillis    int       `gorm:"column:cpu_millis;not null"                    json:"cpu_millis"`
	MemoryMB     int       `gorm:"column:memory_mb;not null"                     json:"memory_mb"`
	Port         int       `gorm:"not null;default:0"                            json:"port"`
	Protocol     string    `gorm:"size:4;not null;default:'http'"                json:"protocol"`
	NodePort     int       `gorm:"column:node_port;not null;default:0"           json:"node_port"`
	Env          EnvMap    `gorm:"type:jsonb;not null;default:'{}'"              json:"env"`
	Command      string    `gorm:"default:''"                                    json:"command"`
	PersistentGB int       `gorm:"column:persistent_gb;not null;default:0"       json:"persistent_gb"`
	MountPath    string    `gorm:"column:mount_path;default:''"                  json:"mount_path"`
	Namespace    string    `gorm:"size:10;not null"                              json:"namespace"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NamespaceFor deriva o namespace k8s do usuário: "u-" + 8 primeiros chars do UUID.
func NamespaceFor(userID uuid.UUID) string {
	return "u-" + userID.String()[:8]
}
```

`catalog.go`:
```go
package entity

// CatalogImage é uma imagem curada. Port 0 = não expor (só terminal).
type CatalogImage struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Image       string   `json:"image"`
	Port        int      `json:"port"`
	Protocol    string   `json:"protocol"`
	MountPath   string   `json:"mount_path"`
	Command     string   `json:"command"`
	RequiredEnv []string `json:"required_env"`
}

var catalog = []CatalogImage{
	{Key: "nginx", Label: "Nginx", Image: "nginx:1.27", Port: 80, Protocol: ProtocolHTTP, MountPath: "/usr/share/nginx/html"},
	{Key: "python", Label: "Python 3.12", Image: "python:3.12", Port: 8000, Protocol: ProtocolHTTP, MountPath: "/data"},
	{Key: "node", Label: "Node.js 22", Image: "node:22", Port: 3000, Protocol: ProtocolHTTP, MountPath: "/data"},
	{Key: "postgres", Label: "PostgreSQL 16", Image: "postgres:16", Port: 5432, Protocol: ProtocolTCP, MountPath: "/var/lib/postgresql/data", RequiredEnv: []string{"POSTGRES_PASSWORD"}},
	{Key: "jupyter", Label: "Jupyter Notebook", Image: "jupyter/minimal-notebook:latest", Port: 8888, Protocol: ProtocolHTTP, MountPath: "/home/jovyan"},
	{Key: "ubuntu", Label: "Ubuntu 24.04 (só terminal)", Image: "ubuntu:24.04", Port: 0, MountPath: "/data", Command: "sleep infinity"},
}

func Catalog() []CatalogImage { return catalog }

func CatalogByKey(key string) (CatalogImage, bool) {
	for _, c := range catalog {
		if c.Key == key {
			return c, true
		}
	}
	return CatalogImage{}, false
}
```

- [ ] **Step 4: Interface e GORM**

`internal/domain/repository/container_repository.go`:
```go
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
)

type ContainerRepository interface {
	Create(ctx context.Context, c *entity.Container) error
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Container, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Container, error)
	FindByUserIDAndName(ctx context.Context, userID uuid.UUID, name string) (*entity.Container, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	SumPersistentGBByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
```

`internal/repository/container_repository.go`:
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

type containerRepository struct{ db *gorm.DB }

func NewContainerRepository(db *gorm.DB) domainrepo.ContainerRepository {
	return &containerRepository{db: db}
}

func (r *containerRepository) Create(ctx context.Context, c *entity.Container) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *containerRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Container, error) {
	var out []entity.Container
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *containerRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Container, error) {
	var c entity.Container
	err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

func (r *containerRepository) FindByUserIDAndName(ctx context.Context, userID uuid.UUID, name string) (*entity.Container, error) {
	var c entity.Container
	err := r.db.WithContext(ctx).First(&c, "user_id = ? AND name = ?", userID, name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

func (r *containerRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.Container{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *containerRepository) SumPersistentGBByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var sum int
	err := r.db.WithContext(ctx).Model(&entity.Container{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(persistent_gb), 0)").Scan(&sum).Error
	return sum, err
}

func (r *containerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&entity.Container{}, "id = ?", id).Error
}
```

- [ ] **Step 5: Migration e AutoMigrate**

`migrations/003_create_containers.sql`:
```sql
CREATE TABLE IF NOT EXISTS containers (
    id            UUID         PRIMARY KEY,
    user_id       UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          VARCHAR(40)  NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    image         TEXT         NOT NULL,
    cpu_millis    INTEGER      NOT NULL,
    memory_mb     INTEGER      NOT NULL,
    port          INTEGER      NOT NULL DEFAULT 0,
    protocol      VARCHAR(4)   NOT NULL DEFAULT 'http',
    node_port     INTEGER      NOT NULL DEFAULT 0,
    env           JSONB        NOT NULL DEFAULT '{}',
    command       TEXT         NOT NULL DEFAULT '',
    persistent_gb INTEGER      NOT NULL DEFAULT 0,
    mount_path    TEXT         NOT NULL DEFAULT '',
    namespace     VARCHAR(10)  NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_containers_user_id ON containers(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_containers_user_name ON containers(user_id, name);
-- alocação atômica de NodePort: só uma linha por porta > 0
CREATE UNIQUE INDEX IF NOT EXISTS ux_containers_node_port ON containers(node_port) WHERE node_port > 0;
```

Em `config.go` `ConnectDatabase`, trocar `db.AutoMigrate(&entity.User{}, &entity.VM{})` por `db.AutoMigrate(&entity.User{}, &entity.VM{}, &entity.Container{})` e, logo depois (ainda dentro do `if development`):
```go
		if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_containers_node_port ON containers(node_port) WHERE node_port > 0`).Error; err != nil {
			return nil, fmt.Errorf("erro ao criar índice de node_port: %w", err)
		}
```
(GORM não gera índice parcial via tag.)

- [ ] **Step 6: Rodar**

Run: `cd backend && go build ./... && go test ./internal/domain/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**
```bash
git add backend/internal/domain/entity/container.go backend/internal/domain/entity/container_test.go backend/internal/domain/entity/catalog.go backend/internal/domain/entity/catalog_test.go backend/internal/domain/repository/container_repository.go backend/internal/repository/container_repository.go backend/migrations/003_create_containers.sql backend/internal/config/config.go
git commit -m "feat(domain): entidade Container, catálogo de imagens e repositório GORM"
```

---

### Task 4: Porta `ContainerRuntime` e dependência client-go

**Files:**
- Create: `backend/internal/domain/repository/container_runtime.go`
- Modify: `backend/go.mod` (via `go get`)

**Interfaces:**
- Produces (usadas por Tasks 5–10):
```go
type ContainerStatus struct{ Phase, Message string } // Phase: pending|running|failed|unknown
type ResourceQuotaSpec struct{ CPUMillis, MemoryMB, Pods, StorageGB int }
type TerminalSession interface{ Stdin() io.Writer; Stdout() io.Reader; Resize(cols, rows uint16) error; Close() error }
type ContainerRuntime interface{ EnsureNamespace; Apply; Delete; Status; Exec; Logs }  // assinaturas abaixo
var ErrRuntimeUnavailable = errors.New("cluster de containers indisponível")
var ErrNoShell = errors.New("esta imagem não possui shell")
```

- [ ] **Step 1: Escrever a porta**

`container_runtime.go`:
```go
package repository

import (
	"context"
	"errors"
	"io"

	"github.com/proxlab/backend/internal/domain/entity"
)

var (
	// ErrRuntimeUnavailable: kubeconfig ausente ou cluster inalcançável.
	ErrRuntimeUnavailable = errors.New("cluster de containers indisponível")
	// ErrNoShell: exec falhou porque a imagem não tem /bin/sh.
	ErrNoShell = errors.New("esta imagem não possui shell")
)

const (
	PhasePending = "pending"
	PhaseRunning = "running"
	PhaseFailed  = "failed"
	PhaseUnknown = "unknown"
)

type ContainerStatus struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

// ResourceQuotaSpec alimenta o ResourceQuota do namespace do usuário.
type ResourceQuotaSpec struct {
	CPUMillis int
	MemoryMB  int
	Pods      int
	StorageGB int
}

type TerminalSession interface {
	Stdin() io.Writer
	Stdout() io.Reader
	Resize(cols, rows uint16) error
	Close() error
}

// ContainerRuntime é a porta para o cluster. Implementada por internal/kube.Runtime.
type ContainerRuntime interface {
	EnsureNamespace(ctx context.Context, ns string, q ResourceQuotaSpec) error
	// Apply cria/atualiza Deployment, Secret, Service, Ingress e PVC. Idempotente.
	Apply(ctx context.Context, c *entity.Container, ingressDomain string) error
	Delete(ctx context.Context, ns, name string) error
	Status(ctx context.Context, ns string, names []string) (map[string]ContainerStatus, error)
	Exec(ctx context.Context, ns, name string, cols, rows uint16) (TerminalSession, error)
	Logs(ctx context.Context, ns, name string, follow bool) (io.ReadCloser, error)
}
```
(Desvio pequeno da spec: `Apply` recebe `ingressDomain` para o pacote `kube` não depender de config.)

- [ ] **Step 2: Dependências**

Run:
```bash
cd backend && go get k8s.io/client-go@v0.31.4 k8s.io/api@v0.31.4 k8s.io/apimachinery@v0.31.4 && go mod tidy && go build ./...
```
Expected: build OK. Se o `go get` de `v0.31.4` falhar por incompatibilidade com Go 1.25, usar a última `v0.3x` que resolva.

- [ ] **Step 3: Commit**
```bash
git add backend/go.mod backend/go.sum backend/internal/domain/repository/container_runtime.go
git commit -m "feat(domain): porta ContainerRuntime e dependência client-go"
```

---

### Task 5: `internal/kube` — cliente e EnsureNamespace

**Files:**
- Create: `backend/internal/kube/client.go`
- Create: `backend/internal/kube/namespace.go`
- Create: `backend/internal/kube/namespace_test.go`

**Interfaces:**
- Produces: `kube.New(kubeconfigPath string) (*Runtime, error)`, `kube.NewWithClientset(cs kubernetes.Interface) *Runtime` (para testes), `(*Runtime).EnsureNamespace`.

- [ ] **Step 1: Teste com fake clientset**

`namespace_test.go`:
```go
package kube_test

import (
	"context"
	"testing"

	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEnsureNamespace_CreatesQuotaLimitRangeAndPolicy(t *testing.T) {
	cs := fake.NewSimpleClientset()
	rt := kube.NewWithClientset(cs)
	ctx := context.Background()
	q := repository.ResourceQuotaSpec{CPUMillis: 2000, MemoryMB: 4096, Pods: 5, StorageGB: 20}

	require.NoError(t, rt.EnsureNamespace(ctx, "u-abc12345", q))
	require.NoError(t, rt.EnsureNamespace(ctx, "u-abc12345", q), "segunda chamada é idempotente")

	ns, err := cs.CoreV1().Namespaces().Get(ctx, "u-abc12345", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "restricted", ns.Labels["pod-security.kubernetes.io/enforce"])
	assert.Equal(t, "proxlab", ns.Labels["app.kubernetes.io/managed-by"])

	rq, err := cs.CoreV1().ResourceQuotas("u-abc12345").Get(ctx, "proxlab-quota", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "2", rq.Spec.Hard.Cpu().String())
	assert.Equal(t, "4Gi", rq.Spec.Hard.Memory().String())
	assert.Equal(t, "5", rq.Spec.Hard.Pods().String())
	assert.Equal(t, "20Gi", rq.Spec.Hard.Name("requests.storage", "").String())

	_, err = cs.CoreV1().LimitRanges("u-abc12345").Get(ctx, "proxlab-defaults", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = cs.NetworkingV1().NetworkPolicies("u-abc12345").Get(ctx, "proxlab-isolation", metav1.GetOptions{})
	require.NoError(t, err)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/kube/ -v`
Expected: FAIL — pacote não existe.

- [ ] **Step 3: client.go**
```go
package kube

import (
	"errors"
	"fmt"
	"os"

	"github.com/proxlab/backend/internal/domain/repository"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const fieldManager = "proxlab"

// Runtime implementa repository.ContainerRuntime com client-go.
type Runtime struct {
	cs         kubernetes.Interface
	restConfig *rest.Config // nil nos testes; exigido por Exec
}

var _ repository.ContainerRuntime = (*Runtime)(nil)

// New carrega o kubeconfig. Arquivo ausente → repository.ErrRuntimeUnavailable
// (o servidor sobe mesmo assim; rotas de containers respondem 503).
func New(kubeconfigPath string) (*Runtime, error) {
	if _, err := os.Stat(kubeconfigPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, repository.ErrRuntimeUnavailable
		}
		return nil, fmt.Errorf("erro ao acessar kubeconfig: %w", err)
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig inválido: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente kubernetes: %w", err)
	}
	return &Runtime{cs: cs, restConfig: cfg}, nil
}

// NewWithClientset é usado em testes com fake.NewSimpleClientset.
func NewWithClientset(cs kubernetes.Interface) *Runtime {
	return &Runtime{cs: cs}
}
```

- [ ] **Step 4: namespace.go**
```go
package kube

import (
	"context"
	"fmt"

	"github.com/proxlab/backend/internal/domain/repository"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	applynetv1 "k8s.io/client-go/applyconfigurations/networking/v1"
)

var applyOpts = metav1.ApplyOptions{FieldManager: fieldManager, Force: true}

func (r *Runtime) EnsureNamespace(ctx context.Context, ns string, q repository.ResourceQuotaSpec) error {
	nsCfg := applycorev1.Namespace(ns).WithLabels(map[string]string{
		"app.kubernetes.io/managed-by":         "proxlab",
		"pod-security.kubernetes.io/enforce":   "restricted",
		"pod-security.kubernetes.io/enforce-version": "latest",
	})
	if _, err := r.cs.CoreV1().Namespaces().Apply(ctx, nsCfg, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar namespace: %w", err)
	}

	hard := corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewMilliQuantity(int64(q.CPUMillis), resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(int64(q.MemoryMB)<<20, resource.BinarySI),
		corev1.ResourcePods:             *resource.NewQuantity(int64(q.Pods), resource.DecimalSI),
		corev1.ResourceRequestsStorage:  *resource.NewQuantity(int64(q.StorageGB)<<30, resource.BinarySI),
	}
	rq := applycorev1.ResourceQuota("proxlab-quota", ns).
		WithSpec(applycorev1.ResourceQuotaSpec().WithHard(hard))
	if _, err := r.cs.CoreV1().ResourceQuotas(ns).Apply(ctx, rq, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar resourcequota: %w", err)
	}

	def := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("250m"),
		corev1.ResourceMemory: resource.MustParse("256Mi"),
	}
	lr := applycorev1.LimitRange("proxlab-defaults", ns).
		WithSpec(applycorev1.LimitRangeSpec().WithLimits(
			applycorev1.LimitRangeItem().WithType(corev1.LimitTypeContainer).
				WithDefault(def).WithDefaultRequest(def)))
	if _, err := r.cs.CoreV1().LimitRanges(ns).Apply(ctx, lr, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar limitrange: %w", err)
	}

	// Ingress: só do próprio namespace e do Traefik (kube-system). Egress: liberado.
	np := applynetv1.NetworkPolicy("proxlab-isolation", ns).
		WithSpec(applynetv1.NetworkPolicySpec().
			WithPodSelector(applymetav1.LabelSelector()).
			WithPolicyTypes(netv1.PolicyTypeIngress).
			WithIngress(
				applynetv1.NetworkPolicyIngressRule().WithFrom(
					applynetv1.NetworkPolicyPeer().WithPodSelector(applymetav1.LabelSelector()),
					applynetv1.NetworkPolicyPeer().WithNamespaceSelector(
						applymetav1.LabelSelector().WithMatchLabels(map[string]string{"kubernetes.io/metadata.name": "kube-system"})),
				),
			))
	if _, err := r.cs.NetworkingV1().NetworkPolicies(ns).Apply(ctx, np, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar networkpolicy: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Rodar**

Run: `cd backend && go test ./internal/kube/ -v`
Expected: PASS. Nota: o fake clientset suporta `Apply` a partir de client-go 0.31 (usa `managedfields` no tracker). Se `Apply` retornar "not supported", trocar por padrão Get → Create/Update em uma função auxiliar `upsert` e manter a mesma asserção.

- [ ] **Step 6: Commit**
```bash
git add backend/internal/kube/
git commit -m "feat(kube): cliente client-go e EnsureNamespace com quota, limitrange e networkpolicy"
```

---

### Task 6: `internal/kube` — Apply, Delete e Status

**Files:**
- Create: `backend/internal/kube/workload.go`
- Create: `backend/internal/kube/status.go`
- Create: `backend/internal/kube/workload_test.go`

**Interfaces:**
- Consumes: `entity.Container`, `repository.ContainerStatus`, fases da Task 4.
- Produces: `(*Runtime).Apply`, `(*Runtime).Delete`, `(*Runtime).Status`. Convenções de nome no cluster: Deployment/Service/Ingress/PVC = `c.Name`; Secret = `c.Name + "-env"`; label `app=<name>`.

- [ ] **Step 1: Testes**

`workload_test.go`:
```go
package kube_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func sample() *entity.Container {
	return &entity.Container{
		ID: uuid.New(), Name: "meu-pg", Image: "postgres:16",
		CPUMillis: 500, MemoryMB: 512, Port: 5432, Protocol: "tcp", NodePort: 31234,
		Env: entity.EnvMap{"POSTGRES_PASSWORD": "x"}, PersistentGB: 5,
		MountPath: "/var/lib/postgresql/data", Namespace: "u-abc12345",
	}
}

func TestApply_TCPWithVolume(t *testing.T) {
	cs := fake.NewSimpleClientset()
	rt := kube.NewWithClientset(cs)
	ctx := context.Background()
	c := sample()

	require.NoError(t, rt.Apply(ctx, c, ""))
	require.NoError(t, rt.Apply(ctx, c, ""), "idempotente")

	dep, err := cs.AppsV1().Deployments(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	require.NoError(t, err)
	ctr := dep.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "postgres:16", ctr.Image)
	assert.Equal(t, "500m", ctr.Resources.Limits.Cpu().String())
	assert.Equal(t, "512Mi", ctr.Resources.Limits.Memory().String())
	assert.Equal(t, "meu-pg-env", ctr.EnvFrom[0].SecretRef.Name)
	assert.Equal(t, "/var/lib/postgresql/data", ctr.VolumeMounts[0].MountPath)

	svc, err := cs.CoreV1().Services(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeNodePort, svc.Spec.Type)
	assert.Equal(t, int32(31234), svc.Spec.Ports[0].NodePort)

	_, err = cs.CoreV1().PersistentVolumeClaims(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = cs.NetworkingV1().Ingresses(c.Namespace).Get(ctx, "meu-pg", metav1.GetOptions{})
	assert.Error(t, err, "tcp não cria ingress")
}

func TestApply_HTTPWithIngress(t *testing.T) {
	cs := fake.NewSimpleClientset()
	rt := kube.NewWithClientset(cs)
	c := sample()
	c.Name, c.Protocol, c.Port, c.NodePort, c.PersistentGB = "site", "http", 80, 0, 0

	require.NoError(t, rt.Apply(context.Background(), c, "lab.example.edu"))
	ing, err := cs.NetworkingV1().Ingresses(c.Namespace).Get(context.Background(), "site", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "site-u-abc12345.lab.example.edu", ing.Spec.Rules[0].Host)
	svc, _ := cs.CoreV1().Services(c.Namespace).Get(context.Background(), "site", metav1.GetOptions{})
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)
	_, err = cs.CoreV1().PersistentVolumeClaims(c.Namespace).Get(context.Background(), "site", metav1.GetOptions{})
	assert.Error(t, err, "sem volume")
}

func TestApply_NoPortNoService(t *testing.T) {
	cs := fake.NewSimpleClientset()
	rt := kube.NewWithClientset(cs)
	c := sample()
	c.Name, c.Port, c.NodePort, c.Command = "shell", 0, 0, "sleep infinity"
	require.NoError(t, rt.Apply(context.Background(), c, "lab.example.edu"))
	_, err := cs.CoreV1().Services(c.Namespace).Get(context.Background(), "shell", metav1.GetOptions{})
	assert.Error(t, err)
	dep, _ := cs.AppsV1().Deployments(c.Namespace).Get(context.Background(), "shell", metav1.GetOptions{})
	assert.Equal(t, []string{"/bin/sh", "-c", "sleep infinity"}, dep.Spec.Template.Spec.Containers[0].Command)
}

func TestDelete_RemovesAll(t *testing.T) {
	cs := fake.NewSimpleClientset()
	rt := kube.NewWithClientset(cs)
	c := sample()
	require.NoError(t, rt.Apply(context.Background(), c, ""))
	require.NoError(t, rt.Delete(context.Background(), c.Namespace, c.Name))
	require.NoError(t, rt.Delete(context.Background(), c.Namespace, c.Name), "não existe → sem erro")
	_, err := cs.AppsV1().Deployments(c.Namespace).Get(context.Background(), "meu-pg", metav1.GetOptions{})
	assert.Error(t, err)
}

func TestStatus_FromPods(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "a-1", Namespace: "ns", Labels: map[string]string{"app": "a", "app.kubernetes.io/managed-by": "proxlab"}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "b-1", Namespace: "ns", Labels: map[string]string{"app": "b", "app.kubernetes.io/managed-by": "proxlab"}},
			Status: corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "not found"}}}}}},
	)
	rt := kube.NewWithClientset(cs)
	st, err := rt.Status(context.Background(), "ns", []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, repository.ContainerStatus{Phase: "running"}, st["a"])
	assert.Equal(t, "failed", st["b"].Phase)
	assert.Contains(t, st["b"].Message, "ImagePullBackOff")
	assert.Equal(t, "pending", st["c"].Phase, "sem pod ainda → pending")
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/kube/ -run 'TestApply|TestDelete|TestStatus' -v`
Expected: FAIL — métodos indefinidos.

- [ ] **Step 3: workload.go**
```go
package kube

import (
	"context"
	"fmt"

	"github.com/proxlab/backend/internal/domain/entity"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	applynetv1 "k8s.io/client-go/applyconfigurations/networking/v1"
)

func secretName(name string) string { return name + "-env" }

func labels(name string) map[string]string {
	return map[string]string{"app": name, "app.kubernetes.io/managed-by": "proxlab"}
}

// IngressHost é o host público de um container HTTP: <name>-<ns>.<domain>.
func IngressHost(name, ns, domain string) string { return name + "-" + ns + "." + domain }

func (r *Runtime) Apply(ctx context.Context, c *entity.Container, ingressDomain string) error {
	ns, name := c.Namespace, c.Name

	sec := applycorev1.Secret(secretName(name), ns).WithLabels(labels(name)).WithStringData(c.Env)
	if _, err := r.cs.CoreV1().Secrets(ns).Apply(ctx, sec, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar secret: %w", err)
	}

	if c.PersistentGB > 0 {
		pvc := applycorev1.PersistentVolumeClaim(name, ns).WithLabels(labels(name)).
			WithSpec(applycorev1.PersistentVolumeClaimSpec().
				WithAccessModes(corev1.ReadWriteOnce).
				WithResources(applycorev1.VolumeResourceRequirements().WithRequests(corev1.ResourceList{
					corev1.ResourceStorage: *resource.NewQuantity(int64(c.PersistentGB)<<30, resource.BinarySI),
				})))
		if _, err := r.cs.CoreV1().PersistentVolumeClaims(ns).Apply(ctx, pvc, applyOpts); err != nil {
			return fmt.Errorf("erro ao aplicar pvc: %w", err)
		}
	}

	res := corev1.ResourceList{
		corev1.ResourceCPU:    *resource.NewMilliQuantity(int64(c.CPUMillis), resource.DecimalSI),
		corev1.ResourceMemory: *resource.NewQuantity(int64(c.MemoryMB)<<20, resource.BinarySI),
	}
	ctr := applycorev1.Container().WithName(name).WithImage(c.Image).
		WithEnvFrom(applycorev1.EnvFromSource().WithSecretRef(applycorev1.SecretEnvSource().WithName(secretName(name)))).
		WithResources(applycorev1.ResourceRequirements().WithLimits(res).WithRequests(res)).
		WithSecurityContext(applycorev1.SecurityContext().
			WithAllowPrivilegeEscalation(false).
			WithCapabilities(applycorev1.Capabilities().WithDrop("ALL")).
			WithSeccompProfile(applycorev1.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault)))
	if c.Port > 0 {
		ctr = ctr.WithPorts(applycorev1.ContainerPort().WithContainerPort(int32(c.Port)))
	}
	if c.Command != "" {
		ctr = ctr.WithCommand("/bin/sh", "-c", c.Command)
	}
	podSpec := applycorev1.PodSpec().WithContainers(ctr).
		WithSecurityContext(applycorev1.PodSecurityContext().WithRunAsNonRoot(false)).
		WithAutomountServiceAccountToken(false)
	if c.PersistentGB > 0 {
		ctr.WithVolumeMounts(applycorev1.VolumeMount().WithName("data").WithMountPath(c.MountPath))
		podSpec = podSpec.WithVolumes(applycorev1.Volume().WithName("data").
			WithPersistentVolumeClaim(applycorev1.PersistentVolumeClaimVolumeSource().WithClaimName(name)))
	}
	dep := applyappsv1.Deployment(name, ns).WithLabels(labels(name)).
		WithSpec(applyappsv1.DeploymentSpec().WithReplicas(1).
			WithSelector(applymetav1.LabelSelector().WithMatchLabels(map[string]string{"app": name})).
			WithStrategy(applyappsv1.DeploymentStrategy().WithType("Recreate")).
			WithTemplate(applycorev1.PodTemplateSpec().WithLabels(labels(name)).WithSpec(podSpec)))
	if _, err := r.cs.AppsV1().Deployments(ns).Apply(ctx, dep, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar deployment: %w", err)
	}

	if c.Port == 0 {
		return nil
	}

	useIngress := c.Protocol == entity.ProtocolHTTP && ingressDomain != "" && c.NodePort == 0
	port := applycorev1.ServicePort().WithName("main").WithPort(int32(c.Port)).
		WithTargetPort(intstr.FromInt32(int32(c.Port))).WithProtocol(corev1.ProtocolTCP)
	svcType := corev1.ServiceTypeClusterIP
	if !useIngress {
		svcType = corev1.ServiceTypeNodePort
		port = port.WithNodePort(int32(c.NodePort))
	}
	svc := applycorev1.Service(name, ns).WithLabels(labels(name)).
		WithSpec(applycorev1.ServiceSpec().WithType(svcType).
			WithSelector(map[string]string{"app": name}).WithPorts(port))
	if _, err := r.cs.CoreV1().Services(ns).Apply(ctx, svc, applyOpts); err != nil {
		return fmt.Errorf("erro ao aplicar service: %w", err)
	}

	if useIngress {
		pt := netv1.PathTypePrefix
		ing := applynetv1.Ingress(name, ns).WithLabels(labels(name)).
			WithSpec(applynetv1.IngressSpec().WithRules(
				applynetv1.IngressRule().WithHost(IngressHost(name, ns, ingressDomain)).
					WithHTTP(applynetv1.HTTPIngressRuleValue().WithPaths(
						applynetv1.HTTPIngressPath().WithPath("/").WithPathType(pt).
							WithBackend(applynetv1.IngressBackend().WithService(
								applynetv1.IngressServiceBackend().WithName(name).
									WithPort(applynetv1.ServiceBackendPort().WithNumber(int32(c.Port)))))))))
		if _, err := r.cs.NetworkingV1().Ingresses(ns).Apply(ctx, ing, applyOpts); err != nil {
			return fmt.Errorf("erro ao aplicar ingress: %w", err)
		}
	}
	return nil
}

// Delete remove todos os objetos do container; ausência não é erro.
func (r *Runtime) Delete(ctx context.Context, ns, name string) error {
	del := metav1.DeleteOptions{}
	ignore := func(err error) error {
		if err == nil || apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := ignore(r.cs.NetworkingV1().Ingresses(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover ingress: %w", err)
	}
	if err := ignore(r.cs.CoreV1().Services(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover service: %w", err)
	}
	if err := ignore(r.cs.AppsV1().Deployments(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover deployment: %w", err)
	}
	if err := ignore(r.cs.CoreV1().Secrets(ns).Delete(ctx, secretName(name), del)); err != nil {
		return fmt.Errorf("erro ao remover secret: %w", err)
	}
	if err := ignore(r.cs.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, name, del)); err != nil {
		return fmt.Errorf("erro ao remover pvc: %w", err)
	}
	return nil
}
```
Nota sobre `runAsNonRoot=false`: as imagens do catálogo (postgres, nginx) iniciam como root e trocam de usuário. O PodSecurity `restricted` rejeitaria isso; por isso o namespace usa `enforce=restricted` **só se** todas as imagens do catálogo funcionarem no smoke test. Se o smoke test da Task 12 mostrar `pods "..." is forbidden`, mudar o label em `namespace.go` para `baseline` e registrar na spec.

- [ ] **Step 4: status.go**
```go
package kube

import (
	"context"
	"fmt"

	"github.com/proxlab/backend/internal/domain/repository"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status lista os pods do namespace uma vez e resume por app. Sem pod → pending.
func (r *Runtime) Status(ctx context.Context, ns string, names []string) (map[string]repository.ContainerStatus, error) {
	out := make(map[string]repository.ContainerStatus, len(names))
	for _, n := range names {
		out[n] = repository.ContainerStatus{Phase: repository.PhasePending, Message: "aguardando pod"}
	}
	pods, err := r.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/managed-by=proxlab"})
	if err != nil {
		return nil, fmt.Errorf("erro ao listar pods: %w", err)
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		app := p.Labels["app"]
		if _, wanted := out[app]; !wanted {
			continue
		}
		st := summarize(p)
		// Vários pods (rollout): running vence pending; failed vence tudo.
		if cur := out[app]; cur.Phase == repository.PhaseFailed || (cur.Phase == repository.PhaseRunning && st.Phase == repository.PhasePending) {
			continue
		}
		out[app] = st
	}
	return out, nil
}

func summarize(p *corev1.Pod) repository.ContainerStatus {
	for _, cs := range p.Status.ContainerStatuses {
		if w := cs.State.Waiting; w != nil {
			switch w.Reason {
			case "ImagePullBackOff", "ErrImagePull", "CrashLoopBackOff", "CreateContainerConfigError", "InvalidImageName":
				return repository.ContainerStatus{Phase: repository.PhaseFailed, Message: w.Reason + ": " + w.Message}
			}
		}
	}
	switch p.Status.Phase {
	case corev1.PodRunning:
		return repository.ContainerStatus{Phase: repository.PhaseRunning}
	case corev1.PodPending:
		return repository.ContainerStatus{Phase: repository.PhasePending, Message: "iniciando"}
	case corev1.PodFailed:
		return repository.ContainerStatus{Phase: repository.PhaseFailed, Message: p.Status.Reason}
	case corev1.PodSucceeded:
		return repository.ContainerStatus{Phase: repository.PhaseFailed, Message: "processo principal terminou"}
	default:
		return repository.ContainerStatus{Phase: repository.PhaseUnknown}
	}
}
```
- [ ] **Step 5: Rodar**

Run: `cd backend && go test ./internal/kube/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**
```bash
git add backend/internal/kube/
git commit -m "feat(kube): Apply/Delete de Deployment, Secret, Service, Ingress e PVC; Status a partir dos pods"
```

---

### Task 7: `internal/kube` — Exec e Logs

**Files:**
- Create: `backend/internal/kube/pod.go`
- Create: `backend/internal/kube/pod_test.go`
- Create: `backend/internal/kube/exec.go`
- Create: `backend/internal/kube/logs.go`

**Interfaces:**
- Produces: `(*Runtime).Exec`, `(*Runtime).Logs`, `findRunningPod(ctx, ns, name) (*corev1.Pod, error)`.

- [ ] **Step 1: Teste da seleção de pod**

`pod_test.go`:
```go
package kube

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func pod(name string, phase corev1.PodPhase, age time.Duration) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns", Labels: map[string]string{"app": "web"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-age))},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func TestFindRunningPod_PicksNewestRunning(t *testing.T) {
	cs := fake.NewSimpleClientset(
		pod("web-old", corev1.PodRunning, 2*time.Hour),
		pod("web-new", corev1.PodRunning, time.Minute),
		pod("web-pending", corev1.PodPending, 0),
	)
	rt := NewWithClientset(cs)
	p, err := rt.findRunningPod(context.Background(), "ns", "web")
	require.NoError(t, err)
	assert.Equal(t, "web-new", p.Name)
}

func TestFindRunningPod_NoneRunning(t *testing.T) {
	cs := fake.NewSimpleClientset(pod("web-pending", corev1.PodPending, 0))
	rt := NewWithClientset(cs)
	_, err := rt.findRunningPod(context.Background(), "ns", "web")
	assert.ErrorIs(t, err, errPodNotRunning)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/kube/ -run TestFindRunningPod -v`
Expected: FAIL.

- [ ] **Step 3: pod.go**
```go
package kube

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var errPodNotRunning = errors.New("container ainda não está em execução")

func (r *Runtime) findRunningPod(ctx context.Context, ns, name string) (*corev1.Pod, error) {
	pods, err := r.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app=" + name})
	if err != nil {
		return nil, fmt.Errorf("erro ao listar pods: %w", err)
	}
	var best *corev1.Pod
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodRunning {
			continue
		}
		if best == nil || p.CreationTimestamp.After(best.CreationTimestamp.Time) {
			best = p
		}
	}
	if best == nil {
		return nil, errPodNotRunning
	}
	return best, nil
}
```

- [ ] **Step 4: exec.go**
```go
package kube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/proxlab/backend/internal/domain/repository"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type execSession struct {
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	sizeCh  chan remotecommand.TerminalSize
	cancel  context.CancelFunc
	done    chan error
}

func (s *execSession) Stdin() io.Writer  { return s.stdinW }
func (s *execSession) Stdout() io.Reader { return s.stdoutR }
func (s *execSession) Resize(cols, rows uint16) error {
	select {
	case s.sizeCh <- remotecommand.TerminalSize{Width: cols, Height: rows}:
	default:
	}
	return nil
}
func (s *execSession) Close() error {
	s.cancel()
	_ = s.stdinW.Close()
	_ = s.stdoutR.Close()
	return nil
}

// Next implementa remotecommand.TerminalSizeQueue.
func (s *execSession) Next() *remotecommand.TerminalSize {
	sz, ok := <-s.sizeCh
	if !ok {
		return nil
	}
	return &sz
}

// Exec abre um shell interativo no pod. Sem /bin/sh → repository.ErrNoShell.
func (r *Runtime) Exec(ctx context.Context, ns, name string, cols, rows uint16) (repository.TerminalSession, error) {
	if r.restConfig == nil {
		return nil, repository.ErrRuntimeUnavailable
	}
	pod, err := r.findRunningPod(ctx, ns, name)
	if err != nil {
		return nil, err
	}
	req := r.cs.CoreV1().RESTClient().Post().Resource("pods").Namespace(ns).Name(pod.Name).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: name,
			Command:   []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"},
			Stdin:     true, Stdout: true, Stderr: false, TTY: true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(r.restConfig, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("erro ao criar executor: %w", err)
	}

	sctx, cancel := context.WithCancel(ctx)
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	s := &execSession{stdinW: stdinW, stdoutR: stdoutR, sizeCh: make(chan remotecommand.TerminalSize, 1), cancel: cancel, done: make(chan error, 1)}
	s.sizeCh <- remotecommand.TerminalSize{Width: cols, Height: rows}

	go func() {
		err := exec.StreamWithContext(sctx, remotecommand.StreamOptions{Stdin: stdinR, Stdout: stdoutW, Tty: true, TerminalSizeQueue: s})
		if err != nil && strings.Contains(err.Error(), "executable file not found") {
			err = repository.ErrNoShell
		}
		_ = stdoutW.CloseWithError(err)
		s.done <- err
	}()
	return s, nil
}

// IsNoShell ajuda o handler a traduzir o erro assíncrono do stream.
func IsNoShell(err error) bool { return errors.Is(err, repository.ErrNoShell) }
```

- [ ] **Step 5: logs.go**
```go
package kube

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
)

// Logs abre o stream de logs do pod mais recente em Running. tailLines=200.
func (r *Runtime) Logs(ctx context.Context, ns, name string, follow bool) (io.ReadCloser, error) {
	pod, err := r.findRunningPod(ctx, ns, name)
	if err != nil {
		return nil, err
	}
	tail := int64(200)
	rc, err := r.cs.CoreV1().Pods(ns).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: name, Follow: follow, TailLines: &tail,
	}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir logs: %w", err)
	}
	return rc, nil
}

// ErrPodNotRunning é exportado para o handler de logs esperar o pod subir.
var ErrPodNotRunning = errPodNotRunning
```

- [ ] **Step 6: Rodar**

Run: `cd backend && go build ./... && go test ./internal/kube/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**
```bash
git add backend/internal/kube/
git commit -m "feat(kube): exec interativo via SPDY e stream de logs do pod"
```

---

### Task 8: `ContainerUseCase`

**Files:**
- Create: `backend/internal/usecase/container_usecase.go`
- Create: `backend/internal/usecase/container_usecase_test.go`

**Interfaces:**
- Consumes: `repository.ContainerRepository`, `repository.ContainerRuntime`, `LimitsFor`, `entity.NamespaceFor`, `entity.ContainerNameRegexp`, (o usecase calcula o host do Ingress localmente com a mesma regra de `kube.IngressHost`, para não depender do pacote `kube`).
- Produces:
```go
type CreateContainerRequest struct{ Name, Description, Image string; CPUMillis, MemoryMB, Port int; Protocol string; Env map[string]string; Command string; PersistentGB int; MountPath, Role string }
type ContainerView struct{ *entity.Container; Status repository.ContainerStatus; ExternalURL, InternalAddress string }
func NewContainerUseCase(repo, runtime, ingressDomain, nodeIP string) *ContainerUseCase   // runtime pode ser nil → ErrRuntimeUnavailable
func (uc) Create(ctx, userID, req) (*ContainerView, error)
func (uc) List(ctx, userID) ([]ContainerView, error)
func (uc) Get(ctx, userID, id) (*ContainerView, error)
func (uc) Delete(ctx, userID, id) error
func (uc) FindOwned(ctx, userID, id) (*entity.Container, error)   // usado pelos handlers WebSocket
var ErrContainerNotFound, ErrContainerNameTaken, ErrInvalidContainer
```

- [ ] **Step 1: Testes**

`container_usecase_test.go`:
```go
package usecase_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockContainerRepo struct{ mock.Mock }

func (m *mockContainerRepo) Create(ctx context.Context, c *entity.Container) error {
	return m.Called(ctx, c).Error(0)
}
func (m *mockContainerRepo) FindByUserID(ctx context.Context, u uuid.UUID) ([]entity.Container, error) {
	a := m.Called(ctx, u)
	return a.Get(0).([]entity.Container), a.Error(1)
}
func (m *mockContainerRepo) FindByID(ctx context.Context, id uuid.UUID) (*entity.Container, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*entity.Container), a.Error(1)
}
func (m *mockContainerRepo) FindByUserIDAndName(ctx context.Context, u uuid.UUID, n string) (*entity.Container, error) {
	a := m.Called(ctx, u, n)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*entity.Container), a.Error(1)
}
func (m *mockContainerRepo) CountByUserID(ctx context.Context, u uuid.UUID) (int64, error) {
	a := m.Called(ctx, u)
	return int64(a.Int(0)), a.Error(1)
}
func (m *mockContainerRepo) SumPersistentGBByUserID(ctx context.Context, u uuid.UUID) (int, error) {
	a := m.Called(ctx, u)
	return a.Int(0), a.Error(1)
}
func (m *mockContainerRepo) Delete(ctx context.Context, id uuid.UUID) error { return m.Called(ctx, id).Error(0) }

type mockRuntime struct{ mock.Mock }

func (m *mockRuntime) EnsureNamespace(ctx context.Context, ns string, q repository.ResourceQuotaSpec) error {
	return m.Called(ctx, ns, q).Error(0)
}
func (m *mockRuntime) Apply(ctx context.Context, c *entity.Container, d string) error {
	return m.Called(ctx, c, d).Error(0)
}
func (m *mockRuntime) Delete(ctx context.Context, ns, n string) error { return m.Called(ctx, ns, n).Error(0) }
func (m *mockRuntime) Status(ctx context.Context, ns string, names []string) (map[string]repository.ContainerStatus, error) {
	a := m.Called(ctx, ns, names)
	return a.Get(0).(map[string]repository.ContainerStatus), a.Error(1)
}
func (m *mockRuntime) Exec(ctx context.Context, ns, n string, c, r uint16) (repository.TerminalSession, error) {
	return nil, errors.New("não usado")
}
func (m *mockRuntime) Logs(ctx context.Context, ns, n string, f bool) (io.ReadCloser, error) {
	return nil, errors.New("não usado")
}

func baseReq() usecase.CreateContainerRequest {
	return usecase.CreateContainerRequest{
		Name: "meu-pg", Image: "postgres:16", CPUMillis: 500, MemoryMB: 512,
		Port: 5432, Protocol: "tcp", Env: map[string]string{"POSTGRES_PASSWORD": "x"},
		PersistentGB: 5, MountPath: "/var/lib/postgresql/data", Role: "aluno",
	}
}

func TestCreate_TCPAllocatesNodePortAndAppliesQuota(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "lab.example.edu", "10.0.0.50")
	user := uuid.New()

	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *entity.Container) bool {
		return c.NodePort >= 30000 && c.NodePort <= 32767 && c.Namespace == entity.NamespaceFor(user)
	})).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, entity.NamespaceFor(user),
		repository.ResourceQuotaSpec{CPUMillis: 2000, MemoryMB: 4096, Pods: 5, StorageGB: 20}).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "lab.example.edu").Return(nil)
	rt.On("Status", mock.Anything, entity.NamespaceFor(user), []string{"meu-pg"}).
		Return(map[string]repository.ContainerStatus{"meu-pg": {Phase: "pending"}}, nil)

	view, err := uc.Create(context.Background(), user, baseReq())
	require.NoError(t, err)
	assert.Equal(t, "pending", view.Status.Phase)
	assert.Regexp(t, `^10\.0\.0\.50:3[0-2]\d{3}$`, view.ExternalURL)
	assert.Equal(t, "meu-pg:5432", view.InternalAddress)
	repo.AssertExpectations(t)
	rt.AssertExpectations(t)
}

func TestCreate_HTTPWithDomainUsesIngress(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "lab.example.edu", "10.0.0.50")
	user := uuid.New()
	req := baseReq()
	req.Name, req.Protocol, req.Port, req.PersistentGB = "site", "http", 80, 0

	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "site").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *entity.Container) bool { return c.NodePort == 0 })).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "lab.example.edu").Return(nil)
	rt.On("Status", mock.Anything, mock.Anything, mock.Anything).Return(map[string]repository.ContainerStatus{}, nil)

	view, err := uc.Create(context.Background(), user, req)
	require.NoError(t, err)
	assert.Equal(t, "http://site-"+entity.NamespaceFor(user)+".lab.example.edu", view.ExternalURL)
	assert.Equal(t, "unknown", view.Status.Phase, "sem entrada no mapa → unknown")
}

func TestCreate_HTTPWithoutDomainFallsBackToNodePort(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	req := baseReq()
	req.Protocol, req.Port = "http", 80
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.MatchedBy(func(c *entity.Container) bool { return c.NodePort > 0 })).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "").Return(nil)
	rt.On("Status", mock.Anything, mock.Anything, mock.Anything).Return(map[string]repository.ContainerStatus{}, nil)
	view, err := uc.Create(context.Background(), user, req)
	require.NoError(t, err)
	assert.Contains(t, view.ExternalURL, "10.0.0.50:")
}

func TestCreate_RetriesOnNodePortCollision(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(errors.New(`duplicate key value violates unique constraint "ux_containers_node_port"`)).Once()
	repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "").Return(nil)
	rt.On("Status", mock.Anything, mock.Anything, mock.Anything).Return(map[string]repository.ContainerStatus{}, nil)
	_, err := uc.Create(context.Background(), user, baseReq())
	require.NoError(t, err)
	repo.AssertNumberOfCalls(t, "Create", 2)
}

func TestCreate_LimitErrors(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()

	req := baseReq()
	req.CPUMillis = 3000
	_, err := uc.Create(context.Background(), user, req)
	assert.ErrorIs(t, err, usecase.ErrLimitExceeded)

	repo.On("CountByUserID", mock.Anything, user).Return(5, nil).Once()
	_, err = uc.Create(context.Background(), user, baseReq())
	assert.ErrorIs(t, err, usecase.ErrLimitExceeded, "5 containers já é o teto de aluno")

	repo.On("CountByUserID", mock.Anything, user).Return(0, nil).Once()
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(18, nil).Once()
	_, err = uc.Create(context.Background(), user, baseReq())
	assert.ErrorIs(t, err, usecase.ErrLimitExceeded, "18+5 > 20 GB")
}

func TestCreate_NameTakenAndInvalid(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(&entity.Container{}, nil)
	_, err := uc.Create(context.Background(), user, baseReq())
	assert.ErrorIs(t, err, usecase.ErrContainerNameTaken)

	req := baseReq()
	req.Name = "Inválido"
	_, err = uc.Create(context.Background(), user, req)
	assert.ErrorIs(t, err, usecase.ErrInvalidContainer)
}

func TestCreate_ApplyFailureRollsBack(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user := uuid.New()
	repo.On("CountByUserID", mock.Anything, user).Return(0, nil)
	repo.On("SumPersistentGBByUserID", mock.Anything, user).Return(0, nil)
	repo.On("FindByUserIDAndName", mock.Anything, user, "meu-pg").Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	repo.On("Delete", mock.Anything, mock.Anything).Return(nil)
	rt.On("EnsureNamespace", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	rt.On("Apply", mock.Anything, mock.Anything, "").Return(errors.New("boom"))
	_, err := uc.Create(context.Background(), user, baseReq())
	require.Error(t, err)
	repo.AssertCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestCreate_RuntimeNil(t *testing.T) {
	uc := usecase.NewContainerUseCase(new(mockContainerRepo), nil, "", "")
	_, err := uc.Create(context.Background(), uuid.New(), baseReq())
	assert.ErrorIs(t, err, repository.ErrRuntimeUnavailable)
}

func TestDelete_RuntimeFirstThenDB(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	user, id := uuid.New(), uuid.New()
	c := &entity.Container{ID: id, UserID: user, Name: "x", Namespace: "u-1"}
	repo.On("FindByID", mock.Anything, id).Return(c, nil)
	rt.On("Delete", mock.Anything, "u-1", "x").Return(errors.New("down")).Once()
	err := uc.Delete(context.Background(), user, id)
	require.Error(t, err)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)

	rt.On("Delete", mock.Anything, "u-1", "x").Return(nil).Once()
	repo.On("Delete", mock.Anything, id).Return(nil)
	require.NoError(t, uc.Delete(context.Background(), user, id))
}

func TestGet_OtherUserIsNotFound(t *testing.T) {
	repo, rt := new(mockContainerRepo), new(mockRuntime)
	uc := usecase.NewContainerUseCase(repo, rt, "", "10.0.0.50")
	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(&entity.Container{ID: id, UserID: uuid.New()}, nil)
	_, err := uc.Get(context.Background(), uuid.New(), id)
	assert.ErrorIs(t, err, usecase.ErrContainerNotFound)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/usecase/ -run 'TestCreate_|TestDelete_Runtime|TestGet_Other' -v`
Expected: FAIL — símbolos indefinidos.

- [ ] **Step 3: Implementação**

`container_usecase.go`:
```go
package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
)

var (
	ErrContainerNotFound  = errors.New("container não encontrado")
	ErrContainerNameTaken = errors.New("já existe um container com esse nome")
	ErrInvalidContainer   = errors.New("dados do container inválidos")
)

const (
	nodePortMin        = 30000
	nodePortMax        = 32767
	nodePortRetries    = 5
	defaultMountPath   = "/data"
)

type CreateContainerRequest struct {
	Name         string
	Description  string
	Image        string
	CPUMillis    int
	MemoryMB     int
	Port         int
	Protocol     string
	Env          map[string]string
	Command      string
	PersistentGB int
	MountPath    string
	Role         string
}

// ContainerView é a entidade + campos calculados a partir do cluster e da config.
type ContainerView struct {
	*entity.Container
	Status          repository.ContainerStatus `json:"status"`
	ExternalURL     string                     `json:"external_url"`
	InternalAddress string                     `json:"internal_address"`
}

type ContainerUseCase struct {
	repo          repository.ContainerRepository
	runtime       repository.ContainerRuntime // nil → cluster indisponível
	ingressDomain string
	nodeIP        string
}

func NewContainerUseCase(repo repository.ContainerRepository, runtime repository.ContainerRuntime, ingressDomain, nodeIP string) *ContainerUseCase {
	return &ContainerUseCase{repo: repo, runtime: runtime, ingressDomain: ingressDomain, nodeIP: nodeIP}
}

func (uc *ContainerUseCase) Create(ctx context.Context, userID uuid.UUID, req CreateContainerRequest) (*ContainerView, error) {
	if uc.runtime == nil {
		return nil, repository.ErrRuntimeUnavailable
	}
	if err := uc.validate(req); err != nil {
		return nil, err
	}
	limits := LimitsFor(req.Role)
	if req.CPUMillis > limits.MaxCPU*1000 {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %d CPU(s) por container", ErrLimitExceeded, req.Role, limits.MaxCPU)
	}
	if req.MemoryMB > limits.MaxMemMB {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %dMB de RAM por container", ErrLimitExceeded, req.Role, limits.MaxMemMB)
	}
	count, err := uc.repo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("erro ao contar containers: %w", err)
	}
	if int(count) >= limits.MaxContainers {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %d containers", ErrLimitExceeded, req.Role, limits.MaxContainers)
	}
	used, err := uc.repo.SumPersistentGBByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("erro ao somar armazenamento: %w", err)
	}
	if used+req.PersistentGB > limits.MaxStorageGB {
		return nil, fmt.Errorf("%w: perfil '%s' permite no máximo %d GB de armazenamento persistente", ErrLimitExceeded, req.Role, limits.MaxStorageGB)
	}
	existing, err := uc.repo.FindByUserIDAndName(ctx, userID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar nome: %w", err)
	}
	if existing != nil {
		return nil, ErrContainerNameTaken
	}

	mount := req.MountPath
	if mount == "" {
		mount = defaultMountPath
	}
	env := entity.EnvMap{}
	for k, v := range req.Env {
		env[k] = v
	}
	c := &entity.Container{
		ID: uuid.New(), UserID: userID, Name: req.Name, Description: req.Description, Image: req.Image,
		CPUMillis: req.CPUMillis, MemoryMB: req.MemoryMB, Port: req.Port, Protocol: req.Protocol,
		Env: env, Command: req.Command, PersistentGB: req.PersistentGB, MountPath: mount,
		Namespace: entity.NamespaceFor(userID),
	}
	if req.Port == 0 {
		c.Protocol = entity.ProtocolHTTP
	}

	if err := uc.persistWithNodePort(ctx, c); err != nil {
		return nil, err
	}

	quota := repository.ResourceQuotaSpec{CPUMillis: limits.MaxCPU * 1000, MemoryMB: limits.MaxMemMB, Pods: limits.MaxContainers, StorageGB: limits.MaxStorageGB}
	if err := uc.runtime.EnsureNamespace(ctx, c.Namespace, quota); err != nil {
		uc.rollback(ctx, c)
		log.Printf("[container] EnsureNamespace %s falhou: %v", c.Namespace, err)
		return nil, errors.New("não foi possível preparar o ambiente do usuário")
	}
	if err := uc.runtime.Apply(ctx, c, uc.ingressDomain); err != nil {
		uc.rollback(ctx, c)
		log.Printf("[container] Apply %s/%s falhou: %v", c.Namespace, c.Name, err)
		return nil, errors.New("não foi possível criar o container")
	}
	return uc.view(ctx, c), nil
}

func (uc *ContainerUseCase) validate(req CreateContainerRequest) error {
	if !entity.ContainerNameRegexp.MatchString(req.Name) {
		return fmt.Errorf("%w: nome deve ter 1–40 caracteres minúsculos, dígitos ou hífen", ErrInvalidContainer)
	}
	if strings.ContainsAny(req.Image, " \t\n") || !strings.Contains(req.Image, ":") {
		return fmt.Errorf("%w: imagem deve ter o formato registry/nome:tag", ErrInvalidContainer)
	}
	if req.Port < 0 || req.Port > 65535 {
		return fmt.Errorf("%w: porta fora do intervalo", ErrInvalidContainer)
	}
	if req.Port > 0 && req.Protocol != entity.ProtocolHTTP && req.Protocol != entity.ProtocolTCP {
		return fmt.Errorf("%w: protocolo deve ser http ou tcp", ErrInvalidContainer)
	}
	if req.CPUMillis <= 0 || req.MemoryMB <= 0 || req.PersistentGB < 0 {
		return fmt.Errorf("%w: cpu, memória e armazenamento devem ser positivos", ErrInvalidContainer)
	}
	if req.MountPath != "" && !strings.HasPrefix(req.MountPath, "/") {
		return fmt.Errorf("%w: caminho de montagem deve ser absoluto", ErrInvalidContainer)
	}
	return nil
}

// persistWithNodePort insere o registro; quando precisa de NodePort, sorteia e
// tenta de novo em colisão do índice único ux_containers_node_port.
func (uc *ContainerUseCase) persistWithNodePort(ctx context.Context, c *entity.Container) error {
	needsNodePort := c.Port > 0 && (c.Protocol == entity.ProtocolTCP || uc.ingressDomain == "")
	if !needsNodePort {
		if err := uc.repo.Create(ctx, c); err != nil {
			return fmt.Errorf("erro ao salvar container: %w", err)
		}
		return nil
	}
	for i := 0; i < nodePortRetries; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(nodePortMax-nodePortMin+1))
		if err != nil {
			return fmt.Errorf("erro ao sortear porta: %w", err)
		}
		c.NodePort = nodePortMin + int(n.Int64())
		err = uc.repo.Create(ctx, c)
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "ux_containers_node_port") {
			return fmt.Errorf("erro ao salvar container: %w", err)
		}
	}
	return errors.New("não foi possível alocar uma porta externa; tente novamente")
}

func (uc *ContainerUseCase) rollback(ctx context.Context, c *entity.Container) {
	if err := uc.repo.Delete(ctx, c.ID); err != nil {
		log.Printf("[container] rollback do registro %s falhou: %v", c.ID, err)
	}
}

func (uc *ContainerUseCase) List(ctx context.Context, userID uuid.UUID) ([]ContainerView, error) {
	items, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar containers: %w", err)
	}
	views := make([]ContainerView, 0, len(items))
	if len(items) == 0 {
		return views, nil
	}
	names := make([]string, len(items))
	for i := range items {
		names[i] = items[i].Name
	}
	statuses := uc.statuses(ctx, items[0].Namespace, names)
	for i := range items {
		c := &items[i]
		views = append(views, uc.buildView(c, statuses[c.Name]))
	}
	return views, nil
}

func (uc *ContainerUseCase) Get(ctx context.Context, userID, id uuid.UUID) (*ContainerView, error) {
	c, err := uc.FindOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return uc.view(ctx, c), nil
}

// Delete remove do cluster primeiro; se o cluster falhar o registro fica (sem órfão).
func (uc *ContainerUseCase) Delete(ctx context.Context, userID, id uuid.UUID) error {
	c, err := uc.FindOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	if uc.runtime == nil {
		return repository.ErrRuntimeUnavailable
	}
	if err := uc.runtime.Delete(ctx, c.Namespace, c.Name); err != nil {
		log.Printf("[container] Delete %s/%s no cluster falhou: %v", c.Namespace, c.Name, err)
		return repository.ErrRuntimeUnavailable
	}
	if err := uc.repo.Delete(ctx, c.ID); err != nil {
		return fmt.Errorf("erro ao remover container do banco: %w", err)
	}
	return nil
}

// FindOwned devolve o container se pertencer ao usuário; senão ErrContainerNotFound (não vaza existência).
func (uc *ContainerUseCase) FindOwned(ctx context.Context, userID, id uuid.UUID) (*entity.Container, error) {
	c, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar container: %w", err)
	}
	if c == nil || c.UserID != userID {
		return nil, ErrContainerNotFound
	}
	return c, nil
}

func (uc *ContainerUseCase) statuses(ctx context.Context, ns string, names []string) map[string]repository.ContainerStatus {
	if uc.runtime == nil {
		return nil
	}
	st, err := uc.runtime.Status(ctx, ns, names)
	if err != nil {
		log.Printf("[container] Status %s falhou: %v", ns, err)
		return nil
	}
	return st
}

func (uc *ContainerUseCase) view(ctx context.Context, c *entity.Container) *ContainerView {
	st := uc.statuses(ctx, c.Namespace, []string{c.Name})
	v := uc.buildView(c, st[c.Name])
	return &v
}

func (uc *ContainerUseCase) buildView(c *entity.Container, st repository.ContainerStatus) ContainerView {
	if st.Phase == "" {
		st = repository.ContainerStatus{Phase: repository.PhaseUnknown, Message: "cluster indisponível"}
	}
	v := ContainerView{Container: c, Status: st}
	if c.Port == 0 {
		return v
	}
	v.InternalAddress = fmt.Sprintf("%s:%d", c.Name, c.Port)
	switch {
	case c.NodePort > 0:
		v.ExternalURL = fmt.Sprintf("%s:%d", uc.nodeIP, c.NodePort)
	case uc.ingressDomain != "":
		v.ExternalURL = fmt.Sprintf("http://%s-%s.%s", c.Name, c.Namespace, uc.ingressDomain)
	}
	return v
}
```
Atenção: `TestCreate_RuntimeNil` chama `Create` antes de qualquer mock, então a checagem de runtime nil deve ser a primeira linha (como acima). Em `TestCreate_LimitErrors`, a validação de CPU precisa vir antes de `CountByUserID` (como acima).

- [ ] **Step 4: Rodar**

Run: `cd backend && go test ./internal/usecase/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add backend/internal/usecase/container_usecase.go backend/internal/usecase/container_usecase_test.go
git commit -m "feat(usecase): ContainerUseCase com limites por perfil, NodePort atômico e rollback"
```

---

### Task 9: Handler HTTP, rotas e wiring no `main.go`

**Files:**
- Create: `backend/internal/handler/container_handler.go`
- Create: `backend/internal/handler/container_handler_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/docs/swagger.yaml` (adicionar rotas de containers)

**Interfaces:**
- Consumes: `usecase.ContainerUseCase`, `entity.Catalog()`, `getUserID`, `getUserRole` (já existem em `vm_handler.go`).
- Produces: `handler.NewContainerHandler(uc *usecase.ContainerUseCase) *ContainerHandler` com `Catalog, Create, List, Get, Delete`.

- [ ] **Step 1: Teste do handler (binding e mapeamento de erros)**

`container_handler_test.go`:
```go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/handler"
	"github.com/proxlab/backend/internal/middleware"
	"github.com/proxlab/backend/internal/usecase"
	"github.com/stretchr/testify/assert"
)

func newRouter(h *handler.ContainerHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, uuid.New().String())
		c.Set(middleware.ContextKeyRole, "aluno")
	})
	r.GET("/api/containers/catalog", h.Catalog)
	r.POST("/api/containers", h.Create)
	return r
}

func TestCatalog(t *testing.T) {
	h := handler.NewContainerHandler(usecase.NewContainerUseCase(nil, nil, "", ""))
	w := httptest.NewRecorder()
	newRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/containers/catalog", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"key":"postgres"`)
}

func TestCreate_BadPayload(t *testing.T) {
	h := handler.NewContainerHandler(usecase.NewContainerUseCase(nil, nil, "", ""))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/containers", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	newRouter(h).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreate_RuntimeUnavailableIs503(t *testing.T) {
	h := handler.NewContainerHandler(usecase.NewContainerUseCase(nil, nil, "", ""))
	w := httptest.NewRecorder()
	body := `{"name":"ok","image":"nginx:1.27","cpu_millis":250,"memory_mb":256,"port":80,"protocol":"http"}`
	req := httptest.NewRequest(http.MethodPost, "/api/containers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	newRouter(h).ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/handler/ -v`
Expected: FAIL — `NewContainerHandler` indefinido.

- [ ] **Step 3: Handler**

`container_handler.go`:
```go
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/usecase"
)

type ContainerHandler struct{ uc *usecase.ContainerUseCase }

func NewContainerHandler(uc *usecase.ContainerUseCase) *ContainerHandler {
	return &ContainerHandler{uc: uc}
}

type createContainerRequest struct {
	Name         string            `json:"name"          binding:"required"`
	Description  string            `json:"description"`
	Image        string            `json:"image"         binding:"required"`
	CPUMillis    int               `json:"cpu_millis"    binding:"required,min=100"`
	MemoryMB     int               `json:"memory_mb"     binding:"required,min=64"`
	Port         int               `json:"port"          binding:"min=0,max=65535"`
	Protocol     string            `json:"protocol"      binding:"omitempty,oneof=http tcp"`
	Env          map[string]string `json:"env"`
	Command      string            `json:"command"`
	PersistentGB int               `json:"persistent_gb" binding:"min=0"`
	MountPath    string            `json:"mount_path"`
}

// Catalog godoc — GET /api/containers/catalog → 200 [CatalogImage]
func (h *ContainerHandler) Catalog(c *gin.Context) {
	c.JSON(http.StatusOK, entity.Catalog())
}

// Create godoc — POST /api/containers → 201 ContainerView
func (h *ContainerHandler) Create(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req createContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Protocol == "" {
		req.Protocol = entity.ProtocolHTTP
	}
	view, err := h.uc.Create(c.Request.Context(), userID, usecase.CreateContainerRequest{
		Name: req.Name, Description: req.Description, Image: req.Image,
		CPUMillis: req.CPUMillis, MemoryMB: req.MemoryMB, Port: req.Port, Protocol: req.Protocol,
		Env: req.Env, Command: req.Command, PersistentGB: req.PersistentGB, MountPath: req.MountPath,
		Role: getUserRole(c),
	})
	if err != nil {
		handleContainerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

// List godoc — GET /api/containers → 200 [ContainerView]
func (h *ContainerHandler) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	views, err := h.uc.List(c.Request.Context(), userID)
	if err != nil {
		handleContainerError(c, err)
		return
	}
	c.JSON(http.StatusOK, views)
}

// Get godoc — GET /api/containers/:id → 200 ContainerView | 404
func (h *ContainerHandler) Get(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de container inválido"})
		return
	}
	view, err := h.uc.Get(c.Request.Context(), userID, id)
	if err != nil {
		handleContainerError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// Delete godoc — DELETE /api/containers/:id → 204 | 404 | 503
func (h *ContainerHandler) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de container inválido"})
		return
	}
	if err := h.uc.Delete(c.Request.Context(), userID, id); err != nil {
		handleContainerError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func handleContainerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usecase.ErrContainerNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrContainerNameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, usecase.ErrLimitExceeded), errors.Is(err, usecase.ErrInvalidContainer):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrRuntimeUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno do servidor"})
	}
}
```
Nota: `TestCreate_RuntimeUnavailableIs503` passa um repo `nil`; o usecase checa `runtime == nil` antes de tocar o repo, então não há panic.

- [ ] **Step 4: Wiring em `main.go`**

Após `vmRepo := ...`:
```go
	containerRepo := repository.NewContainerRepository(db)
```
Após o `provisioner`:
```go
	var containerRuntime domainrepo.ContainerRuntime
	if rt, err := kube.New(cfg.KubeconfigPath); err != nil {
		log.Printf("[kube] cluster de containers indisponível (%v); rotas de containers responderão 503", err)
	} else {
		containerRuntime = rt
	}
```
(imports: `"github.com/proxlab/backend/internal/kube"` e `domainrepo "github.com/proxlab/backend/internal/domain/repository"`). Atenção à interface nil: só atribuir `rt` quando não houver erro, como acima, para não criar interface não-nil com ponteiro nil.

Após `vmUC`:
```go
	containerUC := usecase.NewContainerUseCase(containerRepo, containerRuntime, cfg.IngressDomain, hostOnly(cfg.K3SNodeIP))
	containerH := handler.NewContainerHandler(containerUC)
```
`hostOnly` NÃO pode viver em `main.go` (regra: sem lógica). Colocar em `config.go`:
```go
// K3SNodeHost devolve o IP sem o prefixo CIDR ("10.0.0.50/24" → "10.0.0.50").
func (c *Config) K3SNodeHost() string {
	host, _, _ := strings.Cut(c.K3SNodeIP, "/")
	return host
}
```
e usar `cfg.K3SNodeHost()`.

Rotas, dentro de `protected`:
```go
		containers := protected.Group("/containers")
		{
			containers.GET("/catalog", containerH.Catalog)
			containers.POST("", containerH.Create)
			containers.GET("", containerH.List)
			containers.GET("/:id", containerH.Get)
			containers.DELETE("/:id", containerH.Delete)
		}
```
(terminal e logs entram na Task 10.)

- [ ] **Step 5: Swagger** — em `backend/docs/swagger.yaml`, adicionar paths `/api/containers/catalog` (GET), `/api/containers` (GET, POST com o schema do payload acima), `/api/containers/{id}` (GET, DELETE), com respostas 201/200/204/404/409/422/503. Copiar o estilo dos paths de `/api/vms`.

- [ ] **Step 6: Rodar**

Run: `cd backend && go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**
```bash
git add backend/internal/handler/container_handler.go backend/internal/handler/container_handler_test.go backend/cmd/server/main.go backend/internal/config/config.go backend/docs/swagger.yaml
git commit -m "feat(api): rotas /api/containers (catálogo, criar, listar, detalhar, excluir)"
```

---

### Task 10: WebSocket — terminal exec e logs

**Files:**
- Create: `backend/internal/websocket/common.go`
- Modify: `backend/internal/websocket/terminal.go` (usar `common.go`)
- Create: `backend/internal/websocket/container_terminal.go`
- Create: `backend/internal/websocket/container_logs.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `usecase.ContainerUseCase.FindOwned`, `repository.ContainerRuntime.Exec/Logs`, `kube.ErrPodNotRunning`, `kube.IsNoShell`.
- Produces: `NewContainerTerminalHandler(uc, runtime)`, `(*ContainerTerminalHandler).Handle(c *gin.Context)`, `NewContainerLogsHandler(uc, runtime)`, `(*ContainerLogsHandler).Handle`.
- Protocolo cliente↔backend: entrada = mensagem de texto; resize = texto JSON `{"type":"resize","cols":N,"rows":N}`; saída = binário. Logs: só saída, texto linha a linha.

- [ ] **Step 1: common.go** — extrair de `terminal.go`:
```go
package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/middleware"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// userFromContext lê o userID injetado pelo middleware Auth (header ou ?token=).
func userFromContext(c *gin.Context) (uuid.UUID, bool) {
	raw, _ := c.Get(middleware.ContextKeyUserID)
	s, _ := raw.(string)
	id, err := uuid.Parse(s)
	return id, err == nil
}

// resizeMessage é o único JSON aceito do cliente; qualquer outro texto é entrada do teclado.
type resizeMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}
```
Em `terminal.go`: remover a declaração `var upgrader` e substituir o bloco que lê `rawUserID`/`userIDStr`/`userID` por:
```go
	userID, ok := userFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido"})
		return
	}
```
Rodar `go build ./...` para confirmar que `middleware` continua importado onde precisa.

- [ ] **Step 2: container_terminal.go**
```go
package websocket

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/proxlab/backend/internal/usecase"
)

type ContainerTerminalHandler struct {
	uc      *usecase.ContainerUseCase
	runtime repository.ContainerRuntime
}

func NewContainerTerminalHandler(uc *usecase.ContainerUseCase, runtime repository.ContainerRuntime) *ContainerTerminalHandler {
	return &ContainerTerminalHandler{uc: uc, runtime: runtime}
}

// Handle — GET /api/containers/:id/terminal?token=
func (h *ContainerTerminalHandler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de container inválido"})
		return
	}
	userID, ok := userFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido"})
		return
	}
	ctr, err := h.uc.FindOwned(c.Request.Context(), userID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "container não encontrado"})
		return
	}
	if h.runtime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": repository.ErrRuntimeUnavailable.Error()})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeText := func(s string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(s))
	}

	session, err := h.runtime.Exec(c.Request.Context(), ctr.Namespace, ctr.Name, 80, 24)
	if err != nil {
		if errors.Is(err, kube.ErrPodNotRunning) {
			writeText("\r\nO container ainda não está em execução.\r\n")
		} else {
			log.Printf("[container-terminal] exec %s/%s: %v", ctr.Namespace, ctr.Name, err)
			writeText("\r\nNão foi possível abrir o terminal.\r\n")
		}
		return
	}
	defer session.Close()

	errc := make(chan error, 2)

	// pod → cliente
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := session.Stdout().Read(buf)
			if n > 0 {
				writeMu.Lock()
				werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				writeMu.Unlock()
				if werr != nil {
					errc <- werr
					return
				}
			}
			if err != nil {
				if kube.IsNoShell(err) {
					writeText("\r\n" + repository.ErrNoShell.Error() + "\r\n")
				} else if !errors.Is(err, io.EOF) {
					log.Printf("[container-terminal] stream %s/%s encerrado: %v", ctr.Namespace, ctr.Name, err)
				}
				errc <- err
				return
			}
		}
	}()

	// cliente → pod
	go func() {
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if mt == websocket.TextMessage && len(msg) > 0 && msg[0] == '{' {
				var r resizeMessage
				if json.Unmarshal(msg, &r) == nil && r.Type == "resize" && r.Cols > 0 && r.Rows > 0 {
					_ = session.Resize(r.Cols, r.Rows)
					continue
				}
			}
			if _, err := session.Stdin().Write(msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-errc:
			return
		case <-ping.C:
			writeMu.Lock()
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}
```

- [ ] **Step 3: container_logs.go**
```go
package websocket

import (
	"bufio"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
	"github.com/proxlab/backend/internal/kube"
	"github.com/proxlab/backend/internal/usecase"
)

type ContainerLogsHandler struct {
	uc      *usecase.ContainerUseCase
	runtime repository.ContainerRuntime
}

func NewContainerLogsHandler(uc *usecase.ContainerUseCase, runtime repository.ContainerRuntime) *ContainerLogsHandler {
	return &ContainerLogsHandler{uc: uc, runtime: runtime}
}

// Handle — GET /api/containers/:id/logs?token=  (somente saída, texto)
func (h *ContainerLogsHandler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de container inválido"})
		return
	}
	userID, ok := userFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token inválido"})
		return
	}
	ctr, err := h.uc.FindOwned(c.Request.Context(), userID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "container não encontrado"})
		return
	}
	if h.runtime == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": repository.ErrRuntimeUnavailable.Error()})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx := c.Request.Context()
	// Descarta leituras do cliente só para detectar fechamento.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Enquanto o pod não está Running, informa o status a cada 2 s.
	for {
		rc, err := h.runtime.Logs(ctx, ctr.Namespace, ctr.Name, true)
		if err == nil {
			defer rc.Close()
			sc := bufio.NewScanner(rc)
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			for sc.Scan() {
				if werr := conn.WriteMessage(websocket.TextMessage, sc.Bytes()); werr != nil {
					return
				}
			}
			return
		}
		if !errors.Is(err, kube.ErrPodNotRunning) {
			log.Printf("[container-logs] %s/%s: %v", ctr.Namespace, ctr.Name, err)
			_ = conn.WriteMessage(websocket.TextMessage, []byte("[proxlab] não foi possível abrir os logs"))
			return
		}
		st, serr := h.runtime.Status(ctx, ctr.Namespace, []string{ctr.Name})
		msg := "[proxlab] aguardando o container iniciar..."
		if serr == nil {
			if s := st[ctr.Name]; s.Message != "" {
				msg = "[proxlab] " + s.Phase + ": " + s.Message
			}
			if st[ctr.Name].Phase == repository.PhaseFailed {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(msg))
				return
			}
		}
		if werr := conn.WriteMessage(websocket.TextMessage, []byte(msg)); werr != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
```
- [ ] **Step 4: Rotas em `main.go`**

Após `terminalH := ...`:
```go
	containerTermH := wshandler.NewContainerTerminalHandler(containerUC, containerRuntime)
	containerLogsH := wshandler.NewContainerLogsHandler(containerUC, containerRuntime)
```
No grupo `containers`:
```go
			containers.GET("/:id/terminal", containerTermH.Handle)
			containers.GET("/:id/logs", containerLogsH.Handle)
```
Mover a criação de `containerUC` para antes desse bloco se ainda não estiver.

- [ ] **Step 5: Verificar**

Run: `cd backend && go vet ./... && go test ./...`
Expected: PASS, sem avisos de vet.

- [ ] **Step 6: Commit**
```bash
git add backend/internal/websocket/ backend/cmd/server/main.go
git commit -m "feat(ws): terminal exec e logs em tempo real de containers via WebSocket"
```

---

### Task 11: `internal/k3s` e `cmd/k3s-bootstrap`

**Files:**
- Create: `backend/internal/k3s/keys.go`, `keys_test.go`
- Create: `backend/internal/k3s/ssh.go`
- Create: `backend/internal/k3s/kubeconfig.go`, `kubeconfig_test.go`
- Create: `backend/internal/k3s/install.go`
- Create: `backend/internal/k3s/manifests/registry.yaml`
- Create: `backend/cmd/k3s-bootstrap/main.go`
- Modify: `backend/Dockerfile`

**Interfaces:**
- Consumes: `repository.Provisioner` (Task 2), `config.Config` (Task 1).
- Produces: `k3s.Install(ctx, opts InstallOptions) error`; `k3s.WorkspaceID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("proxlab-k3s"))`.

- [ ] **Step 1: Testes de chave e kubeconfig**

`keys_test.go`:
```go
package k3s

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestEnsureKeyPair_GeneratesOnceAndReuses(t *testing.T) {
	dir := t.TempDir()
	priv1, pub1, err := ensureKeyPair(filepath.Join(dir, "id_ed25519"))
	require.NoError(t, err)
	assert.True(t, len(pub1) > 11 && string(pub1[:11]) == "ssh-ed25519")
	_, err = ssh.ParsePrivateKey(priv1)
	require.NoError(t, err)

	priv2, pub2, err := ensureKeyPair(filepath.Join(dir, "id_ed25519"))
	require.NoError(t, err)
	assert.Equal(t, priv1, priv2)
	assert.Equal(t, pub1, pub2)
}
```
`kubeconfig_test.go`:
```go
package k3s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteServer(t *testing.T) {
	in := "apiVersion: v1\nclusters:\n- cluster:\n    server: https://127.0.0.1:6443\n  name: default\n"
	out := rewriteServer([]byte(in), "10.0.0.50")
	assert.Contains(t, string(out), "server: https://10.0.0.50:6443")
	assert.NotContains(t, string(out), "127.0.0.1")
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/k3s/ -v`
Expected: FAIL — pacote não existe.

- [ ] **Step 3: keys.go**
```go
package k3s

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// ensureKeyPair devolve (privada PEM, pública authorized_keys); gera se não existir.
func ensureKeyPair(privPath string) ([]byte, []byte, error) {
	priv, err := os.ReadFile(privPath)
	if err == nil {
		signer, perr := ssh.ParsePrivateKey(priv)
		if perr != nil {
			return nil, nil, fmt.Errorf("chave existente inválida: %w", perr)
		}
		return priv, ssh.MarshalAuthorizedKey(signer.PublicKey()), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("erro ao ler chave: %w", err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao gerar chave: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(key, "proxlab-k3s")
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao serializar chave: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)
	if err := os.MkdirAll(filepath.Dir(privPath), 0o750); err != nil {
		return nil, nil, fmt.Errorf("erro ao criar diretório da chave: %w", err)
	}
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		return nil, nil, fmt.Errorf("erro ao gravar chave: %w", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("erro ao converter chave pública: %w", err)
	}
	return privPEM, ssh.MarshalAuthorizedKey(sshPub), nil
}
```

- [ ] **Step 4: ssh.go**
```go
package k3s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// waitSSH tenta conectar até o timeout do ctx; retorna o cliente pronto.
func waitSSH(ctx context.Context, host, user string, privPEM []byte, logf func(string, ...any)) (*ssh.Client, error) {
	signer, err := ssh.ParsePrivateKey(privPEM)
	if err != nil {
		return nil, fmt.Errorf("chave privada inválida: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // VM recém-criada na rede interna
		Timeout:         10 * time.Second,
	}
	addr := net.JoinHostPort(host, "22")
	for {
		client, err := ssh.Dial("tcp", addr, cfg)
		if err == nil {
			return client, nil
		}
		logf("ssh %s ainda indisponível: %v", addr, err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout aguardando SSH em %s: %w", addr, ctx.Err())
		case <-time.After(10 * time.Second):
		}
	}
}

// run executa "sudo sh -s" lendo o script de stdin; devolve stdout.
func run(client *ssh.Client, stdin io.Reader, out io.Writer) ([]byte, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir sessão ssh: %w", err)
	}
	defer sess.Close()
	var stdout bytes.Buffer
	sess.Stdout = io.MultiWriter(&stdout, out)
	sess.Stderr = out
	sess.Stdin = stdin
	if err := sess.Run("sudo sh -s"); err != nil {
		return stdout.Bytes(), fmt.Errorf("script remoto falhou: %w", err)
	}
	return stdout.Bytes(), nil
}
```

- [ ] **Step 5: kubeconfig.go**
```go
package k3s

import "regexp"

var serverRe = regexp.MustCompile(`server: https://[^:\s]+:6443`)

func rewriteServer(kubeconfig []byte, host string) []byte {
	return serverRe.ReplaceAll(kubeconfig, []byte("server: https://"+host+":6443"))
}
```

- [ ] **Step 6: manifests/registry.yaml**
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: proxlab-system
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: registry-data
  namespace: proxlab-system
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 50Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: proxlab-system
spec:
  replicas: 1
  selector:
    matchLabels: { app: registry }
  template:
    metadata:
      labels: { app: registry }
    spec:
      containers:
        - name: registry
          image: registry:2
          ports: [{ containerPort: 5000 }]
          volumeMounts: [{ name: data, mountPath: /var/lib/registry }]
      volumes:
        - name: data
          persistentVolumeClaim: { claimName: registry-data }
---
apiVersion: v1
kind: Service
metadata:
  name: registry
  namespace: proxlab-system
spec:
  selector: { app: registry }
  ports: [{ port: 5000, targetPort: 5000 }]
```

- [ ] **Step 7: install.go**

O script remoto usa heredocs internos; para não colidir, os delimitadores são `REGYAML` e `MANIFESTS`.
```go
package k3s

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/repository"
)

//go:embed manifests/*.yaml
var manifests embed.FS

// WorkspaceID é o workspace Terraform fixo da VM do cluster.
var WorkspaceID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("proxlab-k3s"))

const sshUser = "ubuntu"

type InstallOptions struct {
	Provisioner    repository.Provisioner
	Workdir        string // $TERRAFORM_WORKDIR
	TemplateVMID   int
	NodeIP         string // CIDR
	Gateway        string
	CPU, MemoryMB  int
	DiskGB         int
	KubeconfigPath string
	Out            io.Writer
}

const installScript = `set -e
if ! command -v k3s >/dev/null 2>&1; then
  curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --write-kubeconfig-mode 644 --node-ip %[1]s" sh -
fi
until kubectl get nodes >/dev/null 2>&1; do sleep 3; done
mkdir -p /etc/rancher/k3s
cat > /etc/rancher/k3s/registries.yaml <<'REGYAML'
mirrors:
  "registry.proxlab.local":
    endpoint: ["http://registry.proxlab-system.svc.cluster.local:5000"]
REGYAML
cat <<'MANIFESTS' | kubectl apply -f -
%[2]s
MANIFESTS
kubectl -n proxlab-system rollout status deploy/registry --timeout=180s || true
`

// Install cria a VM (se ainda não existir workspace), instala k3s por SSH,
// aplica os manifests base e grava o kubeconfig. Idempotente.
func Install(ctx context.Context, o InstallOptions) error {
	logf := func(f string, a ...any) { fmt.Fprintf(o.Out, f+"\n", a...) }
	host, _, _ := strings.Cut(o.NodeIP, "/")
	if host == "" || o.Gateway == "" {
		return fmt.Errorf("K3S_NODE_IP e K3S_NODE_GATEWAY são obrigatórios")
	}

	keyDir := filepath.Join(o.Workdir, "k3s")
	privPEM, pub, err := ensureKeyPair(filepath.Join(keyDir, "id_ed25519"))
	if err != nil {
		return err
	}

	wsDir := filepath.Join(o.Workdir, WorkspaceID.String())
	if _, err := os.Stat(filepath.Join(wsDir, "terraform.tfstate")); err == nil {
		logf("▶ VM do cluster já provisionada (workspace %s); pulando terraform", WorkspaceID)
	} else {
		logf("▶ 1/4 Provisionando VM k3s-server (%s)", host)
		spec := repository.VMSpec{
			Name: "k3s-server", CPUCores: o.CPU, MemoryMB: o.MemoryMB, DiskGB: o.DiskGB,
			Tags:             []string{"proxlab", "k3s"},
			CloneVMID:        o.TemplateVMID,
			CloudInitUser:    sshUser,
			CloudInitSSHKeys: []string{strings.TrimSpace(string(pub))},
			StaticIP:         o.NodeIP,
			Gateway:          o.Gateway,
		}
		vmid, err := o.Provisioner.Provision(ctx, WorkspaceID, spec)
		if err != nil {
			return fmt.Errorf("provision: %w", err)
		}
		logf("  ok — VMID=%d", vmid)
	}

	logf("▶ 2/4 Aguardando SSH em %s", host)
	sshCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	client, err := waitSSH(sshCtx, host, sshUser, privPEM, logf)
	if err != nil {
		return err
	}
	defer client.Close()

	logf("▶ 3/4 Instalando k3s e manifests base")
	reg, err := manifests.ReadFile("manifests/registry.yaml")
	if err != nil {
		return fmt.Errorf("manifest embutido ausente: %w", err)
	}
	script := fmt.Sprintf(installScript, host, string(reg))
	if _, err := run(client, strings.NewReader(script), o.Out); err != nil {
		return err
	}

	logf("▶ 4/4 Copiando kubeconfig para %s", o.KubeconfigPath)
	kc, err := run(client, strings.NewReader("cat /etc/rancher/k3s/k3s.yaml\n"), io.Discard)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(o.KubeconfigPath), 0o750); err != nil {
		return fmt.Errorf("erro ao criar diretório do kubeconfig: %w", err)
	}
	if err := os.WriteFile(o.KubeconfigPath, rewriteServer(kc, host), 0o600); err != nil {
		return fmt.Errorf("erro ao gravar kubeconfig: %w", err)
	}
	logf("Cluster pronto. Reinicie o backend para carregar o kubeconfig.")
	return nil
}
```

- [ ] **Step 8: cmd/k3s-bootstrap/main.go**
```go
// k3s-bootstrap cria a VM do cluster k3s no Proxmox e instala o k3s via SSH.
// Executar uma vez: docker compose run --rm backend /app/k3s-bootstrap
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/proxlab/backend/internal/config"
	"github.com/proxlab/backend/internal/k3s"
	"github.com/proxlab/backend/internal/terraform"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("configuração: %v", err)
	}
	runner, err := terraform.NewRunner(cfg)
	if err != nil {
		fatalf("terraform: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err = k3s.Install(ctx, k3s.InstallOptions{
		Provisioner: runner, Workdir: cfg.TerraformWorkdir, TemplateVMID: cfg.ProxmoxUbuntuTemplateVMID,
		NodeIP: cfg.K3SNodeIP, Gateway: cfg.K3SNodeGateway,
		CPU: cfg.K3SNodeCPU, MemoryMB: cfg.K3SNodeMemoryMB, DiskGB: cfg.K3SNodeDiskGB,
		KubeconfigPath: cfg.KubeconfigPath, Out: os.Stdout,
	})
	if err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERRO: "+format+"\n", args...)
	os.Exit(1)
}
```

- [ ] **Step 9: Dockerfile** — no builder, após o build do server:
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /k3s-bootstrap ./cmd/k3s-bootstrap
```
e no runtime, após `COPY --from=builder /app /app/server`:
```dockerfile
COPY --from=builder /k3s-bootstrap /app/k3s-bootstrap
```

- [ ] **Step 10: Rodar**

Run: `cd backend && go vet ./... && go test ./internal/k3s/ ./internal/terraform/ -v`
Expected: PASS.

- [ ] **Step 11: Commit**
```bash
git add backend/internal/k3s/ backend/cmd/k3s-bootstrap/ backend/Dockerfile
git commit -m "feat(k3s): comando k3s-bootstrap provisiona a VM do cluster e instala k3s via SSH"
```

---

### Task 12: Smoke test real (`proxmox-check --k3s`) e execução do bootstrap

**Files:**
- Modify: `backend/cmd/proxmox-check/main.go`

Pré-requisito manual (usuário): reservar o IP, preencher `K3S_NODE_IP`/`K3S_NODE_GATEWAY` no `.env` e rodar o bootstrap. Isso cria **uma** VM nova (`k3s-server`), nunca toca em VMs existentes.

- [ ] **Step 1: Modo `--k3s` no smoke test**

No início de `main()` após `config.Load()`:
```go
	if len(os.Args) > 1 && os.Args[1] == "--k3s" {
		smokeK3s(cfg)
		return
	}
```
e a função:
```go
// smokeK3s cria um container nginx do catálogo no cluster real, espera running,
// lê logs e apaga. Só toca em recursos criados aqui (namespace u-smoke00).
func smokeK3s(cfg *config.Config) {
	rt, err := kube.New(cfg.KubeconfigPath)
	if err != nil {
		fatalf("kube: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ns := "u-smoke00"
	img, _ := entity.CatalogByKey("nginx")
	c := &entity.Container{
		ID: uuid.New(), UserID: uuid.New(), Name: "smoke-" + uuid.New().String()[:6], Image: img.Image,
		CPUMillis: 250, MemoryMB: 128, Port: img.Port, Protocol: img.Protocol, NodePort: 32000,
		Env: entity.EnvMap{}, Namespace: ns,
	}
	fmt.Printf("▶ 1/4 EnsureNamespace(%s)\n", ns)
	if err := rt.EnsureNamespace(ctx, ns, repository.ResourceQuotaSpec{CPUMillis: 2000, MemoryMB: 4096, Pods: 5, StorageGB: 20}); err != nil {
		fatalf("EnsureNamespace: %v", err)
	}
	fmt.Printf("▶ 2/4 Apply(%s)\n", c.Name)
	if err := rt.Apply(ctx, c, cfg.IngressDomain); err != nil {
		fatalf("Apply: %v", err)
	}
	fmt.Println("▶ 3/4 Aguardando running")
	for {
		st, err := rt.Status(ctx, ns, []string{c.Name})
		if err != nil {
			fatalf("Status: %v", err)
		}
		fmt.Printf("  %s %s\n", st[c.Name].Phase, st[c.Name].Message)
		if st[c.Name].Phase == repository.PhaseRunning {
			break
		}
		if st[c.Name].Phase == repository.PhaseFailed {
			_ = rt.Delete(ctx, ns, c.Name)
			fatalf("container falhou")
		}
		time.Sleep(3 * time.Second)
	}
	rc, err := rt.Logs(ctx, ns, c.Name, false)
	if err != nil {
		fatalf("Logs: %v", err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	fmt.Printf("  logs (%d bytes) ok\n", len(b))
	fmt.Printf("▶ 4/4 Delete — testar antes: curl http://%s:32000\n", cfg.K3SNodeHost())
	if err := rt.Delete(ctx, ns, c.Name); err != nil {
		fatalf("Delete: %v", err)
	}
	fmt.Println("Smoke k3s concluído.")
}
```
Imports novos: `io`, `github.com/proxlab/backend/internal/domain/entity`, `github.com/proxlab/backend/internal/kube`.

- [ ] **Step 2: Build**

Run: `cd backend && go build ./... && go vet ./cmd/...`
Expected: OK.

- [ ] **Step 3: Rodar o bootstrap (usuário, contra o Proxmox real)**

```bash
cd backend && go run ./cmd/k3s-bootstrap
```
Expected: 4 etapas concluídas; `$TERRAFORM_WORKDIR/k3s/kubeconfig` criado. Se travar em "Aguardando SSH", conferir no console do Proxmox se a VM pegou o IP fixo (cloud-init) e se a rede permite porta 22.

- [ ] **Step 4: Rodar o smoke**

```bash
cd backend && go run ./cmd/proxmox-check --k3s
```
Expected: `running` e "Smoke k3s concluído". Se o pod for rejeitado por PodSecurity (`violates PodSecurity "restricted"`), alterar o label em `internal/kube/namespace.go` para `baseline`, atualizar `namespace_test.go`, e registrar no bloco "Decisões" da spec.

- [ ] **Step 5: Commit**
```bash
git add backend/cmd/proxmox-check/main.go backend/internal/kube/
git commit -m "chore(proxmox-check): smoke test --k3s cria e apaga um container no cluster real"
```

---

### Task 13: Frontend — `api.ts` e página de criação

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/app/(provisionamento)/containers/criar/page.tsx`

**Interfaces:**
- Produces (TS): `CatalogImage`, `ContainerStatus`, `Container`, `CreateContainerInput`, `STATUS_LABEL`, `apiGetCatalog()`, `apiListContainers()`, `apiGetContainer(id)`, `apiCreateContainer(data)`, `apiDeleteContainer(id)`, `containerWsURL(id, kind: "terminal"|"logs")`.

- [ ] **Step 1: api.ts** — acrescentar ao fim:
```ts
// ─── Containers ───────────────────────────────────────────────────────────────

export interface CatalogImage {
  key: string;
  label: string;
  image: string;
  port: number;
  protocol: "http" | "tcp" | "";
  mount_path: string;
  command: string;
  required_env: string[] | null;
}

export interface ContainerStatus {
  phase: "pending" | "running" | "failed" | "unknown";
  message: string;
}

export interface Container {
  id: string;
  name: string;
  description: string;
  image: string;
  cpu_millis: number;
  memory_mb: number;
  port: number;
  protocol: "http" | "tcp";
  node_port: number;
  env: Record<string, string>;
  command: string;
  persistent_gb: number;
  mount_path: string;
  namespace: string;
  created_at: string;
  status: ContainerStatus;
  external_url: string;
  internal_address: string;
}

export interface CreateContainerInput {
  name: string;
  description?: string;
  image: string;
  cpu_millis: number;
  memory_mb: number;
  port: number;
  protocol: "http" | "tcp";
  env: Record<string, string>;
  command?: string;
  persistent_gb: number;
  mount_path?: string;
}

export const STATUS_LABEL: Record<ContainerStatus["phase"], string> = {
  pending: "INICIANDO", running: "EM EXECUÇÃO", failed: "FALHOU", unknown: "DESCONHECIDO",
};

export async function apiGetCatalog(): Promise<CatalogImage[]> {
  return request<CatalogImage[]>("/api/containers/catalog");
}
export async function apiListContainers(): Promise<Container[]> {
  return request<Container[]>("/api/containers");
}
export async function apiGetContainer(id: string): Promise<Container> {
  return request<Container>(`/api/containers/${id}`);
}
export async function apiCreateContainer(data: CreateContainerInput): Promise<Container> {
  return request<Container>("/api/containers", { method: "POST", body: JSON.stringify(data) });
}
export async function apiDeleteContainer(id: string): Promise<void> {
  return request<void>(`/api/containers/${id}`, { method: "DELETE" });
}

/** URL do WebSocket de terminal/logs com ?token=, mesma regra do terminal de VM. */
export function containerWsURL(id: string, kind: "terminal" | "logs"): string {
  const token = getToken();
  const wsBase = API_BASE
    ? API_BASE.replace(/^http/, "ws")
    : `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`;
  return `${wsBase}/api/containers/${id}/${kind}${token ? `?token=${encodeURIComponent(token)}` : ""}`;
}
```
(Confirmar o nome da constante base em `api.ts`; se não for `API_BASE`, usar o nome existente.)

- [ ] **Step 2: Página `/containers/criar`** — reescrever como client component. Manter wrapper, header, breadcrumb, título, `section` e botões com as classes atuais. Remover o bloco "REQUER GPU" e o input "DEFINIÇÃO DE PORTAS". Remover a altura fixa `h-[484px]` do `section` (o conteúdo cresceu; única mudança de layout, coberta pela exceção acordada).

Estado e envio:
```tsx
"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiCreateContainer, apiGetCatalog, getRole, type CatalogImage } from "@/lib/api";

const LABEL = "mt-4 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)] uppercase";
const INPUT = "mt-1 h-[34px] w-full border border-black bg-transparent px-2";
const RADIO_ROW = "mt-3 flex gap-10 text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[rgba(0,0,0,0.87)]";

const CPU_OPTIONS = [500, 1000, 2000, 4000, 8000]; // millis
const MEM_OPTIONS = [256, 512, 1024, 2048, 4096, 8192, 16384];

export default function CriarContainerPage() {
  const router = useRouter();
  const role = getRole() ?? "aluno";
  const maxCpu = role === "pesquisador" ? 8000 : 2000;
  const maxMem = role === "pesquisador" ? 16384 : 4096;

  const [catalog, setCatalog] = useState<CatalogImage[]>([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [catalogKey, setCatalogKey] = useState("nginx");
  const [customImage, setCustomImage] = useState("");
  const [cpu, setCpu] = useState(500);
  const [mem, setMem] = useState(512);
  const [expose, setExpose] = useState(true);
  const [port, setPort] = useState(80);
  const [protocol, setProtocol] = useState<"http" | "tcp">("http");
  const [env, setEnv] = useState<{ k: string; v: string }[]>([]);
  const [persist, setPersist] = useState(false);
  const [persistGb, setPersistGb] = useState(5);
  const [command, setCommand] = useState("");
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => { apiGetCatalog().then(setCatalog).catch(() => setCatalog([])); }, []);

  const selected = catalog.find((c) => c.key === catalogKey);

  // Ao trocar de item do catálogo, pré-preenche porta/protocolo/comando/env obrigatórias.
  useEffect(() => {
    if (!selected) return;
    setExpose(selected.port > 0);
    setPort(selected.port || 80);
    setProtocol(selected.protocol === "tcp" ? "tcp" : "http");
    setCommand(selected.command ?? "");
    setEnv((selected.required_env ?? []).map((k) => ({ k, v: "" })));
  }, [selected?.key]); // eslint-disable-line react-hooks/exhaustive-deps

  async function handleCriar() {
    setError("");
    const image = catalogKey === "outra" ? customImage.trim() : selected?.image ?? "";
    if (!/^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$/.test(name)) {
      setError("Nome: 1–40 caracteres, letras minúsculas, números e hífen.");
      return;
    }
    if (!image.includes(":")) { setError("Informe a imagem com tag, ex.: nginx:1.27."); return; }
    const missing = env.find((e) => e.k && !e.v && selected?.required_env?.includes(e.k));
    if (missing) { setError(`Preencha a variável ${missing.k}.`); return; }
    setLoading(true);
    try {
      await apiCreateContainer({
        name, description, image, cpu_millis: cpu, memory_mb: mem,
        port: expose ? port : 0, protocol,
        env: Object.fromEntries(env.filter((e) => e.k).map((e) => [e.k, e.v])),
        command: command || undefined,
        persistent_gb: persist ? persistGb : 0,
        mount_path: catalogKey === "outra" ? undefined : selected?.mount_path,
      });
      router.push("/containers");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erro ao criar container.");
    } finally {
      setLoading(false);
    }
  }
  /* JSX abaixo */
}
```
JSX do `section` (ordem da spec; classes copiadas dos inputs existentes):
```tsx
<section className="bg-white px-3 pt-2 pb-4 shadow-[0_3px_6px_0_rgba(51,51,51,0.16)]">
  <p className="text-center text-[11.67px] leading-[16px] font-bold tracking-[1.5px] text-[#1A1A1A]">DETALHES DO CONTAINER</p>

  <p className={LABEL}>NOME DO CONTAINER:</p>
  <input className="mt-1 h-7 w-[623px] border border-black bg-transparent px-2" value={name} onChange={(e) => setName(e.target.value)} />

  <p className={LABEL}>IMAGEM DO CONTAINER</p>
  <select className={INPUT} value={catalogKey} onChange={(e) => setCatalogKey(e.target.value)}>
    {catalog.map((c) => <option key={c.key} value={c.key}>{c.label} ({c.image})</option>)}
    <option value="outra">Outra imagem...</option>
  </select>
  {catalogKey === "outra" && (
    <input className={INPUT} placeholder="registry/imagem:tag" value={customImage} onChange={(e) => setCustomImage(e.target.value)} />
  )}

  <p className={LABEL}>CPU:</p>
  <select className={INPUT} value={cpu} onChange={(e) => setCpu(Number(e.target.value))}>
    {CPU_OPTIONS.filter((v) => v <= maxCpu).map((v) => <option key={v} value={v}>{v / 1000} vCPU</option>)}
  </select>
  <p className={LABEL}>MEMÓRIA:</p>
  <select className={INPUT} value={mem} onChange={(e) => setMem(Number(e.target.value))}>
    {MEM_OPTIONS.filter((v) => v <= maxMem).map((v) => <option key={v} value={v}>{v >= 1024 ? `${v / 1024} GB` : `${v} MB`}</option>)}
  </select>

  <p className={LABEL}>PORTA E PROTOCOLO:</p>
  <div className={RADIO_ROW}>
    <label className="flex items-center gap-2"><input type="checkbox" checked={!expose} onChange={(e) => setExpose(!e.target.checked)} /> NÃO EXPOR</label>
  </div>
  {expose && (
    <>
      <input className={INPUT} type="number" min={1} max={65535} value={port} onChange={(e) => setPort(Number(e.target.value))} />
      <div className={RADIO_ROW}>
        <label className="flex items-center gap-2"><input type="radio" name="proto" checked={protocol === "http"} onChange={() => setProtocol("http")} /> HTTP (SITE/API)</label>
        <label className="flex items-center gap-2"><input type="radio" name="proto" checked={protocol === "tcp"} onChange={() => setProtocol("tcp")} /> TCP (BANCO, OUTRO SERVIÇO)</label>
      </div>
    </>
  )}

  <p className={LABEL}>VARIÁVEIS DE AMBIENTE:</p>
  {env.map((e, i) => (
    <div key={i} className="mt-1 flex gap-2">
      <input className="h-[34px] w-1/3 border border-black bg-transparent px-2" placeholder="CHAVE" value={e.k}
        onChange={(ev) => setEnv(env.map((x, j) => j === i ? { ...x, k: ev.target.value } : x))} />
      <input className="h-[34px] flex-1 border border-black bg-transparent px-2" placeholder="valor" value={e.v}
        onChange={(ev) => setEnv(env.map((x, j) => j === i ? { ...x, v: ev.target.value } : x))} />
      <button type="button" className="h-[34px] px-2 text-[#B21D38]" onClick={() => setEnv(env.filter((_, j) => j !== i))}>remover</button>
    </div>
  ))}
  <button type="button" className="mt-2 text-[11.67px] font-bold tracking-[1.5px] text-[#1351B4] uppercase" onClick={() => setEnv([...env, { k: "", v: "" }])}>+ adicionar variável</button>

  <p className={LABEL}>ARMAZENAMENTO PERSISTENTE:</p>
  <div className={RADIO_ROW}>
    <label className="flex items-center gap-2"><input type="radio" name="persist" checked={persist} onChange={() => setPersist(true)} /> SIM</label>
    <label className="flex items-center gap-2"><input type="radio" name="persist" checked={!persist} onChange={() => setPersist(false)} /> NÃO</label>
  </div>
  {persist && (
    <input className={INPUT} type="number" min={1} max={role === "pesquisador" ? 100 : 20} value={persistGb} onChange={(e) => setPersistGb(Number(e.target.value))} placeholder="Tamanho em GB" />
  )}

  <button type="button" className={LABEL + " text-[#1351B4]"} onClick={() => setShowAdvanced(!showAdvanced)}>AVANÇADO {showAdvanced ? "▲" : "▼"}</button>
  {showAdvanced && (
    <>
      <p className={LABEL}>COMANDO (OPCIONAL):</p>
      <input className={INPUT} value={command} onChange={(e) => setCommand(e.target.value)} placeholder="ex.: python app.py" />
    </>
  )}

  <p className={LABEL}>DESCRIÇÃO DO CONTAINER:</p>
  <input className={INPUT} value={description} onChange={(e) => setDescription(e.target.value)} />

  {error && <p className="mt-3 text-[13px] text-[#E52207]">{error}</p>}
</section>
```
Rodapé: trocar o `<Link>` "Criar" por `<button type="button" onClick={handleCriar} disabled={loading}>` com as mesmas classes; adicionar `<Link href="/containers">` "Voltar" com a classe do botão de borda vermelha usada em outras telas.

- [ ] **Step 3: Verificar**

Run: `cd frontend && npm run lint && npm run build`
Expected: sem erros de tipo/lint.

- [ ] **Step 4: Commit**
```bash
git add frontend/src/lib/api.ts "frontend/src/app/(provisionamento)/containers/criar/page.tsx"
git commit -m "feat(front): formulário de criação de container ligado ao catálogo e à API"
```

---

### Task 14: Frontend — lista e detalhe

**Files:**
- Modify: `frontend/src/app/(provisionamento)/containers/page.tsx`
- Modify: `frontend/src/app/(provisionamento)/containers/[id]/page.tsx`

- [ ] **Step 1: Lista** — client component. Substituir o array mock por estado carregado de `apiListContainers()`, com polling de 5 s enquanto houver `status.phase === "pending"`:
```tsx
"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { InternalBreadcrumb, InternalHeader } from "@/components";
import { apiListContainers, STATUS_LABEL, type Container } from "@/lib/api";

export default function ContainersPage() {
  const [items, setItems] = useState<Container[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;
    async function load() {
      try {
        const data = await apiListContainers();
        if (cancelled) return;
        setItems(data);
        setError("");
        if (data.some((c) => c.status.phase === "pending")) timer = setTimeout(load, 5000);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Erro ao listar containers.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => { cancelled = true; if (timer) clearTimeout(timer); };
  }, []);
  /* JSX: mesmo card do mock; linhas NOME, IMAGEM, STATUS, ENDEREÇO, DATA */
}
```
No card, manter as classes `<p>` do mock e exibir `NOME: {c.name}`, `IMAGEM: {c.image}`, `STATUS: {STATUS_LABEL[c.status.phase]}`, `ENDEREÇO: {c.external_url || "—"}`, `DATA DE CRIAÇÃO: {new Date(c.created_at).toLocaleDateString("pt-BR")}`. Mostrar `error` em `<p className="text-[13px] text-[#E52207]">`, "Carregando..." enquanto `loading`, e "Nenhum container criado." com a classe do label quando vazio. `STATUS_LABEL` vem de `api.ts` (Task 13).

- [ ] **Step 2: Detalhe** — client component com `useParams()`, `apiGetContainer(id)` repetido a cada 5 s enquanto `pending`, botão "Excluir Container" com `confirm()` chamando `apiDeleteContainer` e `router.push("/containers")`, botão "Utilizar Container" → `/containers/${id}/utilizar`. Seções com as classes do mock:
  - DETALHES: nome, descrição, data.
  - CONFIGURAÇÃO: imagem, CPU (`cpu_millis/1000` vCPU), memória, porta+protocolo (ou "não exposto"), armazenamento (`persistent_gb` GB em `mount_path` ou "não"), comando.
  - ACESSO: `ENDEREÇO EXTERNO: {external_url || "—"}`, `ENDEREÇO INTERNO: {internal_address || "—"}`.
  - VARIÁVEIS: uma linha por chave; valor oculto (`••••`) por padrão com botão "mostrar/ocultar" (estado `showEnv`).
  - ESTADO: substituir as duas linhas fixas por uma linha com `STATUS_LABEL[phase]`, bolinha verde `#1FA433` (`running`), amarela `#F5A623` (`pending`), vermelha `#FA1D1D` (`failed`/`unknown`), e `status.message` quando houver.
  Manter `export function generateStaticParams() { return []; }`.

- [ ] **Step 3: Verificar**

Run: `cd frontend && npm run lint && npm run build`
Expected: OK.

- [ ] **Step 4: Commit**
```bash
git add frontend/src/lib/api.ts "frontend/src/app/(provisionamento)/containers/page.tsx" "frontend/src/app/(provisionamento)/containers/[id]/page.tsx"
git commit -m "feat(front): lista e detalhe de containers com status do cluster e exclusão"
```

---

### Task 15: Frontend — terminal e logs

**Files:**
- Modify: `frontend/src/app/(provisionamento)/containers/[id]/utilizar/page.tsx`

- [ ] **Step 1: Página** — client component. Manter os dois `section` do mock. Painel lateral: nome, recursos em texto (CPU, memória, armazenamento), variáveis e ENDPOINT = `external_url`. Painel escuro dividido em duas metades: cabeçalho "Terminal" (mesma classe do cabeçalho "Logs do Container") + `<div ref={terminalRef} className="h-[280px] bg-[#111111]" />`; cabeçalho "Logs do Container" + `<div ref={logRef} className="h-[280px] overflow-y-auto px-[22px] pt-2 text-white">` com um `<p className="text-[13px] leading-[normal]">` por linha.

Terminal: copiar o `useEffect` do terminal de VM (`maquinas-virtuais/[id]/utilizar/page.tsx`, linhas do `useEffect` até o cleanup) trocando a URL por `containerWsURL(id, "terminal")`, mantendo reconexão, `binaryType = "arraybuffer"` e `term.onData → ws.send`. Adicionar envio de resize e o primeiro `fit` após conectar:
```ts
term.onResize(({ cols, rows }: { cols: number; rows: number }) => {
  const ws = wsRef.current;
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "resize", cols, rows }));
});
// dentro de ws.onopen, após o writeln:
fitAddon.fit();
ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
```

Logs: segundo `useEffect` com `new WebSocket(containerWsURL(id, "logs"))`; `onmessage` faz `setLines((l) => [...l.slice(-999), String(evt.data)])`; `useEffect` em `lines` faz `logRef.current?.scrollTo(0, logRef.current.scrollHeight)`. Em `onclose`, reconectar após 3 s até 60 tentativas sem limpar as linhas; no cleanup, fechar o socket e cancelar o timer.

Rodapé: substituir "Reiniciar/Desligar" (v2) por um único `<Link href={`/containers/${id}`}>` "Voltar" com a classe do botão azul.

- [ ] **Step 2: Verificar**

Run: `cd frontend && npm run lint && npm run build`
Expected: OK. Teste manual: abrir `/containers/<id>/utilizar` de um container `running`, digitar `ls` no terminal, ver logs chegando.

- [ ] **Step 3: Commit**
```bash
git add "frontend/src/app/(provisionamento)/containers/[id]/utilizar/page.tsx"
git commit -m "feat(front): terminal exec e logs em tempo real na tela de uso do container"
```

---

### Task 16: Documentação e verificação ponta a ponta

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/superpowers/specs/2026-09-10-containers-k3s-design.md`

- [ ] **Step 1: CLAUDE.md** — na seção Proxmox, acrescentar:
```
- Containers: cluster k3s single-node na VM `k3s-server` (criada por `cmd/k3s-bootstrap`, IP fixo `K3S_NODE_IP`). Backend fala com o cluster via `internal/kube` (client-go, kubeconfig em `KUBECONFIG_PATH`). Namespace por usuário `u-<8 chars>`, quota por perfil, status lido do cluster a cada request (sem coluna no banco). HTTP + `INGRESS_DOMAIN` → Ingress `http://<nome>-<ns>.<domínio>`; TCP ou sem domínio → NodePort 30000–32767 em `K3S_NODE_IP`; porta 0 = só terminal.
- Env k3s: `K3S_NODE_IP`, `K3S_NODE_GATEWAY`, `K3S_NODE_CPU`, `K3S_NODE_MEMORY_MB`, `K3S_NODE_DISK_GB`, `INGRESS_DOMAIN`, `KUBECONFIG_PATH`
```
Na árvore de pastas, adicionar `internal/kube/`, `internal/k3s/` e `cmd/k3s-bootstrap/`. No fluxo obrigatório, adicionar `Handler → UseCase → ContainerRuntime Interface → client-go → k3s`. Em Fases: `6. Containers de aplicação em k3s ← ✅ concluído (v1)`. Em Pendências: marcar a fase feita e listar v2 (start/stop/restart, kaniko/tarball, TLS, GPU).

- [ ] **Step 2: Spec** — registrar em "Decisões fechadas" os desvios: `Apply(ctx, c, ingressDomain)`; workspace do cluster = UUID determinístico `uuid.NewSHA1(NameSpaceOID, "proxlab-k3s")`; nível PodSecurity final (restricted ou baseline) conforme o smoke test.

- [ ] **Step 3: Ponta a ponta manual (usuário)**
  1. Backend reiniciado com kubeconfig presente; log não mostra "cluster indisponível".
  2. Front: criar `postgres` do catálogo com `POSTGRES_PASSWORD`, 5 GB persistente.
  3. Detalhe mostra `ENDEREÇO EXTERNO: <K3S_NODE_IP>:<porta>`; conectar com `psql -h <ip> -p <porta> -U postgres`.
  4. `/utilizar`: terminal responde a `psql -U postgres -c 'select 1'`; logs mostram "database system is ready".
  5. Excluir: registro some da lista; via SSH na VM, `kubectl -n u-xxxxxxxx get all` não mostra o container.
  6. Criar `nginx` HTTP: com `INGRESS_DOMAIN` vazio recebe `ip:porta`; `curl` responde 200.

- [ ] **Step 4: Commit**
```bash
git add CLAUDE.md docs/superpowers/specs/2026-09-10-containers-k3s-design.md
git commit -m "docs: CLAUDE.md e spec refletem a fase de containers k3s concluída"
```
