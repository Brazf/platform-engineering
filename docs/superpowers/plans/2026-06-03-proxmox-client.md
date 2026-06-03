# Proxmox VE HTTP Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar o cliente HTTP real para a API REST do Proxmox VE, separar os tipos em `types.go`, mover a interface para `domain/repository/proxmox_client.go`, e atualizar todos os pontos de consumo para compilar.

**Architecture:** `ProxmoxClient` (infra) implementa implicitamente `ProxmoxClientInterface` (domain/repository) via duck typing do Go. A interface fica no domínio; a implementação fica no pacote `proxmox`. Os use cases dependem apenas da interface, não do pacote concreto.

**Tech Stack:** `net/http`, `encoding/json`, `url.Values` para form-encoding (Proxmox usa `application/x-www-form-urlencoded`), generics Go 1.21+ para helper `decode[T]`.

---

## Delta — mudanças de assinatura críticas

| Elemento | Estado atual | Após essa task |
|---|---|---|
| `proxmox.ClientInterface` | No `proxmox/client.go`, métodos com `node string` explícito | **Removido** |
| `proxmox.NewClient(...)` | 4 args | `NewProxmoxClient(cfg *config.Config)` |
| `StartVM`, `StopVM` | `error` | `(taskID string, err error)` |
| `ListVMs` | `(ctx, node string)` | `(ctx) []VMStatus` — node é do client |
| `GetVMStatus`, `GetVNCProxy` | `(ctx, node, vmid)` | `(ctx, vmid)` — node é do client |
| `usecase.VMUseCase.proxmoxClient` | `proxmox.ClientInterface` | `repository.ProxmoxClientInterface` |
| `websocket.TerminalHandler.proxmoxClient` | `proxmox.ClientInterface` | `repository.ProxmoxClientInterface` |

---

## File map

| Arquivo | Ação |
|---|---|
| `backend/internal/proxmox/types.go` | **Criar** — tipos públicos e privados JSON |
| `backend/internal/proxmox/client.go` | **Reescrever** — ProxmoxClient, NewProxmoxClient, doRequest, todos os métodos |
| `backend/internal/domain/repository/proxmox_client.go` | **Criar** — ProxmoxClientInterface |
| `backend/internal/usecase/vm_usecase.go` | **Atualizar** — trocar import proxmox → domain/repository |
| `backend/internal/websocket/terminal.go` | **Atualizar** — trocar import proxmox → domain/repository |
| `backend/cmd/server/main.go` | **Atualizar** — NewProxmoxClient(cfg) |

---

### Task 1: Types — tipos públicos e structs JSON privados

**Files:**
- Create: `backend/internal/proxmox/types.go`

- [ ] Criar o arquivo com os tipos públicos e os helpers JSON privados:

```go
package proxmox

// VMCreateRequest contém os parâmetros para criar uma VM QEMU via
// POST /nodes/{node}/qemu
type VMCreateRequest struct {
	VMID     int    // 0 = obter automaticamente via /cluster/nextid
	Name     string
	CPUCores int
	MemoryMB int    // RAM em MB
	DiskGB   int    // tamanho do disco raiz em GB
	Storage  string // storage Proxmox, ex: "local-lvm". Default: "local-lvm"
	OSType   string // "l26" Linux, "win10" Win 10, "win11" Win 11, "other"
	ISO      string // caminho da ISO, ex: "local:iso/ubuntu-22.04.iso"
	Network  string // config de rede. Default: "virtio,bridge=vmbr0"
}

// VMStatus representa o estado de runtime de uma VM obtido via
// GET /nodes/{node}/qemu/{vmid}/status/current
type VMStatus struct {
	VMID    int
	Name    string
	Status  string  // "running" | "stopped" | "paused"
	CPU     float64 // fração de uso de CPU (0.0–1.0)
	MemUsed int64   // memória em uso (bytes)
	MemMax  int64   // memória total alocada (bytes)
	Uptime  int64   // tempo ligada em segundos
}

// VNCProxyResponse contém as credenciais retornadas por
// POST /nodes/{node}/qemu/{vmid}/vncproxy?websocket=1
type VNCProxyResponse struct {
	Ticket string // ticket temporário de autenticação VNC
	Port   int    // porta VNC no nó Proxmox
	Cert   string // certificado TLS do servidor VNC (PEM, pode ser vazio)
}

// TaskStatus representa o estado de uma tarefa assíncrona do Proxmox.
// Obtido via GET /nodes/{node}/tasks/{upid}/status
type TaskStatus struct {
	UPID       string // Unique Process ID da tarefa
	Status     string // "running" | "stopped"
	ExitStatus string // "OK" ou mensagem de erro quando Status == "stopped"
	Type       string // tipo da tarefa, ex: "qmcreate", "qmstart", "qmdestroy"
}

// --- Tipos privados para decodificação do JSON do Proxmox ---

// pveEnvelope é o wrapper JSON de todas as respostas da API: {"data": T}
type pveEnvelope[T any] struct {
	Data T `json:"data"`
}

type pveVMStatus struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
	Uptime int64   `json:"uptime"`
}

type pveQEMUEntry struct {
	VMID   int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type pveVNCProxy struct {
	Ticket string `json:"ticket"`
	Port   int    `json:"port"`
	Cert   string `json:"cert"`
}

type pveTaskStatus struct {
	UPID       string `json:"upid"`
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
	Type       string `json:"type"`
}
```

