package proxmox

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// VMCreateRequest contém os parâmetros para criar uma VM QEMU via
// POST /nodes/{node}/qemu
type VMCreateRequest struct {
	VMID       int      // 0 = obter automaticamente via /cluster/nextid
	Name       string
	CPUCores   int
	MemoryMB   int      // RAM em MB
	DiskGB     int      // tamanho do disco raiz em GB
	Storage    string   // storage Proxmox, ex: "local-lvm". Default: "local-lvm"
	OSType     string   // "l26" Linux, "win10" Win 10, "win11" Win 11, "other"
	ISO        string   // caminho da ISO, ex: "local:iso/ubuntu-22.04.iso"
	Network    string   // config de rede. Default: "model=virtio,bridge=vmbr0"
	Pool       string   // resource pool do Proxmox, ex: "researchers" ou "students"
	Tags       []string // tags para identificar a VM no painel do Proxmox
	RequireGPU bool     // se true e PROXMOX_GPU_PCI configurado, adiciona hostpci0
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

// TermProxyResponse contém as credenciais retornadas por
// POST /nodes/{node}/qemu/{vmid}/termproxy
// Usado para abrir um terminal xterm.js via WebSocket.
type TermProxyResponse struct {
	Ticket string // ticket de autenticação temporário
	Port   int    // porta do WebSocket de terminal no nó Proxmox
	UPID   string // UPID da tarefa
}

// TaskStatus representa o estado de uma tarefa assíncrona do Proxmox.
// Obtido via GET /nodes/{node}/tasks/{upid}/status
type TaskStatus struct {
	UPID       string // Unique Process ID da tarefa
	Status     string // "running" | "stopped"
	ExitStatus string // "OK" ou mensagem de erro quando Status == "stopped"
	Type       string // tipo da tarefa, ex: "qmcreate", "qmstart", "qmdestroy"
}

// --- Tipos privados para decodificação do JSON da API do Proxmox ---

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

type pveTermProxy struct {
	Ticket string      `json:"ticket"`
	Port   flexInt     `json:"port"`
	UPID   string      `json:"upid"`
}

// flexInt deserializa um campo JSON que pode chegar como número (5900)
// ou como string ("5900") dependendo da versão do Proxmox.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	// tenta número primeiro
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexInt(n)
		return nil
	}
	// tenta string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("flexInt: não é número nem string: %s", data)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("flexInt: string não numérica: %q", s)
	}
	*f = flexInt(n)
	return nil
}

type pveTaskStatus struct {
	UPID       string `json:"upid"`
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
	Type       string `json:"type"`
}