- [ ] `go build ./internal/proxmox/...` — deve compilar
- [ ] Commit: `git add backend/internal/proxmox/types.go && git commit -m "feat(proxmox): add public types and private JSON mapping structs"`

---

### Task 2: Interface ProxmoxClientInterface no domínio

**Files:**
- Create: `backend/internal/domain/repository/proxmox_client.go`

> **Nota de arquitetura:** a interface referencia tipos do pacote `proxmox`, o que cria uma dependência `domain/repository → proxmox`. Isso é uma concessão pragmática: evita duplicar tipos de API do Proxmox como value objects de domínio. O `proxmox.Client` implementa a interface implicitamente (duck typing Go) — sem import reverso.

- [ ] Criar a interface:

```go
package repository

import (
	"context"

	"github.com/proxlab/backend/internal/proxmox"
)

// ProxmoxClientInterface é o port para operações na API do Proxmox VE.
// Definido na camada de domínio para desacoplar os use cases da implementação HTTP concreta.
// Implementado por internal/proxmox.ProxmoxClient.
type ProxmoxClientInterface interface {
	// CreateVM cria uma VM QEMU no nó Proxmox.
	// Proxmox: POST /nodes/{node}/qemu
	// Retorna o VMID atribuído e o UPID da tarefa assíncrona.
	CreateVM(ctx context.Context, req proxmox.VMCreateRequest) (vmid int, taskID string, err error)

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

	// DeleteVM destroi uma VM e seus discos.
	// Proxmox: DELETE /nodes/{node}/qemu/{vmid}
	DeleteVM(ctx context.Context, vmid int) error

	// GetVNCProxy cria um ticket VNC para acesso via terminal web (xterm.js).
	// Proxmox: POST /nodes/{node}/qemu/{vmid}/vncproxy  body: websocket=1
	GetVNCProxy(ctx context.Context, vmid int) (proxmox.VNCProxyResponse, error)

	// ListVMs lista todas as VMs no nó configurado.
	// Proxmox: GET /nodes/{node}/qemu
	ListVMs(ctx context.Context) ([]proxmox.VMStatus, error)

	// GetTaskStatus consulta o estado de uma tarefa assíncrona pelo UPID.
	// Proxmox: GET /nodes/{node}/tasks/{upid}/status
	GetTaskStatus(ctx context.Context, upid string) (proxmox.TaskStatus, error)
}
```

- [ ] `go build ./internal/domain/...` — deve compilar
- [ ] Commit: `git add backend/internal/domain/repository/proxmox_client.go && git commit -m "feat(domain): add ProxmoxClientInterface port"`

---

### Task 3: Implementação ProxmoxClient com todos os métodos reais

**Files:**
- Modify: `backend/internal/proxmox/client.go`

- [ ] Reescrever completamente (remove stubs, old interface, old NewClient):

```go
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/proxlab/backend/internal/config"
)

// ProxmoxClient é o cliente HTTP para a API REST do Proxmox VE.
type ProxmoxClient struct {
	baseURL     string
	node        string
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
}

// NewProxmoxClient cria um ProxmoxClient configurado a partir do Config da aplicação.
// TLS verification é desabilitada pois o ambiente de laboratório usa certificados self-signed.
func NewProxmoxClient(cfg *config.Config) *ProxmoxClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // rede interna da faculdade
	}
	return &ProxmoxClient{
		baseURL:     cfg.ProxmoxHost,
		node:        cfg.ProxmoxNode,
		tokenID:     cfg.ProxmoxTokenID,
		tokenSecret: cfg.ProxmoxTokenSecret,
		httpClient:  &http.Client{Transport: transport},
	}
}

// doRequest executa uma requisição HTTP autenticada contra a API REST do Proxmox.
// path é relativo a /api2/json, ex: "/nodes/pve/qemu".
// body deve ser form-encoded (url.Values.Encode()) ou nil.
func (c *ProxmoxClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	fullURL := c.baseURL + "/api2/json" + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição ao Proxmox: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxmox retornou %d: %s", resp.StatusCode, string(raw))
	}
	return resp, nil
}

// decodeBody parseia o envelope {"data": T} de uma resposta do Proxmox.
func decodeBody[T any](resp *http.Response) (T, error) {
	defer resp.Body.Close()
	var envelope pveEnvelope[T]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		var zero T
		return zero, fmt.Errorf("erro ao decodificar resposta do Proxmox: %w", err)
	}
	return envelope.Data, nil
}

// nextVMID obtém o próximo ID de VM disponível no cluster.
// Proxmox: GET /cluster/nextid
func (c *ProxmoxClient) nextVMID(ctx context.Context) (int, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/cluster/nextid", nil)
	if err != nil {
		return 0, fmt.Errorf("erro ao obter próximo VMID: %w", err)
	}
	// Proxmox pode retornar nextid como número ou string dependendo da versão
	raw, err := decodeBody[json.RawMessage](resp)
	if err != nil {
		return 0, err
	}
	var id int
	if err := json.Unmarshal(raw, &id); err == nil {
		return id, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("formato inesperado de nextid: %s", string(raw))
	}
	return strconv.Atoi(s)
}

// CreateVM cria uma VM QEMU no nó Proxmox.
// Proxmox: POST /nodes/{node}/qemu
// Se req.VMID == 0, obtém automaticamente o próximo ID disponível via /cluster/nextid.
func (c *ProxmoxClient) CreateVM(ctx context.Context, req VMCreateRequest) (int, string, error) {
	vmid := req.VMID
	if vmid == 0 {
		var err error
		vmid, err = c.nextVMID(ctx)
		if err != nil {
			return 0, "", err
		}
	}

	storage := req.Storage
	if storage == "" {
		storage = "local-lvm"
	}
	network := req.Network
	if network == "" {
		network = "virtio,bridge=vmbr0"
	}

	params := url.Values{}
	params.Set("vmid", strconv.Itoa(vmid))
	params.Set("name", req.Name)
	params.Set("cores", strconv.Itoa(req.CPUCores))
	params.Set("memory", strconv.Itoa(req.MemoryMB))
	params.Set("scsihw", "virtio-scsi-pci")
	params.Set("scsi0", fmt.Sprintf("%s:%d,format=qcow2", storage, req.DiskGB))
	params.Set("ostype", req.OSType)
	params.Set("net0", network)
	if req.ISO != "" {
		params.Set("ide2", req.ISO+",media=cdrom")
		params.Set("boot", "order=ide2;scsi0")
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/nodes/"+c.node+"/qemu", strings.NewReader(params.Encode()))
	if err != nil {
		return 0, "", fmt.Errorf("erro ao criar VM: %w", err)
	}
	taskID, err := decodeBody[string](resp)
	if err != nil {
		return 0, "", err
	}
	return vmid, taskID, nil
}

// GetVMStatus retorna o estado atual de runtime de uma VM.
// Proxmox: GET /nodes/{node}/qemu/{vmid}/status/current
func (c *ProxmoxClient) GetVMStatus(ctx context.Context, vmid int) (VMStatus, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return VMStatus{}, fmt.Errorf("erro ao obter status da VM %d: %w", vmid, err)
	}
	raw, err := decodeBody[pveVMStatus](resp)
	if err != nil {
		return VMStatus{}, err
	}
	return VMStatus{
		VMID:    raw.VMID,
		Name:    raw.Name,
		Status:  raw.Status,
		CPU:     raw.CPU,
		MemUsed: raw.Mem,
		MemMax:  raw.MaxMem,
		Uptime:  raw.Uptime,
	}, nil
}

// StartVM inicia uma VM parada de forma assíncrona.
// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/start
// Retorna o UPID da tarefa de inicialização.
func (c *ProxmoxClient) StartVM(ctx context.Context, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(""))
	if err != nil {
		return "", fmt.Errorf("erro ao iniciar VM %d: %w", vmid, err)
	}
	return decodeBody[string](resp)
}

// StopVM para uma VM em execução (hard stop — sem graceful shutdown).
// Proxmox: POST /nodes/{node}/qemu/{vmid}/status/stop
// Retorna o UPID da tarefa de parada.
func (c *ProxmoxClient) StopVM(ctx context.Context, vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(""))
	if err != nil {
		return "", fmt.Errorf("erro ao parar VM %d: %w", vmid, err)
	}
	return decodeBody[string](resp)
}

// DeleteVM destroi uma VM e seus discos.
// Proxmox: DELETE /nodes/{node}/qemu/{vmid}
func (c *ProxmoxClient) DeleteVM(ctx context.Context, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", c.node, vmid)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("erro ao deletar VM %d: %w", vmid, err)
	}
	resp.Body.Close()
	return nil
}

// GetVNCProxy cria um ticket VNC para acesso via terminal web (xterm.js + noVNC).
// Proxmox: POST /nodes/{node}/qemu/{vmid}/vncproxy  body: websocket=1
func (c *ProxmoxClient) GetVNCProxy(ctx context.Context, vmid int) (VNCProxyResponse, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", c.node, vmid)
	params := url.Values{}
	params.Set("websocket", "1")
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(params.Encode()))
	if err != nil {
		return VNCProxyResponse{}, fmt.Errorf("erro ao obter VNC proxy para VM %d: %w", vmid, err)
	}
	raw, err := decodeBody[pveVNCProxy](resp)
	if err != nil {
		return VNCProxyResponse{}, err
	}
	return VNCProxyResponse{Ticket: raw.Ticket, Port: raw.Port, Cert: raw.Cert}, nil
}

// ListVMs lista todas as VMs no nó configurado.
// Proxmox: GET /nodes/{node}/qemu
func (c *ProxmoxClient) ListVMs(ctx context.Context) ([]VMStatus, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", c.node)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar VMs: %w", err)
	}
	entries, err := decodeBody[[]pveQEMUEntry](resp)
	if err != nil {
		return nil, err
	}
	statuses := make([]VMStatus, len(entries))
	for i, e := range entries {
		statuses[i] = VMStatus{VMID: e.VMID, Name: e.Name, Status: e.Status}
	}
	return statuses, nil
}

// GetTaskStatus consulta o estado de uma tarefa assíncrona pelo UPID.
// Proxmox: GET /nodes/{node}/tasks/{upid}/status
func (c *ProxmoxClient) GetTaskStatus(ctx context.Context, upid string) (TaskStatus, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", c.node, url.PathEscape(upid))
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return TaskStatus{}, fmt.Errorf("erro ao obter status da tarefa: %w", err)
	}
	raw, err := decodeBody[pveTaskStatus](resp)
	if err != nil {
		return TaskStatus{}, err
	}
	return TaskStatus{UPID: raw.UPID, Status: raw.Status, ExitStatus: raw.ExitStatus, Type: raw.Type}, nil
}
```

- [ ] `go build ./internal/proxmox/...` — deve compilar
- [ ] Commit: `git add backend/internal/proxmox/client.go && git commit -m "feat(proxmox): implement full Proxmox VE HTTP client"`

---

### Task 4: Atualizar vm_usecase para usar a interface do domínio

**Files:**
- Modify: `backend/internal/usecase/vm_usecase.go`

- [ ] Trocar `proxmox.ClientInterface` por `repository.ProxmoxClientInterface`:

```go
package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/proxlab/backend/internal/domain/entity"
	"github.com/proxlab/backend/internal/domain/repository"
)

type CreateVMInput struct {
	Name       string
	CPUCores   int
	MemoryMB   int
	DiskGB     int
	OSTemplate string
	Node       string
}

type CreateVMOutput struct {
	VMID         int
	Status       entity.VMStatus
	WebSocketURL string
}

type VMUseCase struct {
	vmRepo        repository.VMRepository
	proxmoxClient repository.ProxmoxClientInterface
}

func NewVMUseCase(vmRepo repository.VMRepository, proxmoxClient repository.ProxmoxClientInterface) *VMUseCase {
	return &VMUseCase{vmRepo: vmRepo, proxmoxClient: proxmoxClient}
}

func (uc *VMUseCase) Create(ctx context.Context, userID uuid.UUID, input CreateVMInput) (*CreateVMOutput, error) {
	_ = userID
	_ = input
	return nil, errors.New("not implemented")
}

func (uc *VMUseCase) List(ctx context.Context, userID uuid.UUID) ([]entity.VM, error) {
	_ = userID
	return nil, errors.New("not implemented")
}

func (uc *VMUseCase) Get(ctx context.Context, userID, vmID uuid.UUID) (*entity.VM, error) {
	_ = userID
	_ = vmID
	return nil, errors.New("not implemented")
}

func (uc *VMUseCase) Start(ctx context.Context, userID, vmID uuid.UUID) error {
	_ = userID
	_ = vmID
	return errors.New("not implemented")
}

func (uc *VMUseCase) Stop(ctx context.Context, userID, vmID uuid.UUID) error {
	_ = userID
	_ = vmID
	return errors.New("not implemented")
}

func (uc *VMUseCase) Delete(ctx context.Context, userID, vmID uuid.UUID) error {
	_ = userID
	_ = vmID
	return errors.New("not implemented")
}
```

- [ ] Commit: `git add backend/internal/usecase/vm_usecase.go && git commit -m "refactor(usecase): depend on ProxmoxClientInterface from domain layer"`

---

### Task 5: Atualizar websocket/terminal.go

**Files:**
- Modify: `backend/internal/websocket/terminal.go`

- [ ] Trocar `proxmox.ClientInterface` por `repository.ProxmoxClientInterface`:

```go
package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/proxlab/backend/internal/domain/repository"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: restringir origens em produção
		return true
	},
}

// TerminalHandler gerencia conexões WebSocket de terminal para VMs.
type TerminalHandler struct {
	proxmoxClient repository.ProxmoxClientInterface
}

func NewTerminalHandler(proxmoxClient repository.ProxmoxClientInterface) *TerminalHandler {
	return &TerminalHandler{proxmoxClient: proxmoxClient}
}

// HandleTerminal faz upgrade da conexão HTTP para WebSocket e faz proxy
// do terminal VNC/SPICE da VM via API do Proxmox.
//
// GET /api/vms/:id/terminal (requer JWT válido no header)
func (h *TerminalHandler) HandleTerminal(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Stub: fecha conexão imediatamente enquanto não implementado
	_ = conn.WriteMessage(websocket.TextMessage, []byte("terminal not implemented"))
}
```

- [ ] Commit: `git add backend/internal/websocket/terminal.go && git commit -m "refactor(websocket): depend on ProxmoxClientInterface from domain layer"`

---

### Task 6: Atualizar main.go — NewProxmoxClient(cfg)

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] Substituir `proxmox.NewClient(...)` por `proxmox.NewProxmoxClient(cfg)`:

Localizar a linha:
```go
proxmoxClient := proxmox.NewClient(
    cfg.ProxmoxHost,
    cfg.ProxmoxNode,
    cfg.ProxmoxTokenID,
    cfg.ProxmoxTokenSecret,
)
```

Substituir por:
```go
proxmoxClient := proxmox.NewProxmoxClient(cfg)
```

- [ ] Commit: `git add backend/cmd/server/main.go && git commit -m "refactor(main): use NewProxmoxClient(cfg)"`

---

### Task 7: Verificação final

- [ ] `go build ./...` — sem erros
- [ ] `go vet ./...` — sem warnings
- [ ] Verificar que `proxmox.Client` implementa `repository.ProxmoxClientInterface` na compilação: `grep -r "ProxmoxClientInterface" backend/`
- [ ] Verificar que nenhum arquivo usa `proxmox.NewClient` ou `proxmox.ClientInterface` antigos: `grep -r "NewClient\b\|ClientInterface" backend/internal/`
